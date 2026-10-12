package inbox

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// View is a draft as the review screen shows it, with checks against your books.
type View struct {
	model.Draft
	Same     *Same    `json:"same,omitempty"`     // a transaction that looks like this one
	Warnings []string `json:"warnings,omitempty"` // things worth a look before accepting
	PairRaw  string   `json:"pair_raw,omitempty"` // the other SMS of a paired transfer

	Confidence string   `json:"confidence,omitempty"` // high | medium | low (pending drafts only)
	Checks     []string `json:"checks,omitempty"`     // what keeps it from "high"
}

// Confidence levels: how safe it is to accept a draft without opening it.
const (
	High   = "high"
	Medium = "medium"
	Low    = "low"
)

// Same is a likely duplicate already in your books.
type Same struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Date   string       `json:"date"`
	Amount money.Amount `json:"amount"`
}

// Views returns drafts with the given statuses (all when empty), pending first,
// newest first within each group.
func Views(d *book.Data, c *book.Computed, statuses ...string) []View {
	want := map[string]bool{}
	for _, s := range statuses {
		want[s] = true
	}
	out := []View{}
	for _, x := range d.Drafts {
		if x.Status == model.DraftMerged || (len(want) > 0 && !want[x.Status]) {
			continue
		}
		v := View{Draft: x}
		if p, ok := d.Draft(x.Pair); ok {
			v.PairRaw = p.Raw
		}
		if x.Status == model.DraftPending {
			v.Same = same(d, c, x)
			v.Warnings = warnings(d, c, x)
			v.Confidence, v.Checks = confidence(x, v.Same != nil, len(v.Warnings) > 0)
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := out[i].Status == model.DraftPending, out[j].Status == model.DraftPending
		if pi != pj {
			return pi
		}
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Time > out[j].Time
	})
	return out
}

// Pending counts drafts waiting for review.
func Pending(d *book.Data) int {
	n := 0
	for _, x := range d.Drafts {
		if x.Status == model.DraftPending {
			n++
		}
	}
	return n
}

// confidence rates a pending draft: high when a rule or your history filled it in
// and nothing looks off, medium for Kronos/AI categories or warnings, low when the
// category is only the fallback, the account is unknown or it may be a duplicate.
func confidence(x model.Draft, dup, warned bool) (string, []string) {
	level := High
	var checks []string
	lower := func(to, why string) {
		if to == Low || level == High {
			level = to
		}
		checks = append(checks, why)
	}
	if x.Type != model.TxTransfer && !x.Edited {
		switch x.Via {
		case model.ViaLearned, model.ViaPast:
		case model.ViaSource:
			lower(Medium, "Category taken from "+sourceName(x.Source))
		case model.ViaAI:
			lower(Medium, "Category suggested by the AI")
		default:
			if x.Category == "" {
				lower(Low, "No category yet")
			} else {
				lower(Low, "Category is only the fallback")
			}
		}
	}
	if x.Account == "" || (x.Type == model.TxTransfer && x.ToAccount == "") {
		lower(Low, "Account not known")
	}
	if dup {
		lower(Low, "May already be recorded")
	}
	if warned {
		lower(Medium, "See the warnings")
	}
	return level, checks
}

func sourceName(s string) string {
	if s == SourceSMS {
		return "the SMS"
	}
	if s == "" {
		return "the source"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// same finds a transaction on the same account for the same amount within a day.
func same(d *book.Data, c *book.Computed, x model.Draft) *Same {
	acc := x.Account
	if acc == "" {
		acc = x.ToAccount
	}
	xd, err := dates.Parse(x.Date)
	if acc == "" || err != nil {
		return nil
	}
	var best *Same
	bestGap := 99
	for _, t := range d.Txns {
		if t.ID == TxID(x.ID) || !c.Valid[t.ID] {
			continue
		}
		td, err := dates.Parse(t.Date)
		if err != nil {
			continue
		}
		gap := abs(xd.DaysUntil(td))
		if gap > 1 || gap >= bestGap {
			continue
		}
		for _, p := range t.Postings {
			if p.Account == acc && p.Amount.Abs() == x.Amount {
				best, bestGap = &Same{ID: t.ID, Title: t.Title, Date: t.Date, Amount: x.Amount}, gap
				break
			}
		}
	}
	return best
}

func warnings(d *book.Data, c *book.Computed, x model.Draft) []string {
	var w []string
	if x.Rule == "generic" {
		w = append(w, "Read by the catch-all SMS pattern — check the amount and direction.")
	}
	if x.Digits != "" && x.Account == "" && x.ToAccount == "" {
		w = append(w, fmt.Sprintf("No account has SMS digits …%s yet — choose one and tick “Remember”.", x.Digits))
	}
	if x.AccountHint != "" && x.Account == "" {
		w = append(w, fmt.Sprintf("“%s” isn't an account name here — choose the account.", x.AccountHint))
	}
	if x.Balance != 0 && x.Account != "" {
		if got := balanceAfter(d, c, x); got != x.Balance {
			w = append(w, fmt.Sprintf("The bank says the balance is %s; Arthik would show %s — a transaction may be missing.", x.Balance.Format(), got.Format()))
		}
	}
	if x.Available != 0 && x.Account != "" {
		if a, ok := d.Account(x.Account); ok && a.CreditLimit > 0 {
			if got := a.CreditLimit + balanceAfter(d, c, x); got != x.Available {
				w = append(w, fmt.Sprintf("The bank says %s of the limit is left; Arthik would show %s.", x.Available.Format(), got.Format()))
			}
		}
	}
	return w
}

// balanceAfter is the account balance once this draft (and earlier pending drafts
// on the same account) are recorded.
func balanceAfter(d *book.Data, c *book.Computed, x model.Draft) money.Amount {
	a, _ := d.Account(x.Account)
	bal := a.OpeningBalance
	for _, t := range d.Txns {
		if !c.Valid[t.ID] || t.Date > x.Date || (t.Date == x.Date && t.Time != "" && x.Time != "" && t.Time > x.Time) {
			continue
		}
		for _, p := range t.Postings {
			if p.Account == x.Account {
				bal += p.Amount
			}
		}
	}
	for _, o := range d.Drafts {
		if o.Status == model.DraftPending && o.Account == x.Account && o.Type != model.TxTransfer &&
			(o.Date < x.Date || o.Date == x.Date && o.Time < x.Time) && same(d, c, o) == nil {
			bal += signed(o)
		}
	}
	if same(d, c, x) == nil {
		bal += signed(x)
	}
	return bal
}
