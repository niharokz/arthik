package loans

import (
	"testing"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/money"
)

func sample(t *testing.T) Loan {
	t.Helper()
	ls, err := Read("testdata")
	if err != nil || len(ls) != 1 {
		t.Fatalf("read: %v %d", err, len(ls))
	}
	return ls[0]
}

// The lender's rows come first, then the estimate runs to payoff: it should land on
// the lender's end date and last EMI.
func TestProjection(t *testing.T) {
	l := sample(t)
	rows := l.Projected(dates.MustParse("2026-11-04"))
	if len(rows) != 105 {
		t.Fatalf("rows = %d, want 105 unpaid installments", len(rows))
	}
	if rows[0].Estimated || !rows[12].Estimated || rows[0].Paid {
		t.Fatalf("known/estimated/paid flags wrong: %+v %+v", rows[0], rows[12])
	}
	last := rows[len(rows)-1]
	if last.Date != "2035-07-05" || last.Outstanding != 0 {
		t.Fatalf("last row %+v", last)
	}
	if got := last.Principal + last.Interest; (got - money.MustParse("42025.89")).Abs() > money.FromRupees(50) {
		t.Errorf("last EMI %s, lender says 42,025.89", got)
	}
}

func TestSplitFor(t *testing.T) {
	l := sample(t)
	s, ok := SplitFor([]Loan{l}, money.MustParse("42054"), dates.MustParse("2026-11-05"))
	if !ok || s.Principal != money.MustParse("21206.77") || s.Interest != money.MustParse("20847.23") || s.Account != "lic-homeloan" {
		t.Fatalf("split %+v %v", s, ok)
	}
	if _, ok := SplitFor([]Loan{l}, money.MustParse("42054"), dates.MustParse("2026-08-20")); ok {
		t.Error("an EMI months away must not match")
	}
}

func TestFY(t *testing.T) {
	l := sample(t)
	from, to := l.FY(dates.MustParse("2027-02-01"))
	if from.String() != "2026-04-01" || to.String() != "2027-03-31" {
		t.Fatalf("FY %s – %s", from, to)
	}
}

// Next year's schedule pasted at the end (overlapping a month, with a new rate)
// continues the table; the estimate starts after it and uses the new rate.
func TestAppendedSchedule(t *testing.T) {
	l := sample(t)
	l.ScheduleText += "Due Date\tInterest Rate\tEMI (Prin.)\tEMI (Int.)\tTotal EMI\tSchedule OS\n" +
		"05-OCT-27\t7.85%%\t₹ 22,783.68\t₹ 19,270.32\t₹ 42,054.00\t₹ 2,922,996.73\n" +
		"05-NOV-27\t7.50%%\t₹ 23,785.27\t₹ 18,268.73\t₹ 42,054.00\t₹ 2,899,211.46\n"
	rows, problems := l.Rows()
	if len(rows) != 13 || len(problems) != 0 || rows[12].Date != "2027-11-05" || rows[12].Rate != money.MustParse("7.50") {
		t.Fatalf("rows %d problems %v last %+v", len(rows), problems, rows[len(rows)-1])
	}
	p := l.Projected(dates.MustParse("2026-11-04"))
	if p[12].Estimated || !p[13].Estimated || p[13].Interest != money.MustParse("2899211.46").MulDiv(750, 120000) {
		t.Fatalf("estimate after the pasted rows: %+v", p[13])
	}
}

// A missing month or a garbled line is reported, not silently used.
func TestScheduleProblems(t *testing.T) {
	l := sample(t)
	l.ScheduleText += "05-DEC-27\t7.85%%\t₹ 22,931.72\t₹ 19,122.28\t₹ 42,054.00\t₹ 2,876,279.74\n05-JAN-28 oops\n"
	_, problems := l.Rows()
	if len(problems) != 2 {
		t.Fatalf("problems %v", problems)
	}
}
