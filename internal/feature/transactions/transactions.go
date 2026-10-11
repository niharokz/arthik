// Package transactions is the Transactions feature: list/search with filters and
// totals, add (past, today or future dated), edit, delete, bulk actions, and
// remembered-transaction autofill.
//
// Creating with a client-supplied id is idempotent, which is what makes the PWA's
// offline queue safe to replay.
package transactions

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/query"
	"gitlab.com/niharokz/arthik/internal/view"
)

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/transactions", list)
	rt.Handle("GET /api/transactions/{id}", get)
	rt.Handle("POST /api/transactions", create)
	rt.Handle("PUT /api/transactions/{id}", update)
	rt.Handle("DELETE /api/transactions/{id}", remove)
	rt.Handle("POST /api/transactions/bulk", bulk)
	rt.Handle("GET /api/suggest", suggest)
}

// Sums are the totals of a filtered list.
type Sums struct {
	Income   money.Amount `json:"income"`
	Expense  money.Amount `json:"expense"`
	Net      money.Amount `json:"net"`
	Inflow   money.Amount `json:"inflow"`  // into the filtered accounts (account filter only)
	Outflow  money.Amount `json:"outflow"` // out of the filtered accounts
	Count    int          `json:"count"`
	Upcoming int          `json:"upcoming"` // future-dated items (not in the totals yet)
}

func list(c *httpx.Ctx) error {
	f := query.Parse(c.R.URL.Query())
	limit, offset := c.QInt("limit", 200), c.QInt("offset", 0)
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		m := query.Compile(f, d, comp)
		all := m.Select()
		var s Sums
		accs := map[string]bool{}
		for _, a := range f.Accounts {
			accs[a] = true
		}
		for _, t := range all {
			s.Count++
			if !comp.Effective(t) {
				s.Upcoming++ // future-dated: listed, but not in the totals until its date
				continue
			}
			for _, p := range t.Postings {
				if p.Category != "" {
					cat, _ := d.Category(p.Category)
					inc, exp := book.IncomeExpense(cat.Kind, p.Amount)
					s.Income += inc
					s.Expense += exp
				} else if accs[p.Account] {
					if p.Amount > 0 {
						s.Inflow += p.Amount
					} else {
						s.Outflow += -p.Amount
					}
				}
			}
		}
		s.Net = s.Income - s.Expense
		items := []view.Tx{}
		for i := offset; i < len(all) && i < offset+limit; i++ {
			items = append(items, view.TxOf(all[i], d, comp))
		}
		out = map[string]any{"items": items, "total": len(all), "sums": s}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

func get(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	var out view.Tx
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		t, ok := d.Tx(id)
		if !ok {
			return httpx.NotFound("transaction")
		}
		out = view.TxOf(t, d, comp)
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

func create(c *httpx.Ctx) error {
	var in book.Entry
	if err := c.Decode(&in); err != nil {
		return err
	}
	if in.ID != "" && !book.ValidID(in.ID) {
		return fmt.Errorf("invalid id %q", in.ID)
	}
	var id string
	dup := false
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		if in.ID != "" {
			if _, ok := d.Tx(in.ID); ok || d.Tombstoned(in.ID) { // offline replay of something already saved (or deleted since)
				id, dup = in.ID, true
				return nil
			}
		}
		e := in
		if e.ID == "" {
			e.ID = d.NewTxID(e.Date)
		}
		t, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		t.Labels = d.EnsureLabels(t.Labels, ch)
		d.SetTx(t, ch)
		id = t.ID
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]any{"id": id, "duplicate": dup})
}

func update(c *httpx.Ctx) error {
	var e book.Entry
	if err := c.Decode(&e); err != nil {
		return err
	}
	id := c.R.PathValue("id")
	e.ID = id
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		old, ok := d.Tx(id)
		if !ok {
			return httpx.NotFound("transaction")
		}
		t, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		t.Extra = old.Extra // keep fields added by hand
		t.Labels = d.EnsureLabels(t.Labels, ch)
		d.SetTx(t, ch)
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]string{"id": id})
}

func remove(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		if !d.DeleteTx(id, ch) {
			return httpx.NotFound("transaction")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

// bulk applies one action to many transactions: status, delete, add_label, remove_label.
func bulk(c *httpx.Ctx) error {
	var in struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
		Value  string   `json:"value"`
	}
	if err := c.Decode(&in); err != nil {
		return err
	}
	n := 0
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		label := ""
		if in.Action == "add_label" {
			ls := d.EnsureLabels([]string{in.Value}, ch)
			if len(ls) == 0 {
				return fmt.Errorf("enter a label")
			}
			label = ls[0]
		}
		for _, id := range in.IDs {
			t, ok := d.Tx(id)
			if !ok {
				continue
			}
			switch in.Action {
			case "delete":
				d.DeleteTx(id, ch)
			case "status":
				switch in.Value {
				case model.StatusUncleared, model.StatusCleared, model.StatusReconciled:
				default:
					return fmt.Errorf("invalid status %q", in.Value)
				}
				t.Status = in.Value
				d.SetTx(t, ch)
			case "add_label":
				if t.HasLabel(label) {
					continue
				}
				t.Labels = append(append([]string(nil), t.Labels...), label)
				d.SetTx(t, ch)
			case "remove_label":
				var keep []string
				for _, l := range t.Labels {
					if l != in.Value {
						keep = append(keep, l)
					}
				}
				t.Labels = keep
				d.SetTx(t, ch)
			default:
				return fmt.Errorf("unknown action %q", in.Action)
			}
			n++
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]int{"updated": n})
}

// Suggestion is a remembered transaction: the details of the latest one with this
// title. Picking it in the add form fills in everything (only the date stays today).
type Suggestion struct {
	Title     string       `json:"title"`
	Type      string       `json:"type"`
	Amount    money.Amount `json:"amount"`
	Account   string       `json:"account,omitempty"`
	ToAccount string       `json:"to_account,omitempty"`
	Category  string       `json:"category,omitempty"`
	Splits    []book.Split `json:"splits,omitempty"`
	Labels    []string     `json:"labels,omitempty"`
	Notes     string       `json:"notes,omitempty"`
	Uses      int          `json:"uses"`
	Last      string       `json:"last"` // date of the latest use
}

// suggest returns remembered transactions whose title contains every word of ?q=
// (all of them when q is empty). Ranking: title starts with q, same ?type=, most
// used, most recent. ?limit= caps the list (default 8).
func suggest(c *httpx.Ctx) error {
	q := strings.ToLower(strings.TrimSpace(c.Q("q")))
	words := strings.Fields(q)
	typ := c.Q("type")
	limit := c.QInt("limit", 8)
	if limit < 1 || limit > 50 {
		limit = 8
	}
	var out []Suggestion
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		byTitle := map[string]*Suggestion{}
		for _, t := range d.Txns { // oldest → newest, so the latest use wins
			if !comp.Valid[t.ID] || t.Type == model.TxJournal || strings.TrimSpace(t.Title) == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(t.Title))
			if !containsAll(key, words) {
				continue
			}
			e := book.EntryOf(t)
			uses := 1
			if old := byTitle[key]; old != nil {
				uses = old.Uses + 1
			}
			byTitle[key] = &Suggestion{Title: t.Title, Type: e.Type, Amount: e.Amount, Account: e.Account,
				ToAccount: e.ToAccount, Category: e.Category, Splits: e.Splits, Labels: e.Labels,
				Notes: e.Notes, Uses: uses, Last: t.Date}
		}
		for _, s := range byTitle {
			out = append(out, *s)
		}
		sort.Slice(out, func(i, j int) bool {
			a, b := out[i], out[j]
			pa, pb := q != "" && strings.HasPrefix(strings.ToLower(a.Title), q), q != "" && strings.HasPrefix(strings.ToLower(b.Title), q)
			if pa != pb {
				return pa
			}
			if ta, tb := a.Type == typ, b.Type == typ; ta != tb {
				return ta
			}
			if a.Uses != b.Uses {
				return a.Uses > b.Uses
			}
			if a.Last != b.Last {
				return a.Last > b.Last
			}
			return a.Title < b.Title
		})
		if len(out) > limit {
			out = out[:limit]
		}
		return nil
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []Suggestion{}
	}
	return c.OK(out)
}

// containsAll reports whether s contains every word.
func containsAll(s string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}
