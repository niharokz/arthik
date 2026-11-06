// Package state serves GET /api/state: everything the app needs at start-up in one
// call (settings, accounts with balances, categories with recent use, the inbox and reminder counts,
// the overview numbers and any data problems). The PWA caches this for offline use.
package state

import (
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/inbox"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/plugin"
	"gitlab.com/niharokz/arthik/internal/view"
)

// Version is set by main.
var Version = "dev"

type summary struct {
	NetWorth    money.Amount `json:"net_worth"`
	Excluded    money.Amount `json:"excluded"`     // accounts left out of net worth, together
	ExcludedIDs []string     `json:"excluded_ids"` // … which ones
	ChecksOff   int          `json:"checks_off"`   // accounts whose bank check does not match
	Unchecked   int          `json:"unchecked"`    // never checked, or checked too long ago
	Assets      money.Amount `json:"assets"`
	Liabilities money.Amount `json:"liabilities"`
	Period      period.Range `json:"period"`
	Income      money.Amount `json:"income"`
	Expense     money.Amount `json:"expense"`
	Net         money.Amount `json:"net"`
}

type category struct {
	model.Category
	Total money.Amount `json:"total"` // in the default period, parents include children
}

// Register adds the routes.
func Register(rt *httpx.Router) {
	// GET /api/period resolves a period (for views that only need the range and
	// the previous/next ranges): ?period=&date=&from=&to=&offset=
	rt.Handle("GET /api/period", func(c *httpx.Ctx) error {
		var out map[string]any
		err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
			o := d.Opts()
			r := period.Resolve(c.Q("period"), c.Q("date"), c.Q("from"), c.Q("to"), c.QInt("offset", 0),
				period.Kind(d.Settings.DefaultPeriod), comp.Today, o)
			out = map[string]any{"range": r, "prev": r.Shift(-1, o), "next": r.Shift(1, o)}
			return nil
		})
		if err != nil {
			return err
		}
		return c.OK(out)
	})

	rt.Handle("GET /api/state", func(c *httpx.Ctx) error {
		out := map[string]any{}
		err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
			accs := make([]view.Account, 0, len(d.Accounts))
			for _, a := range d.Accounts {
				accs = append(accs, view.AccountOf(a, d, comp))
			}
			cats := make([]category, 0, len(d.Categories))
			for _, ct := range d.Categories {
				cats = append(cats, category{Category: ct, Total: comp.CatTotals[ct.ID]})
			}
			out["user"] = c.User.Name
			out["read_only"] = c.User.ReadOnly
			out["version"] = Version
			out["today"] = comp.Today
			out["settings"] = d.Settings
			out["accounts"] = accs
			out["categories"] = cats
			pending := 0
			for _, x := range d.Drafts {
				if x.Status == model.DraftPending {
					pending++
				}
			}
			out["inbox"] = map[string]int{"pending": pending}
			out["notices"] = map[string]int{"active": inbox.ActiveNotices(d, comp)}
			out["plugins"] = plugin.Enabled(c.Book.Dir)
			out["account_types"] = model.AccountTypes
			out["period_kinds"] = period.Kinds
			out["look_options"] = map[string]any{ // what Settings → Look offers
				"palettes": model.Palettes, "cards": model.CardStyles, "colors": model.LookColors, "charts": model.Charts,
				"densities": model.Densities,
			}
			out["category_uses"] = categoryUses(d, comp)
			s := summary{
				NetWorth: comp.NetWorth, Assets: comp.Assets, Liabilities: comp.Liabilities,
				Period: comp.Period, Income: comp.Income, Expense: comp.Expense, Net: comp.Income - comp.Expense,
			}
			s.Excluded, s.ExcludedIDs = view.Excluded(d, comp)
			s.ChecksOff, s.Unchecked = view.Checks(d, comp)
			out["summary"] = s
			return nil
		})
		if err != nil {
			return err
		}
		problems := c.Book.Problems()
		if problems == nil {
			problems = []book.Problem{}
		}
		out["problems"] = problems
		if le := c.Book.LoadError(); le != nil {
			out["load_error"] = le.Error()
		}
		return c.OK(out)
	})
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// pinWindow is how far back category use is counted for the picker's pinned row.
const pinWindow = 90

// categoryUses counts, per category, the transactions using it in the last pinWindow
// days (the category picker pins the most used ones).
func categoryUses(d *book.Data, comp *book.Computed) map[string]int {
	from := comp.Today.AddDays(-pinWindow)
	out := map[string]int{}
	for i := len(d.Txns) - 1; i >= 0; i-- {
		t := d.Txns[i]
		td, err := dates.Parse(t.Date)
		if err != nil || td.After(comp.Today) {
			continue
		}
		if td.Before(from) {
			break // sorted by date
		}
		for _, p := range t.Postings {
			if p.Category != "" {
				out[p.Category]++
			}
		}
	}
	return out
}
