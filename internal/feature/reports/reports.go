// Package reports is the Reports feature (Bluecoins "Reports" tab): cash flow,
// income/expense by category with drill-down, category trend, net worth over time,
// balance sheet, label report, account flow and the calendar.
package reports

import (
	"sort"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/query"
)

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/reports/cashflow", cashflow)
	rt.Handle("GET /api/reports/categories", categories)
	rt.Handle("GET /api/reports/trend", trend)
	rt.Handle("GET /api/reports/networth", networth)
	rt.Handle("GET /api/reports/balance-sheet", balanceSheet)
	rt.Handle("GET /api/reports/labels", labels)
	rt.Handle("GET /api/reports/accounts", accountFlow)
	rt.Handle("GET /api/reports/calendar", calendar)
}

// span reads the report range: an explicit from/to, or period+date(+offset), or
// the last 12 months ending today.
func span(c *httpx.Ctx, d *book.Data, comp *book.Computed) period.Range {
	if c.Q("period") != "" {
		return period.Resolve(c.Q("period"), c.Q("date"), c.Q("from"), c.Q("to"), c.QInt("offset", 0),
			period.Kind(d.Settings.DefaultPeriod), comp.Today, d.Opts())
	}
	from, err1 := dates.Parse(c.Q("from"))
	to, err2 := dates.Parse(c.Q("to"))
	if err2 != nil {
		to = period.Of(period.Monthly, comp.Today, d.Opts()).To
	}
	if err1 != nil {
		from = period.Of(period.Monthly, to, d.Opts()).Shift(-11, d.Opts()).From
	}
	return period.Between(from, to)
}

func bucketKind(c *httpx.Ctx) period.Kind {
	k := period.Kind(c.Q("bucket"))
	if !k.Valid() || k == period.Custom {
		return period.Monthly
	}
	return k
}

// filtered returns a keep-func from the shared filter (account/label/status/q),
// so every report can be narrowed the same way as the transaction list.
func filtered(c *httpx.Ctx, d *book.Data, comp *book.Computed) func(model.Transaction) bool {
	f := query.Parse(c.R.URL.Query())
	f.From, f.To = dates.Date{}, dates.Date{}
	if len(f.Accounts)+len(f.Labels)+len(f.Statuses)+len(f.Types) == 0 && f.Text == "" {
		return nil
	}
	f.Categories = nil
	m := query.Compile(f, d, comp)
	return m.Match
}

type flowRow struct {
	Label   string       `json:"label"`
	From    dates.Date   `json:"from"`
	To      dates.Date   `json:"to"`
	Income  money.Amount `json:"income"`
	Expense money.Amount `json:"expense"`
	Net     money.Amount `json:"net"`
}

func incExp(d *book.Data, totals map[string]money.Amount) (inc, exp money.Amount) {
	for id, v := range totals {
		cat, _ := d.Category(id)
		switch {
		case cat.Kind == model.KindExpense:
			exp += v
		case cat.Kind == model.KindIncome || v >= 0:
			inc += v
		default:
			exp += -v
		}
	}
	return
}

// cashflow: income, expense and net per bucket.
func cashflow(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := span(c, d, comp)
		keep := filtered(c, d, comp)
		var rows []flowRow
		var tot flowRow
		for _, b := range period.Buckets(bucketKind(c), r.From, r.To, d.Opts()) {
			inc, exp := incExp(d, d.CategoryTotals(comp, b.From, b.To, keep))
			rows = append(rows, flowRow{b.Label, b.From, b.To, inc, exp, inc - exp})
			tot.Income += inc
			tot.Expense += exp
		}
		tot.Net = tot.Income - tot.Expense
		tot.Label = r.Label
		out = map[string]any{"range": r, "rows": nonNil(rows), "total": tot}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type catRow struct {
	Category string       `json:"category"`
	Total    money.Amount `json:"total"`
	Percent  float64      `json:"percent"`
	HasKids  bool         `json:"has_children"`
}

// categories: totals by category for a kind (expense|income|both), top level by
// default or the children of ?parent=.
func categories(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := span(c, d, comp)
		kind, parent := c.Q("kind"), c.Q("parent")
		if kind == "" {
			kind = model.KindExpense
		}
		own := d.CategoryTotals(comp, r.From, r.To, filtered(c, d, comp))
		var parentOwn money.Amount
		if parent != "" {
			parentOwn = own[parent]
		}
		all := d.RollUp(own)
		var rows []catRow
		var sum money.Amount
		for _, cat := range d.Categories {
			if cat.Kind != kind && parent == "" {
				continue
			}
			if (parent == "" && cat.Parent != "") || (parent != "" && cat.Parent != parent) {
				continue
			}
			v := all[cat.ID]
			if v == 0 {
				continue
			}
			rows = append(rows, catRow{Category: cat.ID, Total: v, HasKids: len(d.Children(cat.ID)) > 0})
			sum += v
		}
		if parent != "" && parentOwn != 0 { // money posted on the parent itself
			rows = append(rows, catRow{Category: parent, Total: parentOwn})
			sum += parentOwn
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Total > rows[j].Total })
		for i := range rows {
			if sum != 0 {
				rows[i].Percent = float64(int64(rows[i].Total)*1000/int64(sum)) / 10
			}
		}
		out = map[string]any{"range": r, "kind": kind, "parent": parent, "rows": nonNil(rows), "total": sum,
			"prev": r.Shift(-1, d.Opts()), "next": r.Shift(1, d.Opts())}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type trendRow struct {
	Label string       `json:"label"`
	From  dates.Date   `json:"from"`
	To    dates.Date   `json:"to"`
	Total money.Amount `json:"total"`
}

// trend: one category (with its children) per bucket.
func trend(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		id := c.Q("category")
		if _, ok := d.Category(id); !ok {
			return httpx.NotFound("category")
		}
		r := span(c, d, comp)
		keep := filtered(c, d, comp)
		var rows []trendRow
		var sum money.Amount
		for _, b := range period.Buckets(bucketKind(c), r.From, r.To, d.Opts()) {
			v := d.RollUp(d.CategoryTotals(comp, b.From, b.To, keep))[id]
			rows = append(rows, trendRow{b.Label, b.From, b.To, v})
			sum += v
		}
		avg := money.Amount(0)
		if len(rows) > 0 {
			avg = sum.MulDiv(1, int64(len(rows)))
		}
		out = map[string]any{"range": r, "category": id, "rows": nonNil(rows), "total": sum, "average": avg}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type nwRow struct {
	Label       string       `json:"label"`
	To          dates.Date   `json:"to"`
	Assets      money.Amount `json:"assets"`
	Liabilities money.Amount `json:"liabilities"`
	NetWorth    money.Amount `json:"net_worth"`
}

// balancesAt returns every account's balance at the end of day `at`.
func balancesAt(d *book.Data, comp *book.Computed, at dates.Date) map[string]money.Amount {
	bal := map[string]money.Amount{}
	for _, a := range d.Accounts {
		bal[a.ID] = a.OpeningBalance
	}
	for _, t := range d.Txns {
		td, err := dates.Parse(t.Date)
		if err != nil || td.After(at) {
			continue
		}
		if !comp.Valid[t.ID] {
			continue
		}
		for _, p := range t.Postings {
			if p.Account != "" {
				bal[p.Account] += p.Amount
			}
		}
	}
	return bal
}

// networth: assets, liabilities and net worth at the end of each bucket.
func networth(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := span(c, d, comp)
		var rows []nwRow
		for _, b := range period.Buckets(bucketKind(c), r.From, r.To, d.Opts()) {
			at := dates.Min(b.To, comp.Today)
			bal := balancesAt(d, comp, at)
			row := nwRow{Label: b.Label, To: at}
			for _, a := range d.Accounts {
				if a.ExcludeNetWorth {
					continue
				}
				if a.Class() == model.ClassLiability {
					row.Liabilities += bal[a.ID]
				} else {
					row.Assets += bal[a.ID]
				}
			}
			row.NetWorth = row.Assets + row.Liabilities
			rows = append(rows, row)
		}
		out = map[string]any{"range": r, "rows": nonNil(rows)}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type sheetAcc struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Balance money.Amount `json:"balance"`
}
type sheetGroup struct {
	Type     string       `json:"type"`
	Label    string       `json:"label"`
	Class    string       `json:"class"`
	Accounts []sheetAcc   `json:"accounts"`
	Total    money.Amount `json:"total"`
}

// balanceSheet: balances grouped by account type at ?date= (default today).
func balanceSheet(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		at := comp.Today
		if v, err := dates.Parse(c.Q("date")); err == nil {
			at = v
		}
		bal := balancesAt(d, comp, at)
		var groups []sheetGroup
		var assets, liabs money.Amount
		for _, ty := range model.AccountTypes {
			g := sheetGroup{Type: ty.ID, Label: ty.Label, Class: ty.Class}
			for _, a := range d.Accounts {
				if a.Type != ty.ID || a.ExcludeNetWorth || (a.Archived && bal[a.ID] == 0) {
					continue
				}
				g.Accounts = append(g.Accounts, sheetAcc{a.ID, a.Name, bal[a.ID]})
				g.Total += bal[a.ID]
			}
			if len(g.Accounts) == 0 {
				continue
			}
			if ty.Class == model.ClassLiability {
				liabs += g.Total
			} else {
				assets += g.Total
			}
			groups = append(groups, g)
		}
		out = map[string]any{"date": at, "groups": nonNil(groups), "assets": assets, "liabilities": liabs, "net_worth": assets + liabs}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type labelRow struct {
	Label   string       `json:"label"`
	Income  money.Amount `json:"income"`
	Expense money.Amount `json:"expense"`
	Count   int          `json:"count"`
}

// labels: income, expense and count per label in the range.
func labels(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := span(c, d, comp)
		rows := map[string]*labelRow{}
		for _, t := range d.Txns {
			td, err := dates.Parse(t.Date)
			if err != nil || !comp.Effective(t) || !td.InRange(r.From, r.To) || len(t.Labels) == 0 {
				continue
			}
			var inc, exp money.Amount
			for _, p := range t.Postings {
				if p.Category != "" {
					cat, _ := d.Category(p.Category)
					i, e := book.IncomeExpense(cat.Kind, p.Amount)
					inc, exp = inc+i, exp+e
				}
			}
			for _, l := range t.Labels {
				lr := rows[l]
				if lr == nil {
					lr = &labelRow{Label: l}
					rows[l] = lr
				}
				lr.Income += inc
				lr.Expense += exp
				lr.Count++
			}
		}
		list := []labelRow{}
		for _, v := range rows {
			list = append(list, *v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Expense+list[i].Income > list[j].Expense+list[j].Income })
		out = map[string]any{"range": r, "rows": list, "prev": r.Shift(-1, d.Opts()), "next": r.Shift(1, d.Opts())}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type accRow struct {
	Account string       `json:"account"`
	Opening money.Amount `json:"opening"` // balance at the start of the range
	Inflow  money.Amount `json:"inflow"`
	Outflow money.Amount `json:"outflow"`
	Closing money.Amount `json:"closing"`
}

// accountFlow: opening, in, out and closing per account for the range.
func accountFlow(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := span(c, d, comp)
		open := balancesAt(d, comp, r.From.AddDays(-1))
		rows := map[string]*accRow{}
		for _, a := range d.Accounts {
			rows[a.ID] = &accRow{Account: a.ID, Opening: open[a.ID]}
		}
		for _, t := range d.Txns {
			td, err := dates.Parse(t.Date)
			if err != nil || !comp.Effective(t) || !td.InRange(r.From, r.To) {
				continue
			}
			for _, p := range t.Postings {
				if ar := rows[p.Account]; ar != nil {
					if p.Amount > 0 {
						ar.Inflow += p.Amount
					} else {
						ar.Outflow += -p.Amount
					}
				}
			}
		}
		list := []accRow{}
		for _, a := range d.Accounts {
			ar := rows[a.ID]
			ar.Closing = ar.Opening + ar.Inflow - ar.Outflow
			if a.Archived && ar.Inflow == 0 && ar.Outflow == 0 && ar.Closing == 0 {
				continue
			}
			list = append(list, *ar)
		}
		out = map[string]any{"range": r, "rows": list, "prev": r.Shift(-1, d.Opts()), "next": r.Shift(1, d.Opts())}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type dayRow struct {
	Date     string       `json:"date"`
	Income   money.Amount `json:"income"`
	Expense  money.Amount `json:"expense"`
	Count    int          `json:"count"`
	Upcoming int          `json:"upcoming"` // scheduled/reminder items on that day
}

// calendar: per-day totals for ?month=YYYY-MM (default this month), including
// future-dated transactions and reminder occurrences.
func calendar(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		first := dates.New(comp.Today.Year(), comp.Today.Month(), 1)
		if v, err := dates.Parse(c.Q("month") + "-01"); err == nil {
			first = v
		}
		last := first.AddMonths(1, 1).AddDays(-1)
		days := map[string]*dayRow{}
		get := func(s string) *dayRow {
			if days[s] == nil {
				days[s] = &dayRow{Date: s}
			}
			return days[s]
		}
		keep := filtered(c, d, comp)
		for _, t := range d.Txns {
			td, err := dates.Parse(t.Date)
			if err != nil || !comp.Valid[t.ID] || !td.InRange(first, last) || (keep != nil && !keep(t)) {
				continue
			}
			row := get(t.Date)
			if td.After(comp.Today) {
				row.Upcoming++
			}
			row.Count++
			for _, p := range t.Postings {
				if p.Category != "" {
					cat, _ := d.Category(p.Category)
					i, e := book.IncomeExpense(cat.Kind, p.Amount)
					row.Income += i
					row.Expense += e
				}
			}
		}
		for _, e := range d.Upcoming(dates.Max(first, comp.Today.AddDays(1)), last) {
			if _, posted := d.Tx(e.ID); posted {
				continue
			}
			row := get(e.Date)
			row.Upcoming++
			row.Count++
			if e.Type == model.TxExpense {
				row.Expense += e.Amount
			} else if e.Type == model.TxIncome {
				row.Income += e.Amount
			}
		}
		list := []dayRow{}
		for _, v := range days {
			list = append(list, *v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Date < list[j].Date })
		out = map[string]any{"month": first.Format("2006-01"), "from": first, "to": last, "days": list,
			"prev": first.AddMonths(-1, 1).Format("2006-01"), "next": first.AddMonths(1, 1).Format("2006-01")}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
