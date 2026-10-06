// Package period turns "monthly / weekly / half-yearly …" plus a reference day into
// a concrete date range. It drives category totals, budgets and reports.
//
// Settings that shape a period: the first day of the week, the day a "month" starts
// on (salary-cycle months, 1–28) and the month a year starts in (4 = Indian FY).
package period

import (
	"fmt"
	"time"

	"gitlab.com/niharokz/arthik/internal/dates"
)

// Kind is a period length.
type Kind string

const (
	Weekly     Kind = "weekly"
	Monthly    Kind = "monthly"
	Quarterly  Kind = "quarterly"
	HalfYearly Kind = "halfyearly"
	Yearly     Kind = "yearly"
	Custom     Kind = "custom"
)

// Kinds lists the selectable kinds in display order.
var Kinds = []Kind{Weekly, Monthly, Quarterly, HalfYearly, Yearly, Custom}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	for _, v := range Kinds {
		if k == v {
			return true
		}
	}
	return false
}

// Opts are the user settings that shape periods.
type Opts struct {
	WeekStart      time.Weekday // Monday by default
	MonthStartDay  int          // 1..28
	YearStartMonth time.Month   // 1..12
}

// DefaultOpts is Monday weeks, calendar months, calendar years.
func DefaultOpts() Opts {
	return Opts{WeekStart: time.Monday, MonthStartDay: 1, YearStartMonth: time.January}
}

func (o Opts) norm() Opts {
	if o.MonthStartDay < 1 || o.MonthStartDay > 28 {
		o.MonthStartDay = 1
	}
	if o.YearStartMonth < 1 || o.YearStartMonth > 12 {
		o.YearStartMonth = time.January
	}
	return o
}

// Range is an inclusive date range with a display label.
type Range struct {
	Kind  Kind       `json:"kind"`
	From  dates.Date `json:"from"`
	To    dates.Date `json:"to"`
	Label string     `json:"label"`
}

// Days is the number of days in the range (inclusive).
func (r Range) Days() int { return r.From.DaysUntil(r.To) + 1 }

// Contains reports whether d is inside the range.
func (r Range) Contains(d dates.Date) bool { return d.InRange(r.From, r.To) }

// months returns the number of months in a month-based kind (0 for others).
func (k Kind) months() int {
	switch k {
	case Monthly:
		return 1
	case Quarterly:
		return 3
	case HalfYearly:
		return 6
	case Yearly:
		return 12
	}
	return 0
}

// Of returns the period of kind k containing ref. Custom is not valid here (use Between).
func Of(k Kind, ref dates.Date, o Opts) Range {
	o = o.norm()
	if k == Weekly {
		back := (int(ref.Weekday()) - int(o.WeekStart) + 7) % 7
		from := ref.AddDays(-back)
		return labelled(Range{Kind: k, From: from, To: from.AddDays(6)}, o)
	}
	n := k.months()
	if n == 0 {
		n, k = 1, Monthly
	}
	// Start of the "month" holding ref (salary-cycle aware).
	ms := dates.New(ref.Year(), ref.Month(), o.MonthStartDay)
	if ref.Before(ms) {
		ms = ms.AddMonths(-1, o.MonthStartDay)
	}
	// Step back to the first month of the block (blocks are aligned to the year start).
	if n > 1 {
		k2 := (int(ms.Month()) - int(o.YearStartMonth) + 12) % 12
		ms = ms.AddMonths(-(k2 % n), o.MonthStartDay)
	}
	to := ms.AddMonths(n, o.MonthStartDay).AddDays(-1)
	return labelled(Range{Kind: k, From: ms, To: to}, o)
}

// Between builds a custom range (swapping the ends if needed).
func Between(from, to dates.Date) Range {
	if to.Before(from) {
		from, to = to, from
	}
	return labelled(Range{Kind: Custom, From: from, To: to}, DefaultOpts())
}

// Shift moves a range by n periods (n may be negative). Custom ranges move by their length.
func (r Range) Shift(n int, o Opts) Range {
	if n == 0 {
		return r
	}
	if r.Kind == Custom {
		d := r.Days() * n
		return Between(r.From.AddDays(d), r.To.AddDays(d))
	}
	if r.Kind == Weekly {
		return Of(Weekly, r.From.AddDays(7*n), o)
	}
	return Of(r.Kind, r.From.AddMonths(r.Kind.months()*n, o.norm().MonthStartDay), o)
}

// Buckets splits [from,to] into consecutive periods of kind k (clipped to the range),
// used by trend reports.
func Buckets(k Kind, from, to dates.Date, o Opts) []Range {
	var out []Range
	if k == Custom || !k.Valid() {
		k = Monthly
	}
	cur := Of(k, from, o)
	for i := 0; i < 400 && !cur.From.After(to); i++ {
		b := cur
		b.From, b.To = dates.Max(b.From, from), dates.Min(b.To, to)
		out = append(out, b)
		cur = cur.Shift(1, o)
	}
	return out
}

func labelled(r Range, o Opts) Range {
	f, t := r.From, r.To
	sameYear := f.Year() == t.Year()
	switch {
	case r.Kind == Monthly && f.Day() == 1:
		r.Label = f.Format("Jan 2006")
	case r.Kind == Yearly && f.Day() == 1 && f.Month() == time.January:
		r.Label = f.Format("2006")
	case r.Kind == Yearly && f.Day() == 1:
		r.Label = fmt.Sprintf("FY %d–%02d", f.Year(), t.Year()%100)
	case (r.Kind == Quarterly || r.Kind == HalfYearly) && f.Day() == 1:
		if sameYear {
			r.Label = f.Format("Jan") + "–" + t.Format("Jan 2006")
		} else {
			r.Label = f.Format("Jan 2006") + " – " + t.Format("Jan 2006")
		}
	case sameYear && f.Month() == t.Month():
		r.Label = f.Format("02") + "–" + t.Format("02 Jan 2006")
	case sameYear:
		r.Label = f.Format("02 Jan") + " – " + t.Format("02 Jan 2006")
	default:
		r.Label = f.Format("02 Jan 2006") + " – " + t.Format("02 Jan 2006")
	}
	return r
}

// MonthlyEquivalent converts an amount-per-period into "per month" numerator/denominator
// factors used by budgets: per-month = amount * num / den.
func MonthlyFactor(k Kind) (num, den int64) {
	switch k {
	case Weekly:
		return 52, 12
	case Quarterly:
		return 1, 3
	case HalfYearly:
		return 1, 6
	case Yearly:
		return 1, 12
	}
	return 1, 1
}

// Resolve builds the range a request asks for: kind + a reference date
// (default today), or from/to for a custom range, then shifted by offset periods.
func Resolve(kind, ref, from, to string, offset int, def Kind, today dates.Date, o Opts) Range {
	k := Kind(kind)
	if !k.Valid() {
		k = def
	}
	if k == Custom {
		f, err1 := dates.Parse(from)
		t, err2 := dates.Parse(to)
		if err1 != nil || err2 != nil {
			k = def
		} else {
			return Between(f, t).Shift(offset, o)
		}
	}
	if k == Custom || !k.Valid() {
		k = Monthly
	}
	d := today
	if r, err := dates.Parse(ref); err == nil {
		d = r
	}
	return Of(k, d, o).Shift(offset, o)
}
