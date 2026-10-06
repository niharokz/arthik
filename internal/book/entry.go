package book

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// Entry is the simple, Bluecoins-shaped form of a transaction that the UI, the CSV
// importer and the reminder scheduler work with. BuildTx turns it into balanced
// double-entry postings; EntryOf turns postings back into an Entry for editing.
type Entry struct {
	ID        string          `json:"id,omitempty"`
	Type      string          `json:"type"` // expense | income | transfer | journal
	Date      string          `json:"date"`
	Time      string          `json:"time,omitempty"`
	Title     string          `json:"title"`
	Amount    money.Amount    `json:"amount"`
	Account   string          `json:"account,omitempty"`
	ToAccount string          `json:"to_account,omitempty"`
	Category  string          `json:"category,omitempty"`
	Splits    []Split         `json:"splits,omitempty"`
	Labels    []string        `json:"labels,omitempty"`
	Notes     string          `json:"notes,omitempty"`
	Status    string          `json:"status,omitempty"`
	Reminder  string          `json:"reminder,omitempty"`
	Postings  []model.Posting `json:"postings,omitempty"` // journal type only
}

// Split is one category line of a split expense/income.
type Split struct {
	Category string       `json:"category"`
	Amount   money.Amount `json:"amount"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a name into an id fragment: "HDFC Savings" → "hdfc-savings".
func Slug(s string) string {
	s = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		s = "item"
	}
	return s
}

func uniqueID(base string, taken func(string) bool) string {
	id := base
	for n := 2; taken(id); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// UniqueAccountID / UniqueCategoryID / UniqueLabelID / UniqueReminderID make a
// fresh id from a name.
func (d *Data) UniqueAccountID(name string) string {
	return uniqueID(Slug(name), func(s string) bool { _, ok := d.idx.acc[s]; return ok })
}
func (d *Data) UniqueCategoryID(name, parent string) string {
	base := Slug(name)
	if parent != "" {
		base = parent + "-" + base
	}
	return uniqueID(base, func(s string) bool { _, ok := d.idx.cat[s]; return ok })
}
func (d *Data) UniqueLabelID(name string) string {
	return uniqueID(Slug(name), func(s string) bool { _, ok := d.idx.lab[s]; return ok })
}
func (d *Data) UniqueReminderID(title string) string {
	return uniqueID("r-"+Slug(title), func(s string) bool { _, ok := d.idx.rem[s]; return ok })
}

// NewTxID returns "t-YYYYMMDD-<6 hex>" (random part, retried if taken).
func NewTxID(date string, taken func(string) bool) string {
	day := strings.ReplaceAll(date, "-", "")
	if len(day) != 8 {
		day = dates.Today().Format("20060102")
	}
	for {
		var b [3]byte
		_, _ = rand.Read(b[:])
		id := "t-" + day + "-" + hex.EncodeToString(b[:])
		if taken == nil || !taken(id) {
			return id
		}
	}
}

// NewTxID makes an id not used in this dataset.
func (d *Data) NewTxID(date string) string {
	return NewTxID(date, func(s string) bool { _, ok := d.idx.tx[s]; return ok })
}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,79}$`)

// ValidID reports whether s is usable as a record id (client-generated ids for
// offline transactions included).
func ValidID(s string) bool { return validID.MatchString(s) }

// BuildTx converts an Entry into a validated, balanced transaction.
func (d *Data) BuildTx(e Entry) (model.Transaction, error) {
	t := model.Transaction{
		ID: e.ID, Date: strings.TrimSpace(e.Date), Time: strings.TrimSpace(e.Time),
		Type: e.Type, Title: strings.TrimSpace(e.Title), Status: e.Status,
		Notes: strings.TrimSpace(e.Notes), Reminder: e.Reminder,
	}
	for _, l := range e.Labels {
		if l = strings.TrimSpace(l); l != "" && !contains(t.Labels, l) {
			t.Labels = append(t.Labels, l)
		}
	}
	if t.Status == "" {
		t.Status = model.StatusUncleared
	}
	switch e.Type {
	case model.TxExpense, model.TxIncome:
		if e.Account == "" {
			return t, fmt.Errorf("choose an account")
		}
		sign := money.Amount(1) // expense: category debit (+), account credit (-)
		if e.Type == model.TxIncome {
			sign = -1
		}
		var total money.Amount
		if len(e.Splits) > 0 {
			for i, s := range e.Splits {
				if s.Category == "" || s.Amount == 0 {
					return t, fmt.Errorf("split line %d needs a category and an amount", i+1)
				}
				t.Postings = append(t.Postings, model.Posting{Category: s.Category, Amount: sign * s.Amount})
				total += s.Amount
			}
		} else {
			if e.Category == "" {
				return t, fmt.Errorf("choose a category")
			}
			if e.Amount == 0 {
				return t, fmt.Errorf("enter an amount")
			}
			t.Postings = append(t.Postings, model.Posting{Category: e.Category, Amount: sign * e.Amount})
			total = e.Amount
		}
		t.Postings = append([]model.Posting{{Account: e.Account, Amount: -sign * total}}, t.Postings...)
	case model.TxTransfer:
		if e.Account == "" || e.ToAccount == "" {
			return t, fmt.Errorf("choose both accounts for a transfer")
		}
		if e.Account == e.ToAccount {
			return t, fmt.Errorf("a transfer needs two different accounts")
		}
		if e.Amount <= 0 {
			return t, fmt.Errorf("enter a positive transfer amount")
		}
		t.Postings = []model.Posting{{Account: e.Account, Amount: -e.Amount}, {Account: e.ToAccount, Amount: e.Amount}}
	case model.TxJournal:
		t.Postings = append([]model.Posting(nil), e.Postings...)
	default:
		return t, fmt.Errorf("unknown transaction type %q", e.Type)
	}
	return t, d.ValidateTx(t)
}

// ValidateTx checks a transaction against the double-entry rules and the dataset.
func (d *Data) ValidateTx(t model.Transaction) error {
	if t.ID != "" && !ValidID(t.ID) {
		return fmt.Errorf("invalid id %q", t.ID)
	}
	if _, err := dates.Parse(t.Date); err != nil {
		return err
	}
	if t.Time != "" && !timeRe.MatchString(t.Time) {
		return fmt.Errorf("invalid time %q (want HH:MM)", t.Time)
	}
	switch t.Status {
	case "", model.StatusUncleared, model.StatusCleared, model.StatusReconciled:
	default:
		return fmt.Errorf("invalid status %q", t.Status)
	}
	if t.Title == "" {
		return fmt.Errorf("enter a title")
	}
	if len(t.Postings) < 2 {
		return fmt.Errorf("a transaction needs at least two postings")
	}
	accounts, categories := 0, 0
	for i, p := range t.Postings {
		switch {
		case p.Account != "" && p.Category != "":
			return fmt.Errorf("posting %d has both an account and a category", i+1)
		case p.Account != "":
			if _, ok := d.Account(p.Account); !ok {
				return fmt.Errorf("unknown account %q", p.Account)
			}
			accounts++
		case p.Category != "":
			if _, ok := d.Category(p.Category); !ok {
				return fmt.Errorf("unknown category %q", p.Category)
			}
			categories++
		default:
			return fmt.Errorf("posting %d has neither an account nor a category", i+1)
		}
		if p.Amount == 0 {
			return fmt.Errorf("posting %d has a zero amount", i+1)
		}
	}
	if s := t.Sum(); s != 0 {
		return fmt.Errorf("postings do not balance (off by %s)", s.String())
	}
	switch t.Type {
	case model.TxExpense, model.TxIncome:
		if accounts != 1 || categories < 1 {
			return fmt.Errorf("%s needs one account and at least one category", t.Type)
		}
	case model.TxTransfer:
		if categories != 0 || accounts != 2 {
			return fmt.Errorf("a transfer moves money between exactly two accounts")
		}
	case model.TxJournal:
	default:
		return fmt.Errorf("unknown transaction type %q", t.Type)
	}
	return nil
}

// EntryOf turns a stored transaction back into the UI form.
func EntryOf(t model.Transaction) Entry {
	e := Entry{ID: t.ID, Type: t.Type, Date: t.Date, Time: t.Time, Title: t.Title, Labels: t.Labels,
		Notes: t.Notes, Status: t.Status, Reminder: t.Reminder}
	if e.Status == "" {
		e.Status = model.StatusUncleared
	}
	var accs, cats []model.Posting
	for _, p := range t.Postings {
		if p.Account != "" {
			accs = append(accs, p)
		} else {
			cats = append(cats, p)
		}
	}
	switch {
	case (t.Type == model.TxExpense || t.Type == model.TxIncome) && len(accs) == 1 && len(cats) >= 1:
		sign := money.Amount(1)
		if t.Type == model.TxIncome {
			sign = -1
		}
		e.Account = accs[0].Account
		e.Amount = -sign * accs[0].Amount
		if len(cats) == 1 {
			e.Category = cats[0].Category
		} else {
			for _, c := range cats {
				e.Splits = append(e.Splits, Split{Category: c.Category, Amount: sign * c.Amount})
			}
		}
	case t.Type == model.TxTransfer && len(accs) == 2 && len(cats) == 0:
		from, to := accs[0], accs[1]
		if from.Amount > 0 {
			from, to = to, from
		}
		e.Account, e.ToAccount, e.Amount = from.Account, to.Account, to.Amount
	default:
		e.Type = model.TxJournal
		e.Postings = t.Postings
		for _, p := range t.Postings {
			if p.Amount > 0 {
				e.Amount += p.Amount
			}
		}
	}
	return e
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// EnsureLabels maps label names/ids to ids, creating labels that do not exist yet
// (so typing a new label on a transaction adds it to labels.md automatically).
func (d *Data) EnsureLabels(in []string, ch *Change) []string {
	var out []string
	for _, l := range in {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		id := ""
		if _, ok := d.idx.lab[l]; ok {
			id = l
		} else {
			for _, x := range d.Labels {
				if strings.EqualFold(x.Name, l) {
					id = x.ID
					break
				}
			}
		}
		if id == "" {
			id = d.UniqueLabelID(l)
			d.Labels = append(d.Labels, model.Label{ID: id, Name: l})
			d.idx.lab[id] = len(d.Labels) - 1
			ch.Labels = true
		}
		if !contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
