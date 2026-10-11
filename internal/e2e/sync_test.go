// Package e2e runs real-world multi-device scenarios: the nimory server (main) and
// the phone app (replica) share one folder through a simulated Syncthing.
package e2e

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// ---- simulated Syncthing ----------------------------------------------------

// mesh syncs two folders the way Syncthing does: one-sided changes propagate,
// changes on both sides keep the newer file and save the other as a conflict copy.
type mesh struct {
	dirs [2]string
	devs [2]string           // device ids (used in conflict file names)
	base map[string][32]byte // content hash at the last sync, per relative path
}

func newMesh(a, b string) *mesh {
	return &mesh{dirs: [2]string{a, b}, devs: [2]string{"NIMORY1", "PHONE01"}, base: map[string][32]byte{}}
}

type fileState struct {
	ok   bool
	hash [32]byte
	mod  time.Time
}

// scan lists files under dir (dot files are temp/hidden and never synced).
func scan(dir string) map[string]fileState {
	out := map[string]fileState{}
	_ = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		b, _ := os.ReadFile(p)
		st, _ := os.Stat(p)
		rel, _ := filepath.Rel(dir, p)
		out[rel] = fileState{true, sha256.Sum256(b), st.ModTime()}
		return nil
	})
	return out
}

// copyFile copies src → dst keeping the modification time.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(src)
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(dst, st.ModTime(), st.ModTime())
}

// sync runs one Syncthing round and returns the conflict copies it created.
func (m *mesh) sync(t *testing.T) []string {
	t.Helper()
	s := [2]map[string]fileState{scan(m.dirs[0]), scan(m.dirs[1])}
	paths := map[string]bool{}
	for i := range s {
		for p := range s[i] {
			paths[p] = true
		}
	}
	for p := range m.base {
		paths[p] = true
	}
	var keys []string
	for p := range paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	var conflicts []string
	for _, p := range keys {
		a, b := s[0][p], s[1][p]
		base, hadBase := m.base[p]
		if a.ok && b.ok && a.hash == b.hash {
			m.base[p] = a.hash
			continue
		}
		changed := func(f fileState) bool { return f.ok != hadBase || (f.ok && f.hash != base) }
		ca, cb := changed(a), changed(b)
		from := -1
		switch {
		case ca && !cb:
			from = 0
		case cb && !ca:
			from = 1
		case ca && cb && (!a.ok || !b.ok): // modify beats delete
			if a.ok {
				from = 0
			} else {
				from = 1
			}
		case ca && cb:
			win, lose := 0, 1
			if b.mod.After(a.mod) {
				win, lose = 1, 0
			}
			ext := filepath.Ext(p)
			cp := strings.TrimSuffix(p, ext) + ".sync-conflict-" + time.Now().Format("20060102-150405") + "-" + m.devs[lose] + ext
			copyFile(t, filepath.Join(m.dirs[lose], p), filepath.Join(m.dirs[lose], cp))
			copyFile(t, filepath.Join(m.dirs[lose], cp), filepath.Join(m.dirs[win], cp))
			copyFile(t, filepath.Join(m.dirs[win], p), filepath.Join(m.dirs[lose], p))
			m.base[cp] = sha256.Sum256(must(os.ReadFile(filepath.Join(m.dirs[win], cp))))
			m.base[p] = s[win][p].hash
			conflicts = append(conflicts, cp)
			continue
		}
		if from < 0 {
			continue
		}
		to := 1 - from
		if s[from][p].ok {
			copyFile(t, filepath.Join(m.dirs[from], p), filepath.Join(m.dirs[to], p))
			m.base[p] = s[from][p].hash
		} else {
			_ = os.Remove(filepath.Join(m.dirs[to], p))
			delete(m.base, p)
		}
	}
	return conflicts
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// touchAll sets the modification time of every file in dir (who saved last).
func touchAll(dir string, tm time.Time) {
	for p := range scan(dir) {
		_ = os.Chtimes(filepath.Join(dir, p), tm, tm)
	}
}

// ---- helpers ------------------------------------------------------------------

var today = dates.MustParse("2026-10-06")

func open(t *testing.T, dir, name string, replica bool) *book.Book {
	t.Helper()
	b, err := book.OpenWith(dir, book.Options{Name: name, Replica: replica})
	if err != nil {
		t.Fatal(err)
	}
	b.Today = func() dates.Date { return today }
	b.Recompute()
	return b
}

// addTx saves an expense and returns its id.
func addTx(t *testing.T, b *book.Book, id, date, title string, rupees int64) {
	t.Helper()
	err := b.Mutate(func(d *book.Data, ch *book.Change) error {
		tx, err := d.BuildTx(book.Entry{ID: id, Type: model.TxExpense, Date: date, Title: title, Amount: money.FromRupees(rupees), Account: "bank", Category: "food"})
		if err != nil {
			return err
		}
		d.SetTx(tx, ch)
		return nil
	})
	if err != nil {
		t.Fatalf("add %s: %v", title, err)
	}
}

func editTx(t *testing.T, b *book.Book, id string, fn func(e *book.Entry)) {
	t.Helper()
	err := b.Mutate(func(d *book.Data, ch *book.Change) error {
		old, ok := d.Tx(id)
		if !ok {
			return fmt.Errorf("no %s", id)
		}
		e := book.EntryOf(old)
		fn(&e)
		tx, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		d.SetTx(tx, ch)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func delTx(t *testing.T, b *book.Book, id string) {
	t.Helper()
	if err := b.Mutate(func(d *book.Data, ch *book.Change) error { d.DeleteTx(id, ch); return nil }); err != nil {
		t.Fatal(err)
	}
}

// snapshot is "id title amount" per live transaction, sorted.
func snapshot(b *book.Book) []string {
	var out []string
	_ = b.View(func(d *book.Data, c *book.Computed) error {
		for _, t := range d.Txns {
			out = append(out, fmt.Sprintf("%s %s %s", t.ID, t.Title, book.EntryOf(t).Amount))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func balance(b *book.Book, acc string) money.Amount {
	var v money.Amount
	_ = b.View(func(d *book.Data, c *book.Computed) error { v = c.Balance[acc]; return nil })
	return v
}

func conflictFiles(dir string) []string {
	var out []string
	for p := range scan(dir) {
		if book.IsConflictCopy(p) {
			out = append(out, p)
		}
	}
	return out
}

// settle syncs until nothing changes, reloading both books after each round (the
// watcher's job) — like leaving both devices online for a minute.
func settle(t *testing.T, m *mesh, main, phone *book.Book) {
	t.Helper()
	for i := 0; i < 6; i++ {
		m.sync(t)
		if err := main.Reload(); err != nil {
			t.Fatal(err)
		}
		if err := phone.Reload(); err != nil {
			t.Fatal(err)
		}
		if a, b := scan(m.dirs[0]), scan(m.dirs[1]); sameTree(a, b) {
			return
		}
	}
	t.Fatal("folders did not settle")
}

func sameTree(a, b map[string]fileState) bool {
	if len(a) != len(b) {
		return false
	}
	for p, f := range a {
		if b[p].hash != f.hash {
			return false
		}
	}
	return true
}

// setup creates the main dataset, syncs it to the phone and opens both.
func setup(t *testing.T) (*mesh, *book.Book, *book.Book) {
	t.Helper()
	nimory, phoneDir := t.TempDir(), t.TempDir()
	main := open(t, nimory, "nimory", false)
	err := main.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = []model.Account{{ID: "bank", Name: "Bank", Type: "bank", OpeningBalance: money.FromRupees(100000)}}
		d.Categories = []model.Category{{ID: "food", Name: "Food", Kind: model.KindExpense}}
		ch.Accounts, ch.Categories = true, true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	addTx(t, main, "t-groceries", "2026-10-02", "Groceries", 1500)
	m := newMesh(nimory, phoneDir)
	m.sync(t)
	phone := open(t, phoneDir, "phone", true)
	return m, main, phone
}

// ---- scenarios ------------------------------------------------------------------

// Both devices online: a phone entry reaches nimory and balances agree.
func TestOnlineEntryFromPhone(t *testing.T) {
	m, main, phone := setup(t)
	addTx(t, phone, "c-tea", "2026-10-05", "Tea", 20)
	settle(t, m, main, phone)
	if got := snapshot(main); strings.Join(got, "|") != strings.Join(snapshot(phone), "|") || len(got) != 2 {
		t.Fatalf("main %v phone %v", got, snapshot(phone))
	}
	if balance(main, "bank") != money.FromRupees(100000-1500-20) || balance(phone, "bank") != balance(main, "bank") {
		t.Fatalf("balances main %s phone %s", balance(main, "bank"), balance(phone, "bank"))
	}
	if c := conflictFiles(m.dirs[0]); len(c) != 0 {
		t.Fatalf("unexpected conflicts %v", c)
	}
}

// The phone is offline while both devices change the same month; whichever file
// Syncthing keeps, nothing is lost once nimory merges the conflict copy.
func TestOfflinePhoneThenReconnect(t *testing.T) {
	for _, phoneNewer := range []bool{false, true} {
		t.Run(fmt.Sprintf("phoneSavedLast=%v", phoneNewer), func(t *testing.T) {
			m, main, phone := setup(t)
			addTx(t, phone, "c-tea", "2026-10-03", "Tea", 20)
			settle(t, m, main, phone)

			// --- offline window: no syncing ---
			addTx(t, phone, "c-lunch", "2026-10-05", "Lunch", 300)
			addTx(t, phone, "c-cab", "2026-10-05", "Cab", 250)
			editTx(t, phone, "t-groceries", func(e *book.Entry) { e.Amount = money.FromRupees(1600) }) // fixed the amount on the phone
			delTx(t, phone, "c-tea")                                                                   // deleted on the phone
			time.Sleep(1100 * time.Millisecond)                                                        // later stamps on nimory
			addTx(t, main, "t-fuel", "2026-10-04", "Fuel", 2000)                                       // added from the laptop
			addTx(t, main, "t-rent", "2026-10-06", "Rent", 20000)                                      // added on nimory too
			if phoneNewer {
				touchAll(m.dirs[1], time.Now().Add(time.Minute))
			} else {
				touchAll(m.dirs[0], time.Now().Add(time.Minute))
			}

			// --- back online ---
			if c := m.sync(t); len(c) == 0 {
				t.Fatal("expected Syncthing to create a conflict copy in this scenario")
			}
			settle(t, m, main, phone)

			want := []string{"c-cab Cab 250.00", "c-lunch Lunch 300.00", "t-fuel Fuel 2000.00", "t-groceries Groceries 1600.00", "t-rent Rent 20000.00"}
			if got := snapshot(main); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("main:\n got %v\nwant %v", got, want)
			}
			if got := snapshot(phone); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("phone:\n got %v\nwant %v", got, want)
			}
			wantBal := money.FromRupees(100000 - 250 - 300 - 20000 - 2000 - 1600)
			if balance(main, "bank") != wantBal || balance(phone, "bank") != wantBal {
				t.Fatalf("balances main %s phone %s want %s", balance(main, "bank"), balance(phone, "bank"), wantBal)
			}
			for _, d := range m.dirs {
				if c := conflictFiles(d); len(c) != 0 {
					t.Fatalf("conflict copies left in %s: %v", d, c)
				}
			}
			if s := string(must(os.ReadFile(filepath.Join(m.dirs[1], "money.md")))); !strings.Contains(s, "balanced: true") {
				t.Fatalf("money.md not balanced:\n%s", s)
			}
		})
	}
}

// An account created on the offline phone survives nimory rewriting accounts.md.
func TestAccountAddedOfflineSurvives(t *testing.T) {
	m, main, phone := setup(t)
	err := phone.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = append(d.Accounts, model.Account{ID: "wallet", Name: "Wallet", Type: "wallet", OpeningBalance: money.FromRupees(500)})
		ch.Accounts = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	addTx(t, main, "t-fuel", "2026-10-04", "Fuel", 2000) // nimory rewrites balances in accounts.md
	touchAll(m.dirs[0], time.Now().Add(time.Minute))     // nimory's accounts.md is newer and wins
	m.sync(t)
	settle(t, m, main, phone)
	for _, b := range []*book.Book{main, phone} {
		var ok bool
		_ = b.View(func(d *book.Data, c *book.Computed) error { _, ok = d.Account("wallet"); return nil })
		if !ok {
			t.Fatalf("%s lost the wallet account", b.Name)
		}
	}
	if c := conflictFiles(m.dirs[1]); len(c) != 0 {
		t.Fatalf("conflict copies left: %v", c)
	}
}
