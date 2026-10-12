// Package summary is the short status the phone shows outside the app (home-screen
// widget, morning notification): this month's spending against the budgets and the
// drafts waiting in the inbox. Served as GET /api/summary and by `arthik summary`.
package summary

import (
	"sort"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/feature/budgets"
	"gitlab.com/niharokz/arthik/internal/feature/reports"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
)

// Summary is everything the widget and the notification need.
type Summary struct {
	Today   string       `json:"today"`
	Period  period.Range `json:"period"` // the current month (salary-cycle aware)
	Income  money.Amount `json:"income"`
	Expense money.Amount `json:"expense"`
	Budget  budgets.Line `json:"budget"` // expense budgets only; Budget = 0 when none are set
	Inbox   Inbox        `json:"inbox"`
	Worth   Worth        `json:"net_worth"`
}

// Worth is net worth now, its change this month and the last 12 month-ends.
type Worth struct {
	Now         money.Amount   `json:"now"`
	Assets      money.Amount   `json:"assets"`
	Liabilities money.Amount   `json:"liabilities"`
	Change      money.Amount   `json:"change"` // since the end of last month
	Trend       []money.Amount `json:"trend"`  // oldest first, the last one is today
}

// Inbox is the review queue at a glance.
type Inbox struct {
	Pending int          `json:"pending"`
	Out     money.Amount `json:"out"` // pending expenses
	In      money.Amount `json:"in"`  // pending income
	Newest  []Draft      `json:"newest"`
}

// Draft is one pending draft in a few words.
type Draft struct {
	Title  string       `json:"title"`
	Type   string       `json:"type"`
	Amount money.Amount `json:"amount"`
	Date   string       `json:"date"`
}

// Register adds the route.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/summary", func(c *httpx.Ctx) error {
		var out Summary
		err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
			out = Build(d, comp, c.QInt("newest", 3))
			return nil
		})
		if err != nil {
			return err
		}
		return c.OK(out)
	})
}

// Build computes the summary for today; newest is how many pending drafts to list.
func Build(d *book.Data, comp *book.Computed, newest int) Summary {
	r := period.Of(period.Monthly, comp.Today, d.Opts())
	s := Summary{Today: comp.Today.String(), Period: r}
	s.Income, s.Expense = d.IncomeExpenseIn(comp, r.From, r.To)
	if total, ok := budgets.Compute(d, comp, r)["total"].(budgets.Line); ok {
		s.Budget = total
	}
	s.Worth = worth(d, comp, r)
	var pending []model.Draft
	for _, x := range d.Drafts {
		if x.Status != model.DraftPending {
			continue
		}
		pending = append(pending, x)
		switch x.Type {
		case model.TxExpense:
			s.Inbox.Out += x.Amount
		case model.TxIncome:
			s.Inbox.In += x.Amount
		}
	}
	s.Inbox.Pending = len(pending)
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].Date != pending[j].Date {
			return pending[i].Date > pending[j].Date
		}
		return pending[i].Time > pending[j].Time
	})
	s.Inbox.Newest = []Draft{}
	for i := 0; i < len(pending) && i < newest; i++ {
		x := pending[i]
		s.Inbox.Newest = append(s.Inbox.Newest, Draft{Title: x.Title, Type: x.Type, Amount: x.Amount, Date: x.Date})
	}
	return s
}

func worth(d *book.Data, comp *book.Computed, month period.Range) Worth {
	o := d.Opts()
	rows := reports.NetWorth(d, comp, period.Monthly, month.Shift(-11, o).From, comp.Today)
	w := Worth{Now: comp.NetWorth, Assets: comp.Assets, Liabilities: comp.Liabilities, Trend: []money.Amount{}}
	for _, r := range rows {
		w.Trend = append(w.Trend, r.NetWorth)
	}
	if n := len(rows); n >= 2 {
		w.Change = rows[n-1].NetWorth - rows[n-2].NetWorth
	}
	return w
}
