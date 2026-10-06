package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

const today = "2026-10-06"

// newBook opens a book in a temp dir with a fixed "today" and two accounts and
// three categories.
func newBook(t *testing.T) *Book {
	t.Helper()
	dir := t.TempDir()
	b, err := Open(dir, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	b.Today = func() dates.Date { return dates.MustParse(today) }
	err = b.Mutate(func(d *Data, ch *Change) error {
		d.Accounts = []model.Account{
			{ID: "bank", Name: "Bank", Type: "bank", OpeningBalance: money.FromRupees(10000)},
			{ID: "card", Name: "Card", Type: "credit_card", OpeningBalance: money.FromRupees(-1000)},
		}
		d.Categories = []model.Category{
			{ID: "food", Name: "Food", Kind: model.KindExpense},
			{ID: "food-out", Name: "Out", Kind: model.KindExpense, Parent: "food"},
			{ID: "salary", Name: "Salary", Kind: model.KindIncome},
		}
		ch.Accounts, ch.Categories = true, true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func add(t *testing.T, b *Book, e Entry) string {
	t.Helper()
	var id string
	err := b.Mutate(func(d *Data, ch *Change) error {
		if e.ID == "" {
			e.ID = d.NewTxID(e.Date)
		}
		tx, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		tx.Labels = d.EnsureLabels(tx.Labels, ch)
		d.SetTx(tx, ch)
		id = tx.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func bal(b *Book, acc string) (now, proj money.Amount) {
	_ = b.View(func(d *Data, c *Computed) error {
		now, proj = c.Balance[acc], c.Projected[acc]
		return nil
	})
	return
}

func read(t *testing.T, b *Book, name string) string {
	t.Helper()
	s, err := os.ReadFile(filepath.Join(b.Dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(s)
}

func TestDoubleEntryAndSync(t *testing.T) {
	b := newBook(t)
	add(t, b, Entry{Type: model.TxIncome, Date: "2026-10-01", Title: "Pay", Amount: money.FromRupees(50000), Account: "bank", Category: "salary"})
	add(t, b, Entry{Type: model.TxExpense, Date: "2026-10-02", Title: "Lunch", Amount: money.FromRupees(300), Account: "card", Category: "food-out", Labels: []string{"Office"}})
	add(t, b, Entry{Type: model.TxTransfer, Date: "2026-10-03", Title: "Pay card", Amount: money.FromRupees(1300), Account: "bank", ToAccount: "card"})
	add(t, b, Entry{Type: model.TxExpense, Date: "2026-09-15", Title: "Backdated", Amount: money.FromRupees(200), Account: "bank", Splits: []Split{
		{Category: "food", Amount: money.FromRupees(150)}, {Category: "food-out", Amount: money.FromRupees(50)}}})

	if now, _ := bal(b, "bank"); now != money.FromRupees(10000+50000-1300-200) {
		t.Errorf("bank = %s", now)
	}
	if now, _ := bal(b, "card"); now != 0 {
		t.Errorf("card = %s", now)
	}
	// computed values are written to the definition files
	if s := read(t, b, AccountsFile); !strings.Contains(s, "balance: '58500.00'") {
		t.Errorf("accounts.md not synced:\n%s", s)
	}
	if s := read(t, b, CategoriesFile); !strings.Contains(s, "total: '300.00'") { // food (Oct) = child 300
		t.Errorf("categories.md not synced:\n%s", s)
	}
	if s := read(t, b, LabelsFile); !strings.Contains(s, "name: Office") || !strings.Contains(s, "count: 1") {
		t.Errorf("labels.md not synced:\n%s", s)
	}
	if s := read(t, b, MoneyFile); !strings.Contains(s, "balanced: true") {
		t.Errorf("money.md not balanced:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(b.Dir, TxDir, "transaction_202609.md")); err != nil {
		t.Errorf("backdated month file missing: %v", err)
	}
}

func TestUnbalancedRejected(t *testing.T) {
	b := newBook(t)
	err := b.Mutate(func(d *Data, ch *Change) error {
		tx := model.Transaction{ID: "x", Date: today, Type: model.TxJournal, Title: "bad", Postings: []model.Posting{
			{Account: "bank", Amount: -100}, {Category: "food", Amount: 90}}}
		return d.ValidateTx(tx)
	})
	if err == nil || !strings.Contains(err.Error(), "do not balance") {
		t.Fatalf("expected balance error, got %v", err)
	}
}

func TestFutureDatedCountsFromItsDate(t *testing.T) {
	b := newBook(t)
	add(t, b, Entry{Type: model.TxExpense, Date: "2026-10-20", Title: "Later", Amount: money.FromRupees(500), Account: "bank", Category: "food"})
	now, proj := bal(b, "bank")
	if now != money.FromRupees(10000) || proj != money.FromRupees(9500) {
		t.Fatalf("before: now %s proj %s", now, proj)
	}
	b.Today = func() dates.Date { return dates.MustParse("2026-10-20") }
	b.tick() // the scheduler notices the new day
	if now, _ := bal(b, "bank"); now != money.FromRupees(9500) {
		t.Fatalf("on the day: %s", now)
	}
}

func TestReminderBackfillAnchorAndIdempotency(t *testing.T) {
	b := newBook(t)
	err := b.Mutate(func(d *Data, ch *Change) error {
		d.Reminders = append(d.Reminders, model.Reminder{ID: "r-rent", Title: "Rent", Type: model.TxExpense, Amount: money.FromRupees(1000),
			Account: "bank", Category: "food", Repeat: "monthly", Next: "2026-07-31"})
		ch.Reminders = true
		d.PostDue(dates.MustParse(today), ch)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var next string
	var n int
	_ = b.View(func(d *Data, c *Computed) error {
		r, _ := d.Reminder("r-rent")
		next = r.Next
		for _, tx := range d.Txns {
			if tx.Reminder == "r-rent" {
				n++
			}
		}
		return nil
	})
	if n != 3 || next != "2026-10-31" { // Jul 31, Aug 31, Sep 30 posted; next keeps the 31st
		t.Fatalf("posted %d, next %s", n, next)
	}
	b.tick()
	b.tick()
	_ = b.View(func(d *Data, c *Computed) error {
		n = len(d.Txns)
		return nil
	})
	if n != 3 {
		t.Fatalf("re-run posted duplicates: %d", n)
	}
}

func TestBrokenFileIsNeverOverwritten(t *testing.T) {
	b := newBook(t)
	add(t, b, Entry{Type: model.TxExpense, Date: today, Title: "Tea", Amount: money.FromRupees(20), Account: "bank", Category: "food"})
	p := filepath.Join(b.Dir, TxDir, "transaction_202610.md")
	broken := read(t, b, filepath.Join(TxDir, "transaction_202610.md")) + "  - id: oops\n   bad: [\n"
	if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	err := b.Mutate(func(d *Data, ch *Change) error { ch.Accounts = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "transaction_202610.md") {
		t.Fatalf("expected a file error, got %v", err)
	}
	if got := read(t, b, filepath.Join(TxDir, "transaction_202610.md")); got != broken {
		t.Fatal("broken file was rewritten")
	}
}

func TestExternalEditPickedUpAndRepaired(t *testing.T) {
	b := newBook(t)
	// A transaction typed by hand in Obsidian: no id, wrong month file, new label.
	p := filepath.Join(b.Dir, TxDir, "transaction_202609.md")
	hand := "transactions:\n  - date: '2026-10-02'\n    type: expense\n    title: Hand typed\n    labels: [goa]\n    postings:\n      - account: bank\n        amount: -250\n      - category: food\n        amount: 250\n"
	if err := os.WriteFile(p, []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	if now, _ := bal(b, "bank"); now != money.FromRupees(9750) {
		t.Errorf("bank = %s", now)
	}
	if s := read(t, b, filepath.Join(TxDir, "transaction_202610.md")); !strings.Contains(s, "Hand typed") || !strings.Contains(s, "id: t-20261002-") {
		t.Errorf("not moved to its month with an id:\n%s", s)
	}
	if s := read(t, b, LabelsFile); !strings.Contains(s, "id: goa") {
		t.Errorf("label not added:\n%s", s)
	}
}

func TestUnknownFieldsSurvive(t *testing.T) {
	b := newBook(t)
	p := filepath.Join(b.Dir, AccountsFile)
	s := strings.Replace(read(t, b, AccountsFile), "name: Bank\n", "name: Bank\n    ifsc: HDFC0001\n", 1)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	add(t, b, Entry{Type: model.TxExpense, Date: today, Title: "Tea", Amount: money.FromRupees(20), Account: "bank", Category: "food"})
	if !strings.Contains(read(t, b, AccountsFile), "ifsc: HDFC0001") {
		t.Fatal("hand-added field lost")
	}
}

func TestEntryRoundTrip(t *testing.T) {
	b := newBook(t)
	_ = b.View(func(d *Data, c *Computed) error {
		for _, e := range []Entry{
			{ID: "a", Type: model.TxExpense, Date: today, Title: "x", Amount: 1234, Account: "bank", Category: "food", Status: "uncleared"},
			{ID: "b", Type: model.TxIncome, Date: today, Title: "x", Amount: 99, Account: "bank", Category: "salary", Status: "cleared"},
			{ID: "c", Type: model.TxTransfer, Date: today, Title: "x", Amount: 500, Account: "bank", ToAccount: "card", Status: "uncleared"},
			{ID: "d", Type: model.TxExpense, Date: today, Title: "refund", Amount: -300, Account: "card", Category: "food", Status: "uncleared"},
		} {
			tx, err := d.BuildTx(e)
			if err != nil {
				t.Fatalf("%s: %v", e.ID, err)
			}
			back := EntryOf(tx)
			if back.Amount != e.Amount || back.Account != e.Account || back.Category != e.Category || back.ToAccount != e.ToAccount {
				t.Errorf("%s: round trip %+v", e.ID, back)
			}
		}
		return nil
	})
}

func TestReadOnly(t *testing.T) {
	b := newBook(t)
	ro, _ := Open(b.Dir, "ro", true)
	if err := ro.Mutate(func(d *Data, ch *Change) error { return nil }); err != ErrReadOnly {
		t.Fatalf("got %v", err)
	}
}
