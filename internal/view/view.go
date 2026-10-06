// Package view shapes records for the API: a transaction with its UI-friendly
// fields, an account with credit-card details, a budget line, etc.
package view

import (
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
)

// Tx is a transaction as the UI sees it.
type Tx struct {
	book.Entry
	Postings  []model.Posting `json:"postings"`
	Scheduled bool            `json:"scheduled,omitempty"` // date is in the future
	Invalid   string          `json:"invalid,omitempty"`   // validation error (excluded from totals)
	Upcoming  bool            `json:"upcoming,omitempty"`  // a not-yet-posted reminder occurrence
}

// TxOf builds a Tx.
func TxOf(t model.Transaction, d *book.Data, c *book.Computed) Tx {
	v := Tx{Entry: book.EntryOf(t), Postings: t.Postings}
	if v.Postings == nil {
		v.Postings = []model.Posting{}
	}
	if td, err := dates.Parse(t.Date); err == nil && td.After(c.Today) {
		v.Scheduled = true
	}
	if !c.Valid[t.ID] {
		if err := d.ValidateTx(t); err != nil {
			v.Invalid = err.Error()
		}
	}
	return v
}

// UpcomingOf wraps a reminder occurrence.
func UpcomingOf(e book.Entry, d *book.Data) Tx {
	v := Tx{Entry: e, Upcoming: true, Scheduled: true}
	if t, err := d.BuildTx(e); err == nil {
		v.Postings = t.Postings
	} else {
		v.Postings = []model.Posting{}
	}
	return v
}

// Card holds credit-card statement details.
type Card struct {
	Available     money.Amount `json:"available"`      // limit minus what is owed
	LastStatement string       `json:"last_statement"` // most recent statement date
	NextStatement string       `json:"next_statement"`
	DueDate       string       `json:"due_date"` // payment due for the last statement
	Billed        money.Amount `json:"billed"`   // net charges in the last closed cycle
	Unbilled      money.Amount `json:"unbilled"` // net charges since the last statement
	Utilised      int          `json:"utilised"` // % of limit used (0 when no limit)
}

// Account is an account plus derived details.
type Account struct {
	model.Account
	Class     string `json:"class"`
	TypeLabel string `json:"type_label"`
	Card      *Card  `json:"card,omitempty"`
}

// AccountOf builds an Account.
func AccountOf(a model.Account, d *book.Data, c *book.Computed) Account {
	info, _ := model.TypeInfo(a.Type)
	v := Account{Account: a, Class: a.Class(), TypeLabel: info.Label}
	if v.TypeLabel == "" {
		v.TypeLabel = a.Type
	}
	if v.Icon == "" {
		v.Icon = info.Icon
	}
	if a.Type == "credit_card" {
		v.Card = cardOf(a, d, c)
	}
	return v
}

func cardOf(a model.Account, d *book.Data, c *book.Computed) *Card {
	card := &Card{}
	owed := -c.Balance[a.ID] // positive when you owe
	if a.CreditLimit > 0 {
		card.Available = a.CreditLimit - owed
		if owed > 0 {
			card.Utilised = int(int64(owed) * 100 / int64(a.CreditLimit))
		}
	}
	if a.StatementDay < 1 || a.StatementDay > 31 {
		return card
	}
	today := c.Today
	last := dates.New(today.Year(), today.Month(), 1).AddMonths(0, a.StatementDay)
	if last.After(today) {
		last = last.AddMonths(-1, a.StatementDay)
	}
	prev := last.AddMonths(-1, a.StatementDay)
	card.LastStatement = last.String()
	card.NextStatement = last.AddMonths(1, a.StatementDay).String()
	if a.DueDay >= 1 && a.DueDay <= 31 {
		due := dates.New(last.Year(), last.Month(), 1).AddMonths(0, a.DueDay)
		if !due.After(last) {
			due = due.AddMonths(1, a.DueDay)
		}
		card.DueDate = due.String()
	}
	for _, t := range d.Txns {
		if !c.Effective(t) {
			continue
		}
		td, _ := dates.Parse(t.Date)
		for _, p := range t.Postings {
			if p.Account != a.ID || p.Amount >= 0 {
				continue // only charges (credits to the card); payments are excluded
			}
			switch {
			case td.After(last):
				card.Unbilled += -p.Amount
			case td.After(prev):
				card.Billed += -p.Amount
			}
		}
	}
	return card
}

// BudgetFor converts a category's budget to the length of range r
// (a monthly ₹12,000 budget is ₹36,000 for a quarter, ₹2,769.23 for a week …).
func BudgetFor(cat model.Category, r period.Range) money.Amount {
	if cat.Budget == 0 {
		return 0
	}
	bk := period.Kind(cat.BudgetPeriod)
	if !bk.Valid() || bk == period.Custom {
		bk = period.Monthly
	}
	if bk == r.Kind {
		return cat.Budget
	}
	num, den := period.MonthlyFactor(bk) // budget → per month
	switch r.Kind {
	case period.Weekly:
		return cat.Budget.MulDiv(num*12, den*52)
	case period.Monthly:
		return cat.Budget.MulDiv(num, den)
	case period.Quarterly:
		return cat.Budget.MulDiv(num*3, den)
	case period.HalfYearly:
		return cat.Budget.MulDiv(num*6, den)
	case period.Yearly:
		return cat.Budget.MulDiv(num*12, den)
	}
	// custom: per-day rate × days (365.25-day year)
	return cat.Budget.MulDiv(num*12*int64(r.Days())*100, den*36525)
}
