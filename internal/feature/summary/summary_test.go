package summary

import (
	"testing"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

func TestBuild(t *testing.T) {
	b, err := book.Open(t.TempDir(), "test", false)
	if err != nil {
		t.Fatal(err)
	}
	b.Today = func() dates.Date { return dates.MustParse("2026-10-12") }
	err = b.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = []model.Account{{ID: "bank", Name: "Bank", Type: "bank", OpeningBalance: money.FromRupees(50000)}}
		d.Categories = []model.Category{
			{ID: "food", Name: "Food", Kind: model.KindExpense, Budget: money.FromRupees(10000), BudgetPeriod: "monthly"},
			{ID: "fun", Name: "Fun", Kind: model.KindExpense},
			{ID: "salary", Name: "Salary", Kind: model.KindIncome},
		}
		ch.Accounts, ch.Categories = true, true
		d.Reindex()
		for _, e := range []book.Entry{
			{Type: "expense", Date: "2026-10-02", Title: "Groceries", Amount: money.FromRupees(2500), Account: "bank", Category: "food"},
			{Type: "expense", Date: "2026-10-05", Title: "Movie", Amount: money.FromRupees(500), Account: "bank", Category: "fun"},
			{Type: "income", Date: "2026-10-01", Title: "Salary", Amount: money.FromRupees(80000), Account: "bank", Category: "salary"},
			{Type: "expense", Date: "2026-09-30", Title: "Last month", Amount: money.FromRupees(999), Account: "bank", Category: "food"},
		} {
			e.ID = d.NewTxID(e.Date)
			tx, err := d.BuildTx(e)
			if err != nil {
				return err
			}
			d.SetTx(tx, ch)
		}
		for _, x := range []model.Draft{
			{ID: "a", Status: model.DraftPending, Date: "2026-10-10", Type: model.TxExpense, Amount: money.FromRupees(120), Title: "Tea"},
			{ID: "b", Status: model.DraftPending, Date: "2026-10-11", Time: "09:00", Type: model.TxExpense, Amount: money.FromRupees(800), Title: "Fuel"},
			{ID: "c", Status: model.DraftPending, Date: "2026-10-11", Time: "18:00", Type: model.TxIncome, Amount: money.FromRupees(300), Title: "Refund"},
			{ID: "d", Status: model.DraftAccepted, Date: "2026-10-11", Type: model.TxExpense, Amount: money.FromRupees(5000), Title: "Done"},
		} {
			d.SetDraft(x, ch)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var s Summary
	_ = b.View(func(d *book.Data, c *book.Computed) error { s = Build(d, c, 2); return nil })

	if s.Period.From.String() != "2026-10-01" || s.Expense != money.FromRupees(3000) || s.Income != money.FromRupees(80000) {
		t.Errorf("month = %s %s in %s out", s.Period.Label, s.Income, s.Expense)
	}
	if s.Budget.Budget != money.FromRupees(10000) || s.Budget.Spent != money.FromRupees(2500) || s.Budget.Percent != 25 {
		t.Errorf("budget = %+v (only budgeted categories count)", s.Budget)
	}
	in := s.Inbox
	if in.Pending != 3 || in.Out != money.FromRupees(920) || in.In != money.FromRupees(300) {
		t.Errorf("inbox = %+v", in)
	}
	w := s.Worth
	if w.Now != money.FromRupees(50000+80000-3000-999) || w.Change != money.FromRupees(80000-3000) || len(w.Trend) != 12 {
		t.Errorf("net worth = %+v", w)
	}
	if w.Trend[10] != money.FromRupees(50000-999) || w.Trend[11] != w.Now {
		t.Errorf("trend = %v", w.Trend)
	}
	if len(in.Newest) != 2 || in.Newest[0].Title != "Refund" || in.Newest[1].Title != "Fuel" {
		t.Errorf("newest = %+v", in.Newest)
	}
}

func TestAlert(t *testing.T) {
	b, err := book.Open(t.TempDir(), "test", false)
	if err != nil {
		t.Fatal(err)
	}
	b.Today = func() dates.Date { return dates.MustParse("2026-10-12") }
	bank := model.Reported(money.FromRupees(900))
	err = b.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = []model.Account{
			{ID: "bank", Name: "Bank", Type: "bank", OpeningBalance: money.FromRupees(1000), BankBalance: &bank, BankDate: "2026-10-10"},
			{ID: "cash", Name: "Cash", Type: "cash", OpeningBalance: money.FromRupees(50)},
		}
		ch.Accounts = true
		for i := 0; i < 3; i++ {
			d.SetDraft(model.Draft{ID: string(rune('a' + i)), Status: model.DraftPending, Date: "2026-10-11", Type: model.TxExpense, Amount: money.FromRupees(10), Title: "x"}, ch)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	build := func(a *model.Alerts) Alert {
		var s Summary
		_ = b.Mutate(func(d *book.Data, ch *book.Change) error { d.Settings.Alerts, ch.Settings = a, true; return nil })
		_ = b.View(func(d *book.Data, c *book.Computed) error { s = Build(d, c, 0); return nil })
		return s.Alert
	}
	a := build(&model.Alerts{Time: "08:30", Mismatch: true, Count: 30})
	if !a.Show || a.Time != "08:30" || len(a.Mismatch) != 1 || a.Mismatch[0] != "Bank" || a.Count != 3 {
		t.Errorf("mismatch alert = %+v", a)
	}
	if a = build(&model.Alerts{Time: "08:30", Mismatch: false, Count: 30}); a.Show || len(a.Mismatch) != 0 {
		t.Errorf("both off = %+v", a)
	}
	if a = build(&model.Alerts{Time: "08:30", Mismatch: false, Count: 2}); !a.Show || a.Text != "3 items wait in the inbox and notifications" {
		t.Errorf("count alert = %+v", a)
	}
	if a = build(&model.Alerts{Time: "08:30", Count: 0}); a.Show {
		t.Errorf("count 0 means off: %+v", a)
	}
}
