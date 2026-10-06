package recur

import (
	"testing"

	"gitlab.com/niharokz/arthik/internal/dates"
)

func TestMonthEndAnchor(t *testing.T) {
	r, _ := Parse("monthly")
	d, a := dates.MustParse("2026-01-31"), 0
	var got []string
	for i := 0; i < 3; i++ {
		d, a = r.Next(d, a)
		got = append(got, d.String())
	}
	want := []string{"2026-02-28", "2026-03-31", "2026-04-30"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]Rule{"none": {}, "weekly": {1, "week"}, "halfyearly": {6, "month"}, "3 days": {3, "day"}, "2 years": {24, "month"}} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v %v", in, got, err)
		}
	}
	if _, err := Parse("sometimes"); err == nil {
		t.Error("expected error")
	}
	r, _ := Parse("2 weeks")
	if d, _ := r.Next(dates.MustParse("2026-10-06"), 0); d.String() != "2026-10-20" {
		t.Errorf("2 weeks = %s", d)
	}
}
