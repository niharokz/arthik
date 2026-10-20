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
	DefaultPeriod  string   `yaml:"default_period" json:"default_period"`     // weekly|monthly|quarterly|halfyearly|yearly
	WeekStart      string   `yaml:"week_start" json:"week_start"`             // monday|sunday|…
	MonthStartDay  int      `yaml:"month_start_day" json:"month_start_day"`   // 1..28
	YearStartMonth int      `yaml:"year_start_month" json:"year_start_month"` // 1..12 (4 = Indian FY)
	UpcomingDays   int      `yaml:"upcoming_days" json:"upcoming_days"`       // dashboard look-ahead
	Theme          string   `yaml:"theme" json:"theme"`                       // auto|light|dark
	HideAmounts    bool     `yaml:"hide_amounts" json:"hide_amounts"`
	NavOrder       []string `yaml:"nav_order,omitempty,flow" json:"nav_order"` // menu order; first 4 = phone bottom bar

	// Inbox: category used when nothing better is known (empty = leave it to you).
	InboxExpenseCategory string `yaml:"inbox_expense_category,omitempty" json:"inbox_expense_category"`
	InboxIncomeCategory  string `yaml:"inbox_income_category,omitempty" json:"inbox_income_category"`
	// Inbox: turn bank SMS into drafts once a day at this time ("23:45"); empty = as soon as they arrive.
	InboxSMSTime string `yaml:"inbox_sms_time,omitempty" json:"inbox_sms_time"`

	Extra map[string]any `yaml:",inline" json:"-"`
}

// NavItems are the screens that can be ordered in the menu (Settings always stays last).
var NavItems = []string{"home", "tx", "inbox", "accounts", "reports", "calendar", "categories", "budgets", "labels"}

// retiredNav renames menu items from older versions (Scheduled became Inbox in 0.2.1).
var retiredNav = map[string]string{"scheduled": "inbox"}

// NormNav cleans a menu order: renames retired items, drops repeats and appends
// missing items in default order. unknown is the first item it did not recognise
// (left out of the result).
func NormNav(in []string) (out []string, unknown string) {
	known := map[string]bool{}
	for _, k := range NavItems {
		known[k] = true
	}
	seen := map[string]bool{}
	for _, k := range in {
		if r, ok := retiredNav[k]; ok {
			k = r
		}
		if !known[k] {
			if unknown == "" {
				unknown = k
			}
			continue
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, k := range NavItems {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out, unknown
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
	SMSDigits       []string     `yaml:"sms_digits,omitempty,flow" json:"sms_digits,omitempty"` // last digits banks use in SMS, e.g. ['9001']

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
// A record with Deleted set is a tombstone: it remembers a deletion so a device that
// was offline cannot bring the transaction back when Syncthing conflict copies merge.
type Transaction struct {
	ID       string         `yaml:"id" json:"id"`
	Date     string         `yaml:"date,omitempty" json:"date"`
	Time     string         `yaml:"time,omitempty" json:"time,omitempty"`
	Type     string         `yaml:"type,omitempty" json:"type"`
	Title    string         `yaml:"title,omitempty" json:"title"`
	Status   string         `yaml:"status,omitempty" json:"status,omitempty"`
	Labels   []string       `yaml:"labels,omitempty,flow" json:"labels,omitempty"`
	Notes    string         `yaml:"notes,omitempty" json:"notes,omitempty"`
	Postings []Posting      `yaml:"postings,omitempty" json:"postings"`
	Updated  string         `yaml:"updated,omitempty" json:"updated,omitempty"` // UTC time of the last change (newest wins on merge)
	Deleted  string         `yaml:"deleted,omitempty" json:"-"`                 // UTC time of deletion (tombstone)
	Extra    map[string]any `yaml:",inline" json:"-"`
}

// IsTombstone reports whether the record only marks a deletion.
func (t Transaction) IsTombstone() bool { return t.Deleted != "" }

// Draft statuses.
const (
	DraftPending  = "pending"  // waiting for review
	DraftAccepted = "accepted" // became transaction Txn
	DraftMatched  = "matched"  // was already recorded as transaction Txn
	DraftRejected = "rejected" // not a transaction you want
	DraftMerged   = "merged"   // folded into draft Pair (two SMS of one transfer)
)

// How a draft's category was chosen (shown as a badge).
const (
	ViaLearned = "learned" // a rule you taught by accepting an earlier draft
	ViaPast    = "past"    // a transaction you entered with the same name
	ViaSource  = "source"  // the producer named it (e.g. Kronos subscription category)
	ViaAI      = "ai"      // suggested by the local AI
	ViaDefault = "default" // the fallback category from settings
)

// Draft is one entry of inbox/drafts.md: a transaction proposed by a producer
// (an SMS from the phone, a subscription from Kronos) that waits for review.
type Draft struct {
	ID        string       `yaml:"id" json:"id"`
	Source    string       `yaml:"source" json:"source"` // sms | kronos | …
	Status    string       `yaml:"status" json:"status"`
	Date      string       `yaml:"date" json:"date"`
	Time      string       `yaml:"time,omitempty" json:"time,omitempty"`
	Type      string       `yaml:"type" json:"type"` // expense | income | transfer
	Amount    money.Amount `yaml:"amount" json:"amount"`
	Account   string       `yaml:"account,omitempty" json:"account,omitempty"`
	ToAccount string       `yaml:"to_account,omitempty" json:"to_account,omitempty"`
	Category  string       `yaml:"category,omitempty" json:"category,omitempty"`
	Title     string       `yaml:"title" json:"title"`
	Labels    []string     `yaml:"labels,omitempty,flow" json:"labels,omitempty"`
	Notes     string       `yaml:"notes,omitempty" json:"notes,omitempty"`
	Via       string       `yaml:"via,omitempty" json:"via,omitempty"` // how the category was chosen

	Merchant     string       `yaml:"merchant,omitempty" json:"merchant,omitempty"`           // name as the source wrote it (learning key)
	Digits       string       `yaml:"digits,omitempty" json:"digits,omitempty"`               // card/account digits from the SMS
	AccountHint  string       `yaml:"account_hint,omitempty" json:"account_hint,omitempty"`   // account name given by the source
	CategoryHint string       `yaml:"category_hint,omitempty" json:"category_hint,omitempty"` // category name given by the source
	Balance      money.Amount `yaml:"balance,omitempty" json:"balance,omitempty"`             // balance the bank reported after it
	Available    money.Amount `yaml:"available,omitempty" json:"available,omitempty"`         // card limit left, as reported
	Rule         string       `yaml:"rule,omitempty" json:"rule,omitempty"`                   // SMS pattern that read it
	Raw          string       `yaml:"raw,omitempty" json:"raw,omitempty"`                     // the original message
	Pair         string       `yaml:"pair,omitempty" json:"pair,omitempty"`                   // other half of a paired transfer
	Txn          string       `yaml:"txn,omitempty" json:"txn,omitempty"`                     // transaction it became / matched
	Edited       bool         `yaml:"edited,omitempty" json:"edited,omitempty"`               // changed by you: never auto-filled again
	AITried      bool         `yaml:"ai_tried,omitempty" json:"-"`                            // the AI has looked at it
	Created      string       `yaml:"created,omitempty" json:"created,omitempty"`
	Updated      string       `yaml:"updated,omitempty" json:"updated,omitempty"` // newest wins on merge

	Extra map[string]any `yaml:",inline" json:"-"`
}

// Rule is one entry of inbox/learned.md: what a merchant (or Kronos name) becomes.
type Rule struct {
	ID       string   `yaml:"id" json:"id"`
	Match    string   `yaml:"match" json:"match"` // normalised merchant key
	Title    string   `yaml:"title,omitempty" json:"title,omitempty"`
	Category string   `yaml:"category,omitempty" json:"category,omitempty"`
	Account  string   `yaml:"account,omitempty" json:"account,omitempty"` // only when the source names no account digits
	Labels   []string `yaml:"labels,omitempty,flow" json:"labels,omitempty"`
	Uses     int      `yaml:"uses,omitempty" json:"uses,omitempty"`
	Updated  string   `yaml:"updated,omitempty" json:"updated,omitempty"`

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
