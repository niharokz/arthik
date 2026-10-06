package period

import (
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/dates"
)

func TestOf(t *testing.T) {
	o := DefaultOpts()
	d := dates.MustParse("2026-10-06") // a Tuesday
	cases := []struct {
		k        Kind
		o        Opts
		from, to string
	}{
		{Weekly, o, "2026-10-05", "2026-10-11"},
		{Weekly, Opts{WeekStart: time.Sunday}, "2026-10-04", "2026-10-10"},
		{Monthly, o, "2026-10-01", "2026-10-31"},
		{Monthly, Opts{MonthStartDay: 25}, "2026-09-25", "2026-10-24"},
		{Quarterly, o, "2026-10-01", "2026-12-31"},
		{HalfYearly, o, "2026-07-01", "2026-12-31"},
		{Yearly, o, "2026-01-01", "2026-12-31"},
		{Yearly, Opts{YearStartMonth: 4}, "2026-04-01", "2027-03-31"},
		{Quarterly, Opts{YearStartMonth: 4}, "2026-10-01", "2026-12-31"},
		{HalfYearly, Opts{YearStartMonth: 4}, "2026-10-01", "2027-03-31"},
	}
	for _, c := range cases {
		r := Of(c.k, d, c.o)
		if r.From.String() != c.from || r.To.String() != c.to {
			t.Errorf("%s %+v: got %s..%s want %s..%s", c.k, c.o, r.From, r.To, c.from, c.to)
		}
	}
}

func TestShiftAndLabels(t *testing.T) {
	o := DefaultOpts()
	r := Of(Monthly, dates.MustParse("2026-01-31"), o)
	if p := r.Shift(-1, o); p.From.String() != "2025-12-01" || p.Label != "Dec 2025" {
		t.Errorf("prev month = %+v", p)
	}
	if n := Of(Yearly, dates.MustParse("2026-05-01"), Opts{YearStartMonth: 4}); n.Label != "FY 2026–27" {
		t.Errorf("FY label = %q", n.Label)
	}
	c := Between(dates.MustParse("2026-10-10"), dates.MustParse("2026-10-01"))
	if c.From.String() != "2026-10-01" || c.Days() != 10 {
		t.Errorf("custom = %+v", c)
	}
	if s := c.Shift(1, o); s.From.String() != "2026-10-11" || s.To.String() != "2026-10-20" {
		t.Errorf("custom shift = %+v", s)
	}
	if b := Buckets(Monthly, dates.MustParse("2026-01-15"), dates.MustParse("2026-03-10"), o); len(b) != 3 || b[0].From.String() != "2026-01-15" || b[2].To.String() != "2026-03-10" {
		t.Errorf("buckets = %+v", b)
	}
}
