package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// at freezes the stamp clock for one step.
func at(t *testing.T, ts string) {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}
	Now = func() time.Time { return tm }
	t.Cleanup(func() { Now = time.Now })
}

// writeFile writes raw text into the dataset.
func writeFile(t *testing.T, b *Book, rel, body string) {
	t.Helper()
	p := filepath.Join(b.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// txIDs lists live transaction ids in the book.
func txIDs(b *Book) map[string]model.Transaction {
	out := map[string]model.Transaction{}
	_ = b.View(func(d *Data, c *Computed) error {
		for _, t := range d.Txns {
			out[t.ID] = t
		}
		return nil
	})
	return out
}

func exists(b *Book, rel string) bool {
	_, err := os.Stat(filepath.Join(b.Dir, rel))
	return err == nil
}

const conflictName = "transactions/transaction_202610.sync-conflict-20261010-120000-PHONE01.md"

func txYAML(id, date, title, amount, updated string) string {
	return "  - id: " + id + "\n    date: '" + date + "'\n    type: expense\n    title: " + title +
		"\n    updated: '" + updated + "'\n    postings:\n      - account: bank\n        amount: '-" + amount +
		"'\n      - category: food\n        amount: '" + amount + "'\n"
}

func TestConflictCopyMergedOnMain(t *testing.T) {
	b := newBook(t)
	writeFile(t, b, "transactions/transaction_202610.md", "transactions:\n"+
		txYAML("a", "2026-10-01", "Main-A", "100", "2026-10-01T10:00:00Z")+
		txYAML("b", "2026-10-02", "Main-B", "200", "2026-10-02T10:00:00Z"))
	writeFile(t, b, conflictName, "transactions:\n"+
		txYAML("a", "2026-10-01", "Old-A", "999", "2026-09-30T10:00:00Z")+ // older: loses
		txYAML("c", "2026-10-03", "Phone-C", "50", "2026-10-03T10:00:00Z")) // only on phone
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	got := txIDs(b)
	if len(got) != 3 || got["a"].Title != "Main-A" || got["c"].Title != "Phone-C" {
		t.Fatalf("merge result: %+v", got)
	}
	if exists(b, conflictName) {
		t.Error("conflict copy should be removed on the main device")
	}
	if s := read(t, b, "transactions/transaction_202610.md"); !strings.Contains(s, "Phone-C") {
		t.Errorf("merged record not written:\n%s", s)
	}
	if now, _ := bal(b, "bank"); now != money.FromRupees(10000-100-200-50) {
		t.Errorf("bank = %s", now)
	}
}

func TestNewestEditWins(t *testing.T) {
	b := newBook(t)
	writeFile(t, b, "transactions/transaction_202610.md", "transactions:\n"+txYAML("a", "2026-10-01", "Server", "100", "2026-10-01T10:00:00Z"))
	writeFile(t, b, conflictName, "transactions:\n"+txYAML("a", "2026-10-01", "Phone", "120", "2026-10-01T11:00:00Z"))
	_ = b.Reload()
	if got := txIDs(b)["a"]; got.Title != "Phone" {
		t.Fatalf("newest edit should win, got %q", got.Title)
	}
}

func TestTombstones(t *testing.T) {
	b := newBook(t)
	at(t, "2026-10-05T09:00:00Z")
	id := add(t, b, Entry{Type: model.TxExpense, Date: "2026-10-04", Title: "Gone", Amount: money.FromRupees(10), Account: "bank", Category: "food"})
	at(t, "2026-10-05T10:00:00Z")
	if err := b.Mutate(func(d *Data, ch *Change) error { d.DeleteTx(id, ch); return nil }); err != nil {
		t.Fatal(err)
	}
	if s := read(t, b, "transactions/transaction_202610.md"); !strings.Contains(s, "deleted: ") {
		t.Fatalf("tombstone not written:\n%s", s)
	}
	// A device that was offline still has the old version: the deletion must win.
	writeFile(t, b, conflictName, "transactions:\n"+txYAML(id, "2026-10-04", "Gone", "10", "2026-10-05T09:00:00Z"))
	_ = b.Reload()
	if _, back := txIDs(b)[id]; back {
		t.Fatal("deleted transaction came back from a conflict copy")
	}
	// An edit made after the deletion wins over it.
	writeFile(t, b, conflictName, "transactions:\n"+txYAML(id, "2026-10-04", "Edited later", "10", "2026-10-05T11:00:00Z"))
	_ = b.Reload()
	if got := txIDs(b)[id]; got.Title != "Edited later" {
		t.Fatalf("later edit should beat the deletion, got %+v", got)
	}
}

func TestTombstonePurgedAfterTTL(t *testing.T) {
	b := newBook(t)
	writeFile(t, b, "transactions/transaction_202606.md", "transactions:\n  - id: old\n    date: '2026-06-01'\n    deleted: '2026-06-01T00:00:00Z'\n")
	at(t, "2026-10-05T00:00:00Z") // > 90 days later
	_ = b.Reload()
	if s := read(t, b, "transactions/transaction_202606.md"); strings.Contains(s, "id: old") {
		t.Fatalf("old tombstone not purged:\n%s", s)
	}
}

func TestDateMovedOnOtherDevice(t *testing.T) {
	b := newBook(t)
	// Same id in two months: the newer copy (moved to September) wins.
	writeFile(t, b, "transactions/transaction_202610.md", "transactions:\n"+txYAML("a", "2026-10-01", "Before", "100", "2026-10-01T10:00:00Z"))
	writeFile(t, b, "transactions/transaction_202609.md", "transactions:\n"+txYAML("a", "2026-09-30", "Moved", "100", "2026-10-02T10:00:00Z"))
	_ = b.Reload()
	got := txIDs(b)
	if len(got) != 1 || got["a"].Date != "2026-09-30" {
		t.Fatalf("got %+v", got)
	}
	if s := read(t, b, "transactions/transaction_202610.md"); strings.Contains(s, "Before") {
		t.Errorf("stale copy left in October:\n%s", s)
	}
}

func TestDefinitionConflictUnion(t *testing.T) {
	b := newBook(t)
	writeFile(t, b, "accounts.sync-conflict-20261010-120000-PHONE01.md", "accounts:\n  - id: bank\n    name: Renamed on phone\n    type: bank\n  - id: wallet\n    name: Wallet\n    type: wallet\n")
	_ = b.Reload()
	var names []string
	_ = b.View(func(d *Data, c *Computed) error {
		for _, a := range d.Accounts {
			names = append(names, a.ID+"="+a.Name)
		}
		return nil
	})
	if strings.Join(names, ",") != "bank=Bank,card=Card,wallet=Wallet" {
		t.Fatalf("accounts after merge: %v", names)
	}
	if exists(b, "accounts.sync-conflict-20261010-120000-PHONE01.md") {
		t.Error("definition conflict copy not removed")
	}
}

func TestReplicaMergesInMemoryOnly(t *testing.T) {
	main := newBook(t)
	writeFile(t, main, "transactions/transaction_202610.md", "transactions:\n"+txYAML("a", "2026-10-01", "A", "100", "2026-10-01T10:00:00Z"))
	writeFile(t, main, conflictName, "transactions:\n"+txYAML("c", "2026-10-03", "C", "50", "2026-10-03T10:00:00Z"))
	before := read(t, main, AccountsFile)
	_ = os.Remove(filepath.Join(main.Dir, MoneyFile))
	rep, err := OpenWith(main.Dir, Options{Name: "phone", Replica: true})
	if err != nil {
		t.Fatal(err)
	}
	rep.Today = main.Today
	rep.Recompute()
	if len(txIDs(rep)) != 2 {
		t.Fatal("replica should include the conflict copy in memory")
	}
	if !exists(rep, conflictName) {
		t.Error("replica must not delete conflict copies")
	}
	if exists(rep, MoneyFile) || read(t, rep, AccountsFile) != before {
		t.Error("replica must not write computed files")
	}
	// A user change on the replica is still saved.
	add(t, rep, Entry{Type: model.TxExpense, Date: "2026-10-04", Title: "From phone", Amount: money.FromRupees(5), Account: "bank", Category: "food"})
	if s := read(t, rep, "transactions/transaction_202610.md"); !strings.Contains(s, "From phone") {
		t.Fatal("replica did not save the user's transaction")
	}
}
