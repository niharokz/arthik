// Package categories is the Categories feature: the two-level income/expense tree,
// per-period totals (weekly … yearly or a custom range) and delete-with-move.
package categories

import (
	"fmt"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/view"
)

type input struct {
	Name         string       `json:"name"`
	Kind         string       `json:"kind"`
	Parent       string       `json:"parent"`
	Color        string       `json:"color"`
	Icon         string       `json:"icon"`
	Budget       money.Amount `json:"budget"`
	BudgetPeriod string       `json:"budget_period"`
	Order        int          `json:"order"`
	Hidden       bool         `json:"hidden"`
}

func (in input) apply(d *book.Data, c *model.Category) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("enter a category name")
	}
	switch in.Kind {
	case model.KindExpense, model.KindIncome, model.KindBoth:
	default:
		return fmt.Errorf("kind must be expense, income or both")
	}
	if in.Parent != "" {
		if in.Parent == c.ID {
			return fmt.Errorf("a category cannot be its own parent")
		}
		p, ok := d.Category(in.Parent)
		if !ok {
			return fmt.Errorf("parent category not found")
		}
		if p.Parent != "" {
			return fmt.Errorf("categories have two levels: choose a top-level parent")
		}
		if c.ID != "" && len(d.Children(c.ID)) > 0 {
			return fmt.Errorf("%q has sub-categories, so it must stay top-level", c.Name)
		}
		in.Kind = p.Kind // children always share the parent's kind
	}
	if in.Budget < 0 {
		return fmt.Errorf("budget cannot be negative")
	}
	if in.BudgetPeriod != "" {
		k := period.Kind(in.BudgetPeriod)
		if !k.Valid() || k == period.Custom {
			return fmt.Errorf("invalid budget period %q", in.BudgetPeriod)
		}
	}
	if in.Budget == 0 {
		in.BudgetPeriod = ""
	} else if in.BudgetPeriod == "" {
		in.BudgetPeriod = string(period.Monthly)
	}
	c.Name, c.Kind, c.Parent = in.Name, in.Kind, in.Parent
	c.Color, c.Icon = strings.TrimSpace(in.Color), strings.TrimSpace(in.Icon)
	c.Budget, c.BudgetPeriod, c.Order, c.Hidden = in.Budget, in.BudgetPeriod, in.Order, in.Hidden
	return nil
}

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("POST /api/categories", create)
	rt.Handle("PUT /api/categories/{id}", update)
	rt.Handle("DELETE /api/categories/{id}", remove)
	rt.Handle("GET /api/categories/totals", totals)
}

func create(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	var id string
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		var cat model.Category
		if err := in.apply(d, &cat); err != nil {
			return err
		}
		for _, x := range d.Categories {
			if x.Parent == cat.Parent && strings.EqualFold(x.Name, cat.Name) {
				return fmt.Errorf("category %q already exists there", cat.Name)
			}
		}
		cat.ID = d.UniqueCategoryID(cat.Name, cat.Parent)
		id = cat.ID
		d.Categories = append(d.Categories, cat)
		ch.Categories = true
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]string{"id": id})
}

func update(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	id := c.R.PathValue("id")
	return okOr(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		for i, cat := range d.Categories {
			if cat.ID != id {
				continue
			}
			oldKind := cat.Kind
			if err := in.apply(d, &cat); err != nil {
				return err
			}
			d.Categories[i] = cat
			// A parent's kind change flows to its children.
			if cat.Kind != oldKind {
				for j, k := range d.Categories {
					if k.Parent == id {
						k.Kind = cat.Kind
						d.Categories[j] = k
					}
				}
			}
			ch.Categories = true
			return nil
		}
		return httpx.NotFound("category")
	}))
}

// remove deletes a category. If transactions or reminders use it, ?move_to=<id>
// is required and they are moved there first (Bluecoins "delete and move").
func remove(c *httpx.Ctx) error {
	id, moveTo := c.R.PathValue("id"), c.Q("move_to")
	return okOr(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		cat, ok := d.Category(id)
		if !ok {
			return httpx.NotFound("category")
		}
		if len(d.Children(id)) > 0 {
			return fmt.Errorf("%q has sub-categories — delete or move those first", cat.Name)
		}
		if moveTo != "" {
			if moveTo == id {
				return fmt.Errorf("choose a different category to move to")
			}
			if _, ok := d.Category(moveTo); !ok {
				return fmt.Errorf("target category not found")
			}
		}
		used := 0
		for _, t := range d.Txns {
			touch := false
			for _, p := range t.Postings {
				if p.Category == id {
					touch = true
				}
			}
			if !touch {
				continue
			}
			used++
			if moveTo == "" {
				continue
			}
			nt := t
			nt.Postings = make([]model.Posting, len(t.Postings))
			for i, p := range t.Postings {
				if p.Category == id {
					p.Category = moveTo
				}
				nt.Postings[i] = p
			}
			d.SetTx(nt, ch)
		}
		for i, r := range d.Reminders {
			if r.Category == id {
				if moveTo == "" {
					return fmt.Errorf("reminder %q uses this category — choose a category to move it to", r.Title)
				}
				r.Category = moveTo
				d.Reminders[i] = r
				ch.Reminders = true
			}
		}
		if used > 0 && moveTo == "" {
			return httpx.Errorf(409, "%d transaction(s) use %q — choose a category to move them to", used, cat.Name)
		}
		for i, x := range d.Categories {
			if x.ID == id {
				d.Categories = append(d.Categories[:i:i], d.Categories[i+1:]...)
				break
			}
		}
		ch.Categories = true
		return nil
	}))
}

// totals returns category totals for a period: ?period=weekly|monthly|quarterly|
// halfyearly|yearly|custom&date=YYYY-MM-DD (or from/to)&offset=-1.
func totals(c *httpx.Ctx) error {
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		r := period.Resolve(c.Q("period"), c.Q("date"), c.Q("from"), c.Q("to"), c.QInt("offset", 0),
			period.Kind(d.Settings.DefaultPeriod), comp.Today, d.Opts())
		own := d.CategoryTotals(comp, r.From, r.To, nil)
		var inc, exp money.Amount
		for id, v := range own {
			cat, _ := d.Category(id)
			kind := cat.Kind
			if kind == model.KindExpense {
				exp += v
			} else if v >= 0 || kind == model.KindIncome {
				inc += v
			} else {
				exp += -v
			}
		}
		all := d.RollUp(own)
		budgets := map[string]money.Amount{}
		for _, cat := range d.Categories {
			if b := view.BudgetFor(cat, r); b > 0 {
				budgets[cat.ID] = b
			}
		}
		out = map[string]any{
			"range": r, "prev": r.Shift(-1, d.Opts()), "next": r.Shift(1, d.Opts()),
			"totals": all, "budgets": budgets, "income": inc, "expense": exp, "net": inc - exp,
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

func okOr(c *httpx.Ctx, err error) error {
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}
