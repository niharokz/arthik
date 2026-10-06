// Package dates holds Arthik's calendar helpers. A Date is a plain calendar day
// (no time of day, no zone); "today" is taken in the container's TZ.
package dates

import (
	"fmt"
	"strings"
	"time"
)

// Layout is the on-disk and API date format.
const Layout = "2006-01-02"

// Date is a calendar day stored as UTC midnight so arithmetic never crosses a DST edge.
type Date struct{ t time.Time }

// New builds a date; out-of-range days roll over like time.Date does.
func New(y int, m time.Month, d int) Date { return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)} }

// Parse reads YYYY-MM-DD (also accepts YYYYMMDD and a leading timestamp's date part).
func Parse(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if len(s) > 10 && (s[10] == ' ' || s[10] == 'T') {
		s = s[:10]
	}
	for _, l := range []string{Layout, "20060102"} {
		if t, err := time.Parse(l, s); err == nil {
			return Date{t}, nil
		}
	}
	return Date{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
}

// MustParse is Parse for tests.
func MustParse(s string) Date {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Today is the current calendar day in the local zone.
func Today() Date {
	n := time.Now()
	return New(n.Year(), n.Month(), n.Day())
}

// Of converts a time to its calendar day in that time's zone.
func Of(t time.Time) Date { return New(t.Year(), t.Month(), t.Day()) }

func (d Date) String() string               { return d.t.Format(Layout) }
func (d Date) IsZero() bool                 { return d.t.IsZero() }
func (d Date) Year() int                    { return d.t.Year() }
func (d Date) Month() time.Month            { return d.t.Month() }
func (d Date) Day() int                     { return d.t.Day() }
func (d Date) Weekday() time.Weekday        { return d.t.Weekday() }
func (d Date) Time() time.Time              { return d.t }
func (d Date) Before(o Date) bool           { return d.t.Before(o.t) }
func (d Date) After(o Date) bool            { return d.t.After(o.t) }
func (d Date) Equal(o Date) bool            { return d.t.Equal(o.t) }
func (d Date) AddDays(n int) Date           { return Date{d.t.AddDate(0, 0, n)} }
func (d Date) Format(layout string) string  { return d.t.Format(layout) }
func (d Date) MonthKey() string             { return d.t.Format("200601") }
func (d Date) DaysUntil(o Date) int         { return int(o.t.Sub(d.t).Hours() / 24) }
func (d Date) InRange(from, to Date) bool   { return !d.Before(from) && !d.After(to) }
func (d Date) Compare(o Date) int           { return d.t.Compare(o.t) }
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// DaysIn returns the number of days in month m of year y.
func DaysIn(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

// AddMonths moves n months keeping the day of month at min(anchor, days in month),
// so 31 Jan → 28 Feb → 31 Mar instead of drifting (same rule as Kronos).
// anchor <= 0 means "use d's own day".
func (d Date) AddMonths(n, anchor int) Date {
	if anchor <= 0 {
		anchor = d.Day()
	}
	idx := d.Year()*12 + int(d.Month()) - 1 + n // months since year 0; always >= 0 here
	y, m := idx/12, time.Month(idx%12+1)
	day := anchor
	if dim := DaysIn(y, m); day > dim {
		day = dim
	}
	return New(y, m, day)
}

// Max returns the later date.
func Max(a, b Date) Date {
	if a.After(b) {
		return a
	}
	return b
}

// Min returns the earlier date.
func Min(a, b Date) Date {
	if a.Before(b) {
		return a
	}
	return b
}
