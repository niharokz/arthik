// Package accounts is the Accounts feature: create / edit / archive / delete
// accounts, balance history for the account chart, and reconciliation.
package accounts

import (
	"fmt"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
)

// input is the editable part of an account.
type input struct {
	Name            string       `json:"name"`
	Type            string       `json:"type"`
	OpeningBalance  money.Amount `json:"opening_balance"`
	OpeningDate     string       `json:"opening_date"`
	CreditLimit     money.Amount `json:"credit_limit"`
	StatementDay    int          `json:"statement_day"`
	DueDay          int          `json:"due_day"`
	Color           string       `json:"color"`
	Icon            string       `json:"icon"`
	Notes           string       `json:"notes"`
	Order           int          `json:"order"`
	Hidden          bool         `json:"hidden"`
	Archived        bool         `json:"archived"`
	ExcludeNetWorth bool         `json:"exclude_net_worth"`
}

func (in input) apply(a *model.Account) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("enter an account name")
	}
	if _, ok := model.TypeInfo(in.Type); !ok {
		return fmt.Errorf("choose an account type")
	}
	if in.OpeningDate != "" {
		if _, err := dates.Parse(in.OpeningDate); err != nil {
			return err
		}
	}
	if in.StatementDay < 0 || in.StatementDay > 31 || in.DueDay < 0 || in.DueDay > 31 {
		return fmt.Errorf("statement and due day must be 1–31")
	}
	if in.CreditLimit < 0 {
		return fmt.Errorf("credit limit cannot be negative")
	}
	a.Name, a.Type, a.OpeningBalance, a.OpeningDate = in.Name, in.Type, in.OpeningBalance, in.OpeningDate
	a.CreditLimit, a.StatementDay, a.DueDay = in.CreditLimit, in.StatementDay, in.DueDay
	a.Color, a.Icon, a.Notes, a.Order = strings.TrimSpace(in.Color), strings.TrimSpace(in.Icon), strings.TrimSpace(in.Notes), in.Order
	a.Hidden, a.Archived, a.ExcludeNetWorth = in.Hidden, in.Archived, in.ExcludeNetWorth
	if a.Type != "credit_card" {
		a.CreditLimit, a.StatementDay, a.DueDay = 0, 0, 0
	}
	return nil
}

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("POST /api/accounts", create)
	rt.Handle("PUT /api/accounts/{id}", update)
	rt.Handle("DELETE /api/accounts/{id}", remove)
	rt.Handle("GET /api/accounts/{id}/history", history)
	rt.Handle("POST /api/accounts/{id}/reconcile", reconcile)
}

func create(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	var id string
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		var a model.Account
		if err := in.apply(&a); err != nil {
			return err
		}
		for _, x := range d.Accounts {
			if strings.EqualFold(x.Name, a.Name) {
				return fmt.Errorf("an account named %q already exists", a.Name)
			}
		}
		a.ID = d.UniqueAccountID(a.Name)
		id = a.ID
		d.Accounts = append(d.Accounts, a)
		ch.Accounts = true
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
		for i, a := range d.Accounts {
			if a.ID != id {
				continue
			}
			for _, x := range d.Accounts {
				if x.ID != id && strings.EqualFold(x.Name, strings.TrimSpace(in.Name)) {
					return fmt.Errorf("an account named %q already exists", in.Name)
				}
			}
			if err := in.apply(&a); err != nil {
				return err
			}
			d.Accounts[i] = a
			ch.Accounts = true
			return nil
		}
		return httpx.NotFound("account")
	}))
}

func remove(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	return okOr(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		used := 0
		for _, t := range d.Txns {
			for _, p := range t.Postings {
				if p.Account == id {
					used++
					break
				}
			}
		}
		for _, r := range d.Reminders {
			if r.Account == id || r.ToAccount == id {
				return fmt.Errorf("reminder %q uses this account — change or delete it first", r.Title)
			}
		}
		if used > 0 {
			return fmt.Errorf("this account has %d transaction(s); archive it instead (or delete those transactions first)", used)
		}
		for i, a := range d.Accounts {
			if a.ID == id {
				d.Accounts = append(d.Accounts[:i:i], d.Accounts[i+1:]...)
				ch.Accounts = true
				return nil
			}
		}
		return httpx.NotFound("account")
	}))
}

type point struct {
	Label   string       `json:"label"`
	From    dates.Date   `json:"from"`
	To      dates.Date   `json:"to"`
	Balance money.Amount `json:"balance"` // at the end of the bucket
}

// history returns the account balance at the end of each bucket (default: monthly,
// last 12 months).
func history(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	var out []point
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		a, ok := d.Account(id)
		if !ok {
			return httpx.NotFound("account")
		}
		to, from := comp.Today, comp.Today.AddMonths(-11, 0)
		if v, err := dates.Parse(c.Q("to")); err == nil {
			to = v
		}
		if v, err := dates.Parse(c.Q("from")); err == nil {
			from = v
		}
		kind := period.Kind(c.Q("bucket"))
		if !kind.Valid() || kind == period.Custom {
			kind = period.Monthly
		}
		buckets := period.Buckets(kind, from, to, d.Opts())
		bal := a.OpeningBalance
		ti := 0
		for _, b := range buckets {
			for ; ti < len(d.Txns); ti++ {
				t := d.Txns[ti]
				td, err := dates.Parse(t.Date)
				if err != nil {
					continue
				}
				if td.After(b.To) {
					break
				}
				if !comp.Valid[t.ID] {
					continue
				}
				for _, p := range t.Postings {
					if p.Account == id {
						bal += p.Amount
					}
				}
			}
			out = append(out, point{Label: b.Label, From: b.From, To: b.To, Balance: bal})
		}
		return nil
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []point{}
	}
	return c.OK(out)
}

// reconcile sets the status of the given transactions (which must touch this account).
func reconcile(c *httpx.Ctx) error {
	var in struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := c.Decode(&in); err != nil {
		return err
	}
	switch in.Status {
	case model.StatusUncleared, model.StatusCleared, model.StatusReconciled:
	default:
		return fmt.Errorf("invalid status %q", in.Status)
	}
	id := c.R.PathValue("id")
	n := 0
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		if _, ok := d.Account(id); !ok {
			return httpx.NotFound("account")
		}
		for _, tid := range in.IDs {
			t, ok := d.Tx(tid)
			if !ok {
				continue
			}
			touches := false
			for _, p := range t.Postings {
				if p.Account == id {
					touches = true
				}
			}
			if touches && t.Status != in.Status {
				t.Status = in.Status
				d.SetTx(t, ch)
				n++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]int{"updated": n})
}

func okOr(c *httpx.Ctx, err error) error {
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}
