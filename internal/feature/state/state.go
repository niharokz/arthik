// Package state serves GET /api/state: everything the app needs at start-up in one
// call (settings, accounts with balances, categories, labels, reminders, the
// overview numbers and any data problems). The PWA caches this for offline use.
package state

import (
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/recur"
	"gitlab.com/niharokz/arthik/internal/view"
)

// Version is set by main.
var Version = "dev"

type summary struct {
	NetWorth    money.Amount `json:"net_worth"`
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
			out["labels"] = nonNil(d.Labels)
			out["reminders"] = nonNil(d.Reminders)
			out["account_types"] = model.AccountTypes
			out["period_kinds"] = period.Kinds
			out["repeat_rules"] = recur.Names
			out["summary"] = summary{
				NetWorth: comp.NetWorth, Assets: comp.Assets, Liabilities: comp.Liabilities,
				Period: comp.Period, Income: comp.Income, Expense: comp.Expense, Net: comp.Income - comp.Expense,
			}
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
