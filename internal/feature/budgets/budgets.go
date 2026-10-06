// Package budgets is the Budgets feature: budget vs actual per category for any
// period. A budget is set on a category with its own period (e.g. ₹12,000 monthly)
// and converted to whichever period you view. Budgets reset every period (no rollover).
package budgets

import (
	"sort"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/view"
)

// Line is one budgeted category.
type Line struct {
	Category string       `json:"category"`
	Parent   string       `json:"parent,omitempty"`
	Kind     string       `json:"kind"`
	Budget   money.Amount `json:"budget"`
	Spent    money.Amount `json:"spent"` // natural: spent for expense, received for income
	Left     money.Amount `json:"left"`
	Percent  int          `json:"percent"`
	Derived  bool         `json:"derived,omitempty"` // parent budget = sum of its children's budgets
}

// Register adds the route.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/budgets", func(c *httpx.Ctx) error {
		var out map[string]any
		err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
			o := d.Opts()
			r := period.Resolve(c.Q("period"), c.Q("date"), c.Q("from"), c.Q("to"), c.QInt("offset", 0),
				period.Kind(d.Settings.DefaultPeriod), comp.Today, o)
			out = Compute(d, comp, r)
			out["prev"], out["next"] = r.Shift(-1, o), r.Shift(1, o)
			return nil
		})
		if err != nil {
			return err
		}
		return c.OK(out)
	})
}

// Compute builds the budget report for range r (also used by the dashboard).
func Compute(d *book.Data, comp *book.Computed, r period.Range) map[string]any {
	spent := d.RollUp(d.CategoryTotals(comp, r.From, r.To, nil))
	var lines []Line
	var totalBudget, totalSpent money.Amount
	pct := func(s, b money.Amount) int {
		if b <= 0 {
			return 0
		}
		return int(int64(s) * 100 / int64(b))
	}
	for _, p := range d.Categories {
		if p.Parent != "" {
			continue
		}
		pb := view.BudgetFor(p, r)
		var kidLines []Line
		var kidSum, kidSpent money.Amount
		for _, kid := range d.Children(p.ID) {
			kc, _ := d.Category(kid)
			if b := view.BudgetFor(kc, r); b > 0 {
				s := spent[kid]
				kidLines = append(kidLines, Line{Category: kid, Parent: p.ID, Kind: kc.Kind, Budget: b, Spent: s, Left: b - s, Percent: pct(s, b)})
				kidSum += b
				kidSpent += s
			}
		}
		derived := false
		if pb == 0 && kidSum > 0 {
			pb, derived = kidSum, true
		}
		if pb == 0 {
			continue
		}
		s := spent[p.ID]
		if derived {
			s = kidSpent
		}
		lines = append(lines, Line{Category: p.ID, Kind: p.Kind, Budget: pb, Spent: s, Left: pb - s, Percent: pct(s, pb), Derived: derived})
		lines = append(lines, kidLines...)
		if p.Kind == model.KindExpense {
			totalBudget += pb
			totalSpent += s
		}
	}
	// Order: most used (by %) first within parents, parents kept with their children.
	sort.SliceStable(lines, func(i, j int) bool {
		pi, pj := groupOf(lines[i]), groupOf(lines[j])
		if pi != pj {
			if a, b := pctOf(lines, pi), pctOf(lines, pj); a != b {
				return a > b
			}
			return pi < pj
		}
		if (lines[i].Parent == "") != (lines[j].Parent == "") {
			return lines[i].Parent == ""
		}
		return lines[i].Percent > lines[j].Percent
	})
	if lines == nil {
		lines = []Line{}
	}
	return map[string]any{
		"range": r, "lines": lines,
		"total": Line{Kind: model.KindExpense, Budget: totalBudget, Spent: totalSpent, Left: totalBudget - totalSpent, Percent: pct(totalSpent, totalBudget)},
	}
}

func groupOf(l Line) string {
	if l.Parent != "" {
		return l.Parent
	}
	return l.Category
}

func pctOf(lines []Line, parent string) int {
	for _, l := range lines {
		if l.Category == parent && l.Parent == "" {
			return l.Percent
		}
	}
	return 0
}
