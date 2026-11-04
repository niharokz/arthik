// Package loans is a personal plugin for EMI loans (e.g. a home loan): the lender's
// summary, the repayment schedule (known rows from the lender, the rest estimated
// from the rate), what was paid this financial year, and the principal/interest
// split of an EMI so one bank debit becomes interest (a category) + principal (the
// loan account).
//
// It is on when <data>/loans.md exists. The file is yours: Arthik only reads it.
// Each year, paste the lender's new schedule under `schedule_text` exactly as the
// lender shows it (or add `schedule` rows); rows are merged by due date, sorted and
// checked, and the estimate starts after the last row you have.
package loans

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/plugin"
	"gitlab.com/niharokz/arthik/internal/store"
)

// File is the plugin's data file in the data folder.
const File = "loans.md"

// maxRows caps the projected schedule (40 years of monthly EMIs).
const maxRows = 480

func init() {
	plugin.Add(plugin.Plugin{ID: "loans", Title: "Loans", Icon: "🏛️", File: File, Register: register})
}

// Row is one EMI of the repayment schedule.
type Row struct {
	Date        string       `yaml:"date" json:"date"`
	Rate        money.Amount `yaml:"rate,omitempty" json:"rate,omitempty"` // % a year for this EMI (pasted rows carry it)
	Principal   money.Amount `yaml:"principal" json:"principal"`
	Interest    money.Amount `yaml:"interest" json:"interest"`
	Outstanding money.Amount `yaml:"outstanding" json:"outstanding"` // after this EMI
	Estimated   bool         `yaml:"-" json:"estimated,omitempty"`   // projected from the rate
	Paid        bool         `yaml:"-" json:"paid,omitempty"`        // due date has passed
}

// Statement is the lender's summary as of a date.
type Statement struct {
	AsOf                 string       `yaml:"as_of" json:"as_of"`
	PrincipalOutstanding money.Amount `yaml:"principal_outstanding" json:"principal_outstanding"`
	UnpaidInstallments   int          `yaml:"unpaid_installments" json:"unpaid_installments"`
	FYPrincipalPaid      money.Amount `yaml:"fy_principal_paid" json:"fy_principal_paid"`
	FYInterestPaid       money.Amount `yaml:"fy_interest_paid" json:"fy_interest_paid"`
}

// Loan is one entry of loans.md.
type Loan struct {
	ID               string       `yaml:"id" json:"id"`
	Name             string       `yaml:"name" json:"name"`
	Lender           string       `yaml:"lender" json:"lender"`
	Number           string       `yaml:"loan_number" json:"loan_number"`
	Office           string       `yaml:"area_office" json:"area_office"`
	Account          string       `yaml:"account" json:"account"`                     // Arthik loan account (principal)
	PayFrom          string       `yaml:"pay_from" json:"pay_from"`                   // account the EMI leaves
	InterestCategory string       `yaml:"interest_category" json:"interest_category"` // expense category for interest
	SanctionDate     string       `yaml:"sanction_date" json:"sanction_date"`
	SanctionAmount   money.Amount `yaml:"sanction_amount" json:"sanction_amount"`
	FirstDisbursed   string       `yaml:"first_disbursement" json:"first_disbursement"`
	Disbursed        money.Amount `yaml:"disbursed" json:"disbursed"`
	Rate             money.Amount `yaml:"rate" json:"rate"` // % a year, e.g. 7.85
	EMI              money.Amount `yaml:"emi" json:"emi"`
	EMIDay           int          `yaml:"emi_day" json:"emi_day"`
	EndDate          string       `yaml:"end_date" json:"end_date"`
	LastEMI          money.Amount `yaml:"last_emi" json:"last_emi"`
	FYStartMonth     int          `yaml:"fy_start_month" json:"fy_start_month"` // tax year start (4 = April)
	Statement        Statement    `yaml:"statement" json:"statement"`
	Schedule         []Row        `yaml:"schedule" json:"-"`
	ScheduleText     string       `yaml:"schedule_text" json:"-"` // the lender's table, pasted as is
}

// lenderLine reads one line of a lender's table:
// "05-NOV-26  7.85%%  ₹ 21,206.77  ₹ 20,847.23  ₹ 42,054.00  ₹ 3,165,629.92"
// (due date, rate, principal, interest, total EMI, outstanding after it).
var (
	lenderDate = regexp.MustCompile(`^\s*(\d{1,2}-[A-Za-z]{3}-\d{2,4}|\d{4}-\d{2}-\d{2})\b`)
	lenderNum  = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)
)

// parseText reads the pasted table; header and blank lines are skipped, a line
// that starts with a date but cannot be read is reported.
func parseText(text string) ([]Row, []string) {
	var rows []Row
	var problems []string
	for n, line := range strings.Split(text, "\n") {
		m := lenderDate.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		day, err := parseDay(m[1])
		nums := lenderNum.FindAllString(line[len(m[0]):], -1)
		if err != nil || len(nums) < 5 {
			problems = append(problems, fmt.Sprintf("schedule_text line %d could not be read: %q", n+1, strings.TrimSpace(line)))
			continue
		}
		vals := make([]money.Amount, 0, len(nums))
		for _, x := range nums {
			v, err := money.Parse(x)
			if err != nil {
				break
			}
			vals = append(vals, v)
		}
		if len(vals) != len(nums) {
			problems = append(problems, fmt.Sprintf("schedule_text line %d has an unreadable amount", n+1))
			continue
		}
		// rate, principal, interest, total, outstanding — the total is only a check
		r := Row{Date: day.String(), Rate: vals[0], Principal: vals[1], Interest: vals[2], Outstanding: vals[len(vals)-1]}
		if vals[1]+vals[2] != vals[3] {
			problems = append(problems, fmt.Sprintf("schedule_text %s: principal + interest is not the EMI", r.Date))
		}
		rows = append(rows, r)
	}
	return rows, problems
}

// parseDay reads "05-NOV-26", "05-Nov-2026" or "2026-11-05".
func parseDay(s string) (dates.Date, error) {
	if d, err := dates.Parse(s); err == nil {
		return d, nil
	}
	for _, l := range []string{"02-Jan-06", "2-Jan-06", "02-Jan-2006", "2-Jan-2006"} { // month names match in any case
		if t, err := time.Parse(l, s); err == nil {
			return dates.Of(t), nil
		}
	}
	return dates.Date{}, fmt.Errorf("invalid date %q", s)
}

// Rows merges `schedule` and `schedule_text` by due date (a later entry for the same
// date wins), sorted, and checks that each row continues from the one before.
func (l Loan) Rows() ([]Row, []string) {
	pasted, problems := parseText(l.ScheduleText)
	byDate := map[string]Row{}
	for _, r := range append(append([]Row(nil), l.Schedule...), pasted...) {
		d, err := parseDay(r.Date)
		if err != nil {
			problems = append(problems, fmt.Sprintf("schedule row with an invalid date %q", r.Date))
			continue
		}
		r.Date = d.String()
		if old, ok := byDate[r.Date]; ok && (old.Principal != r.Principal || old.Interest != r.Interest) {
			problems = append(problems, fmt.Sprintf("schedule %s appears twice with different amounts — the later one is used", r.Date))
		}
		byDate[r.Date] = r
	}
	rows := make([]Row, 0, len(byDate))
	for _, r := range byDate {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
	for i := 1; i < len(rows); i++ {
		if want := rows[i-1].Outstanding - rows[i].Principal; (want - rows[i].Outstanding).Abs() > 100 {
			problems = append(problems, fmt.Sprintf("schedule %s does not continue from %s (outstanding %s, expected %s) — a month may be missing",
				rows[i].Date, rows[i-1].Date, rows[i].Outstanding.Format(), want.Format()))
		}
	}
	return rows, problems
}

type doc struct {
	Loans []Loan `yaml:"loans"`
}

// Read loads loans.md from dir (no file = no loans).
func Read(dir string) ([]Loan, error) {
	var d doc
	if _, err := store.Read(filepath.Join(dir, File), &d); err != nil {
		return nil, err
	}
	return d.Loans, nil
}

// Projected is the full schedule: the lender's rows, then estimated rows from the
// rate until the loan is paid off. Rows due on or before today are marked paid.
func (l Loan) Projected(today dates.Date) []Row {
	rows, _ := l.Rows()
	rate := l.Rate
	if n := len(rows); n > 0 && rows[n-1].Rate > 0 {
		rate = rows[n-1].Rate // the estimate uses the latest rate the lender gave
	}
	out, next := l.Statement.PrincipalOutstanding, dates.Date{}
	if n := len(rows); n > 0 {
		out = rows[n-1].Outstanding
		if d, err := dates.Parse(rows[n-1].Date); err == nil {
			next = d.AddMonths(1, l.EMIDay)
		}
	} else if d, err := dates.Parse(l.Statement.AsOf); err == nil {
		next = dates.New(d.Year(), d.Month(), 1).AddMonths(0, l.EMIDay)
		if next.Before(d) {
			next = next.AddMonths(1, l.EMIDay)
		}
	}
	for i := 0; out > 0 && l.EMI > 0 && !next.IsZero() && i < maxRows; i++ {
		interest := out.MulDiv(int64(rate), 120000) // rate is in hundredths of a percent
		principal := l.EMI - interest
		if principal <= 0 {
			break // the EMI does not cover the interest: stop rather than loop forever
		}
		if principal > out {
			principal = out
		}
		out -= principal
		rows = append(rows, Row{Date: next.String(), Principal: principal, Interest: interest, Outstanding: out, Estimated: true})
		next = next.AddMonths(1, l.EMIDay)
	}
	for i := range rows {
		if d, err := dates.Parse(rows[i].Date); err == nil && !d.After(today) {
			rows[i].Paid = true
		}
	}
	return rows
}

// FY returns the tax year that contains day.
func (l Loan) FY(day dates.Date) (from, to dates.Date) {
	m := l.FYStartMonth
	if m < 1 || m > 12 {
		m = 4
	}
	y := day.Year()
	if int(day.Month()) < m {
		y--
	}
	from = dates.New(y, time.Month(m), 1)
	return from, from.AddMonths(12, 1).AddDays(-1)
}

// Recorded sums what Arthik's transactions paid on the loan between from and to:
// principal = money into the loan account, interest = the interest category.
func (l Loan) Recorded(d *book.Data, c *book.Computed, from, to dates.Date) (principal, interest money.Amount) {
	for _, t := range d.Txns {
		td, err := dates.Parse(t.Date)
		if err != nil || !c.Valid[t.ID] || !td.InRange(from, to) || td.After(c.Today) {
			continue
		}
		for _, p := range t.Postings {
			switch {
			case p.Account != "" && p.Account == l.Account && p.Amount > 0:
				principal += p.Amount
			case p.Category != "" && p.Category == l.InterestCategory:
				interest += p.Amount
			}
		}
	}
	return
}

// Split is how one EMI divides, for the transaction form.
type Split struct {
	Loan      string       `json:"loan"`
	Name      string       `json:"name"`
	Date      string       `json:"date"`
	From      string       `json:"from"`
	Account   string       `json:"account"`
	Category  string       `json:"category"`
	Principal money.Amount `json:"principal"`
	Interest  money.Amount `json:"interest"`
	Estimated bool         `json:"estimated,omitempty"`
}

// SplitFor finds the EMI of amount due nearest to day (within half a month).
func SplitFor(loans []Loan, amount money.Amount, day dates.Date) (Split, bool) {
	for _, l := range loans {
		for _, r := range l.Projected(day) {
			if r.Principal+r.Interest != amount {
				continue
			}
			rd, err := dates.Parse(r.Date)
			if err != nil {
				continue
			}
			if n := day.DaysUntil(rd); n < -16 || n > 16 {
				continue
			}
			return Split{Loan: l.ID, Name: l.Name, Date: r.Date, From: l.PayFrom, Account: l.Account, Category: l.InterestCategory,
				Principal: r.Principal, Interest: r.Interest, Estimated: r.Estimated}, true
		}
	}
	return Split{}, false
}

// View is a loan with its computed figures, as the plugin screen shows it.
type View struct {
	Loan
	Rows         []Row          `json:"schedule"`
	Balance      money.Amount   `json:"balance"`        // the Arthik loan account today (owed: negative)
	Remaining    int            `json:"remaining"`      // EMIs left
	InterestLeft money.Amount   `json:"interest_left"`  // interest still to pay (estimated)
	PayoffDate   string         `json:"payoff_date"`    // last EMI by the schedule
	Next         *Row           `json:"next,omitempty"` // next EMI due
	FYFrom       string         `json:"fy_from"`
	FYTo         string         `json:"fy_to"`
	FYPrincipal  money.Amount   `json:"fy_principal"` // statement + recorded after the statement date
	FYInterest   money.Amount   `json:"fy_interest"`
	Problems     []string       `json:"problems"`
	EMIs         []money.Amount `json:"emis"` // amounts the form offers to split
}

func viewOf(l Loan, d *book.Data, c *book.Computed) View {
	_, problems := l.Rows()
	v := View{Loan: l, Rows: l.Projected(c.Today), Problems: append([]string{}, problems...)}
	if a, ok := d.Account(l.Account); ok {
		v.Balance = c.Balance[a.ID]
	} else {
		v.Problems = append(v.Problems, fmt.Sprintf("account %q is not in accounts.md", l.Account))
	}
	if _, ok := d.Account(l.PayFrom); !ok {
		v.Problems = append(v.Problems, fmt.Sprintf("pay_from account %q is not in accounts.md", l.PayFrom))
	}
	if _, ok := d.Category(l.InterestCategory); !ok {
		v.Problems = append(v.Problems, fmt.Sprintf("interest_category %q is not in categories.md — EMI splits need it", l.InterestCategory))
	}
	seen := map[money.Amount]bool{}
	for i := range v.Rows {
		r := v.Rows[i]
		if amt := r.Principal + r.Interest; !seen[amt] {
			seen[amt] = true
			v.EMIs = append(v.EMIs, amt)
		}
		if r.Paid {
			continue
		}
		v.Remaining++
		v.InterestLeft += r.Interest
		if v.Next == nil {
			v.Next = &v.Rows[i]
		}
	}
	if n := len(v.Rows); n > 0 {
		v.PayoffDate = v.Rows[n-1].Date
	}
	from, to := l.FY(c.Today)
	v.FYFrom, v.FYTo = from.String(), to.String()
	after := from
	if asOf, err := dates.Parse(l.Statement.AsOf); err == nil && asOf.InRange(from, to) {
		v.FYPrincipal, v.FYInterest = l.Statement.FYPrincipalPaid, l.Statement.FYInterestPaid
		after = asOf.AddDays(1)
	}
	p, i := l.Recorded(d, c, after, to)
	v.FYPrincipal += p
	v.FYInterest += i
	return v
}

func register(rt *httpx.Router) {
	rt.Handle("GET /api/plugins/loans", func(c *httpx.Ctx) error {
		loans, err := Read(c.Book.Dir)
		if err != nil {
			return err
		}
		out := []View{}
		err = c.Book.View(func(d *book.Data, comp *book.Computed) error {
			for _, l := range loans {
				out = append(out, viewOf(l, d, comp))
			}
			return nil
		})
		if err != nil {
			return err
		}
		return c.OK(map[string]any{"loans": out})
	})
	// ?amount=42054&date=2026-11-05 → how that EMI splits.
	rt.Handle("GET /api/plugins/loans/split", func(c *httpx.Ctx) error {
		loans, err := Read(c.Book.Dir)
		if err != nil {
			return err
		}
		amount, err := money.Parse(c.Q("amount"))
		if err != nil {
			return err
		}
		day, err := dates.Parse(c.Q("date"))
		if err != nil {
			return err
		}
		s, ok := SplitFor(loans, amount, day)
		if !ok {
			return httpx.NotFound("an EMI of " + amount.Format() + " near " + day.String())
		}
		return c.OK(s)
	})
}
