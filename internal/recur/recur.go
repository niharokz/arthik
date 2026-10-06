// Package recur parses repeat rules ("monthly", "2 weeks", …) and steps dates by
// them. Month/year steps keep an anchor day so month-end bills never drift
// (31 Jan → 28 Feb → 31 Mar), the same rule Kronos uses for events.
package recur

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/niharokz/arthik/internal/dates"
)

// Rule is a parsed repeat rule: every N units. N == 0 means "does not repeat".
type Rule struct {
	N    int
	Unit string // day | week | month
}

// Names are the fixed rules offered in the UI (plus "N days|weeks|months|years").
var Names = []string{"none", "daily", "weekly", "biweekly", "monthly", "bimonthly", "quarterly", "halfyearly", "yearly"}

// Parse reads a rule. Accepted: none, daily, weekly, biweekly, monthly, bimonthly,
// quarterly, halfyearly, yearly, or "N days|weeks|months|years".
func Parse(s string) (Rule, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "none", "once", "never":
		return Rule{}, nil
	case "daily":
		return Rule{1, "day"}, nil
	case "weekly":
		return Rule{1, "week"}, nil
	case "biweekly", "fortnightly":
		return Rule{2, "week"}, nil
	case "monthly":
		return Rule{1, "month"}, nil
	case "bimonthly":
		return Rule{2, "month"}, nil
	case "quarterly":
		return Rule{3, "month"}, nil
	case "halfyearly", "half-yearly", "semiannual":
		return Rule{6, "month"}, nil
	case "yearly", "annual", "annually":
		return Rule{12, "month"}, nil
	}
	f := strings.Fields(s)
	if len(f) == 2 {
		n, err := strconv.Atoi(f[0])
		if err == nil && n > 0 && n <= 999 {
			u := strings.TrimSuffix(f[1], "s")
			switch u {
			case "day", "week", "month":
				return Rule{n, u}, nil
			case "year":
				return Rule{n * 12, "month"}, nil
			}
		}
	}
	return Rule{}, fmt.Errorf("unknown repeat rule %q (use daily, weekly, monthly, quarterly, halfyearly, yearly or \"N days|weeks|months|years\")", s)
}

// Repeats reports whether the rule produces more than one date.
func (r Rule) Repeats() bool { return r.N > 0 }

// Next returns the date one step after d. anchor is the remembered day of month
// for month-based rules (0 = use d's day); the returned anchor is what to store
// (0 when it is not needed, i.e. day < 29).
func (r Rule) Next(d dates.Date, anchor int) (dates.Date, int) {
	switch r.Unit {
	case "day":
		return d.AddDays(r.N), 0
	case "week":
		return d.AddDays(7 * r.N), 0
	case "month":
		a := anchor
		if a <= 0 || d.Day() != min(a, dates.DaysIn(d.Year(), d.Month())) {
			a = d.Day() // anchor no longer matches (date was moved by hand) → reset
		}
		nd := d.AddMonths(r.N, a)
		if a >= 29 {
			return nd, a
		}
		return nd, 0
	}
	return d, anchor
}
