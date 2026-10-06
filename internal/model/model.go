// Package model defines Arthik's records exactly as they are stored in the data
// files. Every struct keeps unknown keys in Extra, so fields added by hand in
// Obsidian (or by a future version) survive a rewrite untouched.
//
// Double-entry in one paragraph: every Transaction is a list of Postings that sum
// to zero. A posting points at an account (bank, card, loan…) or a category
// (income/expense). Positive = debit, negative = credit. Spending ₹500 from HDFC on
// groceries is {hdfc: -500, groceries: +500}; a salary credit is {hdfc: +N, salary: -N};
// a transfer is {from: -N, to: +N}. Opening balances post against an implicit
// "opening balances" equity account, so the whole book always balances.
package model

import (
	"gitlab.com/niharokz/arthik/internal/money"
)

// Account classes.
const (
	ClassAsset     = "asset"
	ClassLiability = "liability"
)

// AccountType describes one selectable account type.
type AccountType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Class string `json:"class"`
	Icon  string `json:"icon"`
}

// AccountTypes is the fixed list of account types, in display order (Bluecoins groups).
var AccountTypes = []AccountType{
	{"bank", "Bank", ClassAsset, "🏦"},
	{"cash", "Cash", ClassAsset, "💵"},
	{"wallet", "Wallet", ClassAsset, "👛"},
	{"investment", "Investment", ClassAsset, "📈"},
	{"deposit", "Deposit", ClassAsset, "🔒"},
	{"property", "Property", ClassAsset, "🏠"},
	{"receivable", "Receivable", ClassAsset, "🤝"},
	{"other_asset", "Other asset", ClassAsset, "💼"},
	{"credit_card", "Credit card", ClassLiability, "💳"},
	{"loan", "Loan", ClassLiability, "🏛️"},
	{"payable", "Payable", ClassLiability, "🧾"},
	{"other_liability", "Other liability", ClassLiability, "📉"},
}

// TypeInfo returns the type entry for id (ok=false if unknown).
func TypeInfo(id string) (AccountType, bool) {
	for _, t := range AccountTypes {
		if t.ID == id {
			return t, true
		}
	}
	return AccountType{}, false
}

// Settings is settings.md.
type Settings struct {
	DefaultPeriod  string         `yaml:"default_period" json:"default_period"`     // weekly|monthly|quarterly|halfyearly|yearly
	WeekStart      string         `yaml:"week_start" json:"week_start"`             // monday|sunday|…
	MonthStartDay  int            `yaml:"month_start_day" json:"month_start_day"`   // 1..28
	YearStartMonth int            `yaml:"year_start_month" json:"year_start_month"` // 1..12 (4 = Indian FY)
	UpcomingDays   int            `yaml:"upcoming_days" json:"upcoming_days"`       // dashboard look-ahead
	Theme          string         `yaml:"theme" json:"theme"`                       // auto|light|dark
	HideAmounts    bool           `yaml:"hide_amounts" json:"hide_amounts"`
	Extra          map[string]any `yaml:",inline" json:"-"`
}

// DefaultSettings are used for any missing value.
func DefaultSettings() Settings {
	return Settings{DefaultPeriod: "monthly", WeekStart: "monday", MonthStartDay: 1, YearStartMonth: 1, UpcomingDays: 30, Theme: "auto"}
}

// Account is one entry of accounts.md. Fields after the "computed" marker are
// rewritten by Arthik from the transactions and must not be edited by hand.
type Account struct {
	ID              string       `yaml:"id" json:"id"`
	Name            string       `yaml:"name" json:"name"`
	Type            string       `yaml:"type" json:"type"`
	OpeningBalance  money.Amount `yaml:"opening_balance" json:"opening_balance"` // signed: liabilities owed are negative
	OpeningDate     string       `yaml:"opening_date,omitempty" json:"opening_date,omitempty"`
	CreditLimit     money.Amount `yaml:"credit_limit,omitempty" json:"credit_limit,omitempty"`
	StatementDay    int          `yaml:"statement_day,omitempty" json:"statement_day,omitempty"`
	DueDay          int          `yaml:"due_day,omitempty" json:"due_day,omitempty"`
	Color           string       `yaml:"color,omitempty" json:"color,omitempty"`
	Icon            string       `yaml:"icon,omitempty" json:"icon,omitempty"`
	Notes           string       `yaml:"notes,omitempty" json:"notes,omitempty"`
	Order           int          `yaml:"order,omitempty" json:"order,omitempty"`
	Hidden          bool         `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Archived        bool         `yaml:"archived,omitempty" json:"archived,omitempty"`
	ExcludeNetWorth bool         `yaml:"exclude_net_worth,omitempty" json:"exclude_net_worth,omitempty"`

	// computed (rewritten on every change)
	Balance   money.Amount `yaml:"balance" json:"balance"`                     // as of today
	Cleared   money.Amount `yaml:"cleared_balance" json:"cleared_balance"`     // cleared + reconciled only
	Projected money.Amount `yaml:"projected_balance" json:"projected_balance"` // incl. future-dated

	Extra map[string]any `yaml:",inline" json:"-"`
}

// Class returns asset or liability (unknown types count as assets).
func (a Account) Class() string {
	if t, ok := TypeInfo(a.Type); ok {
		return t.Class
	}
	return ClassAsset
}

// Category kinds. "both" is for categories money flows into and out of, e.g.
// "Market" for investment gains and losses.
const (
	KindExpense = "expense"
	KindIncome  = "income"
	KindBoth    = "both"
)

// Category is one entry of categories.md. A category is either a parent (Parent == "")
// or a child of a parent — two levels, as in Bluecoins.
type Category struct {
	ID           string       `yaml:"id" json:"id"`
	Name         string       `yaml:"name" json:"name"`
	Kind         string       `yaml:"kind" json:"kind"`
	Parent       string       `yaml:"parent,omitempty" json:"parent,omitempty"`
	Color        string       `yaml:"color,omitempty" json:"color,omitempty"`
	Icon         string       `yaml:"icon,omitempty" json:"icon,omitempty"`
	Budget       money.Amount `yaml:"budget,omitempty" json:"budget,omitempty"`
	BudgetPeriod string       `yaml:"budget_period,omitempty" json:"budget_period,omitempty"`
	Order        int          `yaml:"order,omitempty" json:"order,omitempty"`
	Hidden       bool         `yaml:"hidden,omitempty" json:"hidden,omitempty"`

	// computed for the default period in settings.md (rewritten on every change)
	Period string       `yaml:"period" json:"-"`
	Total  money.Amount `yaml:"total" json:"-"` // spent (expense) / received (income) / net gain (both)

	Extra map[string]any `yaml:",inline" json:"-"`
}

// Label is one entry of labels.md (Bluecoins "labels" = free tags on transactions).
type Label struct {
	ID    string `yaml:"id" json:"id"`
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color,omitempty" json:"color,omitempty"`

	// computed
	Count int `yaml:"count" json:"count"`

	Extra map[string]any `yaml:",inline" json:"-"`
}

// Transaction types (informational; the postings are what count).
const (
	TxExpense  = "expense"
	TxIncome   = "income"
	TxTransfer = "transfer"
	TxJournal  = "journal" // free-form postings, e.g. typed by hand in Obsidian
)

// Statuses (reconciliation).
const (
	StatusUncleared  = "uncleared"
	StatusCleared    = "cleared"
	StatusReconciled = "reconciled"
)

// Posting is one leg of a transaction: exactly one of Account / Category is set.
type Posting struct {
	Account  string         `yaml:"account,omitempty" json:"account,omitempty"`
	Category string         `yaml:"category,omitempty" json:"category,omitempty"`
	Amount   money.Amount   `yaml:"amount" json:"amount"` // + debit, - credit
	Extra    map[string]any `yaml:",inline" json:"-"`
}

// Transaction is one entry of transactions/transaction_YYYYMM.md.
type Transaction struct {
	ID       string         `yaml:"id" json:"id"`
	Date     string         `yaml:"date" json:"date"`
	Time     string         `yaml:"time,omitempty" json:"time,omitempty"`
	Type     string         `yaml:"type" json:"type"`
	Title    string         `yaml:"title" json:"title"`
	Status   string         `yaml:"status,omitempty" json:"status,omitempty"`
	Labels   []string       `yaml:"labels,omitempty,flow" json:"labels,omitempty"`
	Notes    string         `yaml:"notes,omitempty" json:"notes,omitempty"`
	Reminder string         `yaml:"reminder,omitempty" json:"reminder,omitempty"` // set when auto-posted by a reminder
	Postings []Posting      `yaml:"postings" json:"postings"`
	Extra    map[string]any `yaml:",inline" json:"-"`
}

// Reminder is one entry of reminders.md: a scheduled (usually recurring) transaction
// that Arthik auto-posts on each due date.
type Reminder struct {
	ID        string       `yaml:"id" json:"id"`
	Title     string       `yaml:"title" json:"title"`
	Type      string       `yaml:"type" json:"type"` // expense|income|transfer
	Amount    money.Amount `yaml:"amount" json:"amount"`
	Account   string       `yaml:"account" json:"account"`
	ToAccount string       `yaml:"to_account,omitempty" json:"to_account,omitempty"`
	Category  string       `yaml:"category,omitempty" json:"category,omitempty"`
	Labels    []string     `yaml:"labels,omitempty,flow" json:"labels,omitempty"`
	Notes     string       `yaml:"notes,omitempty" json:"notes,omitempty"`
	Status    string       `yaml:"status,omitempty" json:"status,omitempty"` // status given to posted transactions
	Repeat    string       `yaml:"repeat" json:"repeat"`
	Next      string       `yaml:"next" json:"next"` // next due date (advanced after each post)
	AnchorDay int          `yaml:"anchor_day,omitempty" json:"anchor_day,omitempty"`
	EndDate   string       `yaml:"end_date,omitempty" json:"end_date,omitempty"`
	Remaining int          `yaml:"remaining,omitempty" json:"remaining,omitempty"` // 0 = unlimited
	Paused    bool         `yaml:"paused,omitempty" json:"paused,omitempty"`
	Done      bool         `yaml:"done,omitempty" json:"done,omitempty"` // finished (end reached)
	Posted    int          `yaml:"posted,omitempty" json:"posted,omitempty"`

	Extra map[string]any `yaml:",inline" json:"-"`
}

// Sum returns the sum of all posting amounts (0 for a balanced transaction).
func (t Transaction) Sum() money.Amount {
	var s money.Amount
	for _, p := range t.Postings {
		s += p.Amount
	}
	return s
}

// HasLabel reports whether the transaction carries label id.
func (t Transaction) HasLabel(id string) bool {
	for _, l := range t.Labels {
		if l == id {
			return true
		}
	}
	return false
}
