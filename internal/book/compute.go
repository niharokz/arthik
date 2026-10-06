package book

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/period"
	"gitlab.com/niharokz/arthik/internal/store"
)

// Computed holds everything derived from the transactions. It is rebuilt after every
// change, reload and day change; nothing in here is ever the source of truth.
type Computed struct {
	Today dates.Date
	Valid map[string]bool // transaction id → passes validation (others are excluded)

	Balance   map[string]money.Amount // account id → balance as of today
	Cleared   map[string]money.Amount // account id → cleared/reconciled balance
	Projected map[string]money.Amount // account id → balance incl. future-dated

	OpeningEquity money.Amount // sum of opening balances (the equity side)
	NetWorth      money.Amount // assets + liabilities, excluding exclude_net_worth accounts
	Assets        money.Amount
	Liabilities   money.Amount // negative when you owe

	Period     period.Range            // the default period (settings.default_period, today)
	CatTotals  map[string]money.Amount // category id → natural total in Period, parents include children
	Income     money.Amount            // in Period
	Expense    money.Amount            // in Period
	LabelCount map[string]int
	Problems   []Problem
}

// Opts returns the period options from settings.
func (d *Data) Opts() period.Opts {
	o := period.DefaultOpts()
	o.MonthStartDay = d.Settings.MonthStartDay
	o.YearStartMonth = time.Month(d.Settings.YearStartMonth)
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		if strings.EqualFold(wd.String(), d.Settings.WeekStart) {
			o.WeekStart = wd
		}
	}
	return o
}

// Natural converts a posting amount on a category into the sign people expect:
// money spent on an expense category is positive, money received from an income
// category is positive, and a "both" category shows net gain as positive.
func Natural(kind string, posting money.Amount) money.Amount {
	if kind == model.KindExpense {
		return posting
	}
	return -posting
}

// Split classifies a category posting into (income, expense), both non-negative
// except for refunds (an expense refund is negative expense).
func IncomeExpense(kind string, posting money.Amount) (inc, exp money.Amount) {
	switch kind {
	case model.KindExpense:
		return 0, posting
	case model.KindIncome:
		return -posting, 0
	}
	if v := -posting; v >= 0 { // both: gains count as income, losses as expense
		return v, 0
	} else {
		return 0, -v
	}
}

// Effective reports whether a transaction counts towards balances today:
// it is valid and its date is not in the future.
func (c *Computed) Effective(t model.Transaction) bool {
	if !c.Valid[t.ID] {
		return false
	}
	d, err := dates.Parse(t.Date)
	return err == nil && !d.After(c.Today)
}

// RollUp adds every child's total into its parent (in place) and returns m.
func (d *Data) RollUp(m map[string]money.Amount) map[string]money.Amount {
	for _, c := range d.Categories {
		if c.Parent != "" {
			if _, ok := d.idx.cat[c.Parent]; ok {
				m[c.Parent] += m[c.ID]
			}
		}
	}
	return m
}

// CategoryTotals sums natural category amounts for valid transactions in [from,to]
// (own postings only; call RollUp for parent totals). Only transactions matching keep
// (nil = all) are included. Future-dated transactions are left out until their date:
// they have not happened yet.
func (d *Data) CategoryTotals(c *Computed, from, to dates.Date, keep func(model.Transaction) bool) map[string]money.Amount {
	out := map[string]money.Amount{}
	for _, t := range d.Txns {
		if !c.Valid[t.ID] {
			continue
		}
		td, err := dates.Parse(t.Date)
		if err != nil || !td.InRange(from, to) || td.After(c.Today) || (keep != nil && !keep(t)) {
			continue
		}
		for _, p := range t.Postings {
			if p.Category == "" {
				continue
			}
			cat, _ := d.Category(p.Category)
			out[p.Category] += Natural(cat.Kind, p.Amount)
		}
	}
	return out
}

// recomputeLocked rebuilds Computed and (when write is set and the data loaded
// cleanly) writes the computed fields to accounts.md, categories.md, labels.md and
// the overview to money.md.
func (b *Book) recomputeLocked(write bool) {
	d := b.data
	if d == nil {
		return
	}
	c := &Computed{
		Today: b.Today(), Valid: map[string]bool{},
		Balance: map[string]money.Amount{}, Cleared: map[string]money.Amount{}, Projected: map[string]money.Amount{},
		LabelCount: map[string]int{},
	}
	for _, a := range d.Accounts {
		c.Balance[a.ID] = a.OpeningBalance
		c.Cleared[a.ID] = a.OpeningBalance
		c.Projected[a.ID] = a.OpeningBalance
		c.OpeningEquity -= a.OpeningBalance
	}
	file := func(t model.Transaction) string { return "transaction_" + d.MonthOf(t) + ".md" }
	for _, t := range d.Txns {
		if err := d.ValidateTx(t); err != nil {
			c.Problems = append(c.Problems, Problem{File: file(t), ID: t.ID, Msg: err.Error() + " — excluded from totals"})
			continue
		}
		c.Valid[t.ID] = true
		td, _ := dates.Parse(t.Date)
		future := td.After(c.Today)
		cleared := t.Status == model.StatusCleared || t.Status == model.StatusReconciled
		for _, p := range t.Postings {
			if p.Account == "" {
				continue
			}
			c.Projected[p.Account] += p.Amount
			if !future {
				c.Balance[p.Account] += p.Amount
				if cleared {
					c.Cleared[p.Account] += p.Amount
				}
			}
		}
		if !future {
			for _, l := range t.Labels {
				c.LabelCount[l]++
			}
		}
	}
	for _, a := range d.Accounts {
		if _, ok := model.TypeInfo(a.Type); !ok {
			c.Problems = append(c.Problems, Problem{File: AccountsFile, ID: a.ID, Msg: fmt.Sprintf("unknown account type %q (treated as an asset)", a.Type)})
		}
		if a.ExcludeNetWorth {
			continue
		}
		bal := c.Balance[a.ID]
		if a.Class() == model.ClassLiability {
			c.Liabilities += bal
		} else {
			c.Assets += bal
		}
	}
	c.NetWorth = c.Assets + c.Liabilities
	for _, cat := range d.Categories {
		if cat.Parent != "" {
			if p, ok := d.Category(cat.Parent); !ok {
				c.Problems = append(c.Problems, Problem{File: CategoriesFile, ID: cat.ID, Msg: fmt.Sprintf("parent %q does not exist", cat.Parent)})
			} else if p.Parent != "" {
				c.Problems = append(c.Problems, Problem{File: CategoriesFile, ID: cat.ID, Msg: "only two category levels are supported"})
			}
		}
	}

	// Default-period category totals and income/expense.
	c.Period = period.Of(period.Kind(d.Settings.DefaultPeriod), c.Today, d.Opts())
	own := d.CategoryTotals(c, c.Period.From, c.Period.To, nil)
	for id, v := range own {
		cat, _ := d.Category(id)
		inc, exp := IncomeExpense(cat.Kind, toPosting(cat.Kind, v))
		c.Income += inc
		c.Expense += exp
	}
	c.CatTotals = d.RollUp(own)
	b.comp = c

	// Mirror computed values onto the records (served by the API and written to disk).
	for i := range d.Accounts {
		a := &d.Accounts[i]
		a.Balance, a.Cleared, a.Projected = c.Balance[a.ID], c.Cleared[a.ID], c.Projected[a.ID]
	}
	for i := range d.Categories {
		d.Categories[i].Period = c.Period.Label
		d.Categories[i].Total = c.CatTotals[d.Categories[i].ID]
	}
	for i := range d.Labels {
		d.Labels[i].Count = c.LabelCount[d.Labels[i].ID]
	}
	if !write || b.loadErr != nil || b.ReadOnly {
		return
	}
	ch := Change{Accounts: true, Categories: true, Labels: true}
	if err := b.persistLocked(d, &ch); err != nil {
		log.Printf("[%s] WARN could not write computed files: %v", b.Name, err)
		return
	}
	if _, err := store.Write(b.path(MoneyFile), hdrMoney, b.moneyDoc(d, c), 0o644); err != nil {
		log.Printf("[%s] WARN could not write %s: %v", b.Name, MoneyFile, err)
	}
	b.sigs = b.scanLocked()
}

// toPosting reverses Natural (natural value → raw posting amount).
func toPosting(kind string, natural money.Amount) money.Amount {
	if kind == model.KindExpense {
		return natural
	}
	return -natural
}

const hdrMoney = "arthik overview — fully computed from the transactions, rewritten on every change. Do not edit."

type moneyType struct {
	Type    string       `yaml:"type"`
	Total   money.Amount `yaml:"total"`
	Account int          `yaml:"accounts"`
}

type moneyDocT struct {
	Overview struct {
		AsOf        string       `yaml:"as_of"`
		NetWorth    money.Amount `yaml:"net_worth"`
		Assets      money.Amount `yaml:"assets"`
		Liabilities money.Amount `yaml:"liabilities"`
	} `yaml:"overview"`
	Period struct {
		Kind    string       `yaml:"kind"`
		Label   string       `yaml:"label"`
		From    string       `yaml:"from"`
		To      string       `yaml:"to"`
		Income  money.Amount `yaml:"income"`
		Expense money.Amount `yaml:"expense"`
		Net     money.Amount `yaml:"net"`
	} `yaml:"period"`
	ByType []moneyType `yaml:"by_type"`
	Ledger struct {
		Transactions  int          `yaml:"transactions"`
		Scheduled     int          `yaml:"scheduled"`
		OpeningEquity money.Amount `yaml:"opening_equity"`
		Balanced      bool         `yaml:"balanced"`
		Problems      []Problem    `yaml:"problems,omitempty"`
	} `yaml:"ledger"`
}

func (b *Book) moneyDoc(d *Data, c *Computed) moneyDocT {
	var m moneyDocT
	m.Overview.AsOf = c.Today.String()
	m.Overview.NetWorth, m.Overview.Assets, m.Overview.Liabilities = c.NetWorth, c.Assets, c.Liabilities
	m.Period.Kind, m.Period.Label = string(c.Period.Kind), c.Period.Label
	m.Period.From, m.Period.To = c.Period.From.String(), c.Period.To.String()
	m.Period.Income, m.Period.Expense, m.Period.Net = c.Income, c.Expense, c.Income-c.Expense
	for _, ty := range model.AccountTypes {
		var mt moneyType
		for _, a := range d.Accounts {
			if a.Type == ty.ID && !a.Archived {
				mt.Total += c.Balance[a.ID]
				mt.Account++
			}
		}
		if mt.Account > 0 {
			mt.Type = ty.ID
			m.ByType = append(m.ByType, mt)
		}
	}
	// The books balance when every account + every category + opening equity sums to zero.
	var sum money.Amount = c.OpeningEquity
	for _, a := range d.Accounts {
		sum += c.Projected[a.ID]
	}
	for _, t := range d.Txns {
		if !c.Valid[t.ID] {
			continue
		}
		if td, _ := dates.Parse(t.Date); td.After(c.Today) {
			m.Ledger.Scheduled++
		} else {
			m.Ledger.Transactions++
		}
		for _, p := range t.Postings {
			if p.Category != "" {
				sum += p.Amount
			}
		}
	}
	m.Ledger.OpeningEquity = c.OpeningEquity
	m.Ledger.Balanced = sum == 0
	m.Ledger.Problems = append(append([]Problem(nil), b.problems...), c.Problems...)
	return m
}
