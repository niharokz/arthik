// Package query is the one transaction filter shared by the transaction list,
// search, reports and CSV export, so "what you see is what you export".
package query

import (
	"net/url"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// Filter selects transactions. Empty fields match everything.
type Filter struct {
	From, To   dates.Date // inclusive; zero = open
	Types      []string
	Accounts   []string
	Categories []string // a parent matches its children too
	Labels     []string // any of
	Statuses   []string
	Text       string // title, notes, labels, amount
	Min, Max   money.Amount
	HasMin     bool
	HasMax     bool
	When       string // "" all | "posted" (date <= today) | "scheduled" (future)
	Invalid    bool   // include transactions that fail validation (default: excluded)
}

func list(v url.Values, name string) []string {
	var out []string
	for _, s := range v[name] {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// Parse reads a filter from URL query parameters:
// from, to, type, account, category, label, status, q, min, max, when.
func Parse(v url.Values) Filter {
	f := Filter{
		Types: list(v, "type"), Accounts: list(v, "account"), Categories: list(v, "category"),
		Labels: list(v, "label"), Statuses: list(v, "status"),
		Text: strings.TrimSpace(v.Get("q")), When: v.Get("when"),
	}
	if d, err := dates.Parse(v.Get("from")); err == nil {
		f.From = d
	}
	if d, err := dates.Parse(v.Get("to")); err == nil {
		f.To = d
	}
	if a, err := money.Parse(v.Get("min")); err == nil {
		f.Min, f.HasMin = a, true
	}
	if a, err := money.Parse(v.Get("max")); err == nil {
		f.Max, f.HasMax = a, true
	}
	return f
}

// Matcher is a compiled filter bound to a dataset.
type Matcher struct {
	f    Filter
	d    *book.Data
	c    *book.Computed
	cats map[string]bool
	accs map[string]bool
	text string
}

// Compile prepares f for d.
func Compile(f Filter, d *book.Data, c *book.Computed) *Matcher {
	m := &Matcher{f: f, d: d, c: c, text: strings.ToLower(f.Text)}
	if len(f.Categories) > 0 {
		m.cats = map[string]bool{}
		for _, id := range f.Categories {
			m.cats[id] = true
			for _, k := range d.Children(id) {
				m.cats[k] = true
			}
		}
	}
	if len(f.Accounts) > 0 {
		m.accs = map[string]bool{}
		for _, id := range f.Accounts {
			m.accs[id] = true
		}
	}
	return m
}

func in(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Match reports whether t passes the filter.
func (m *Matcher) Match(t model.Transaction) bool { return m.match(t, true) }

// MatchUnstored is Match for a transaction that is not stored (a reminder's next
// occurrence): it skips the stored-validity check.
func (m *Matcher) MatchUnstored(t model.Transaction) bool { return m.match(t, false) }

func (m *Matcher) match(t model.Transaction, stored bool) bool {
	f := m.f
	if stored && !f.Invalid && !m.c.Valid[t.ID] {
		return false
	}
	td, err := dates.Parse(t.Date)
	if err == nil {
		if !f.From.IsZero() && td.Before(f.From) {
			return false
		}
		if !f.To.IsZero() && td.After(f.To) {
			return false
		}
		switch f.When {
		case "posted":
			if td.After(m.c.Today) {
				return false
			}
		case "scheduled":
			if !td.After(m.c.Today) {
				return false
			}
		}
	}
	if len(f.Types) > 0 && !in(f.Types, t.Type) {
		return false
	}
	if len(f.Statuses) > 0 {
		st := t.Status
		if st == "" {
			st = model.StatusUncleared
		}
		if !in(f.Statuses, st) {
			return false
		}
	}
	if len(f.Labels) > 0 {
		ok := false
		for _, l := range f.Labels {
			if t.HasLabel(l) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if m.accs != nil || m.cats != nil {
		accOK, catOK := m.accs == nil, m.cats == nil
		for _, p := range t.Postings {
			if m.accs != nil && p.Account != "" && m.accs[p.Account] {
				accOK = true
			}
			if m.cats != nil && p.Category != "" && m.cats[p.Category] {
				catOK = true
			}
		}
		if !accOK || !catOK {
			return false
		}
	}
	amt := book.EntryOf(t).Amount
	if f.HasMin && amt.Abs() < f.Min {
		return false
	}
	if f.HasMax && amt.Abs() > f.Max {
		return false
	}
	if m.text != "" {
		hay := strings.ToLower(t.Title + " " + t.Notes + " " + strings.Join(t.Labels, " ") + " " + amt.Abs().String())
		for _, p := range t.Postings {
			if a, ok := m.d.Account(p.Account); ok {
				hay += " " + strings.ToLower(a.Name)
			}
			if c, ok := m.d.Category(p.Category); ok {
				hay += " " + strings.ToLower(c.Name)
			}
		}
		for _, word := range strings.Fields(m.text) {
			if !strings.Contains(hay, word) {
				return false
			}
		}
	}
	return true
}

// Select returns matching transactions newest first.
func (m *Matcher) Select() []model.Transaction {
	var out []model.Transaction
	for i := len(m.d.Txns) - 1; i >= 0; i-- {
		if t := m.d.Txns[i]; m.Match(t) {
			out = append(out, t)
		}
	}
	return out
}
