// Package inbox turns what producers drop into <finance>/inbox/ into drafts that
// wait for review, and applies your decisions.
//
// Producers each own one file, so no two programs ever write the same file:
//
//	inbox/kronos.md     Kronos: subscriptions due today
//	inbox/sms_<dev>.md  the Arthik phone app: bank SMS, raw text
//
// Arthik (the main device) reads them, writes inbox/drafts.md (the review list)
// and inbox/learned.md (what a merchant becomes, learned when you accept).
package inbox

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/store"
)

// Source file names inside <finance>/inbox/.
const (
	KronosFile = "kronos.md"
	SMSGlob    = "sms_*.md"
)

// Draft sources.
const (
	SourceSMS    = "sms"
	SourceKronos = "kronos"
)

// Item is one transaction a producer offers for review.
type Item struct {
	ID           string // draft id (stable: the same message always gives the same id)
	Source       string
	Date, Time   string
	Kind         string // expense | income | transfer_in
	Amount       money.Amount
	Title        string
	Merchant     string
	Digits       string
	AccountHint  string
	CategoryHint string
	Labels       []string
	Notes        string
	Raw          string
	Rule         string
	Generic      bool
	Balance      money.Amount
	Available    money.Amount
}

type smsMessage struct {
	ID   string `yaml:"id"`
	From string `yaml:"from"`
	At   string `yaml:"at"` // RFC 3339, when the phone received it
	Body string `yaml:"body"`
}

type smsDoc struct {
	Device   string       `yaml:"device"`
	Schedule string       `yaml:"schedule"` // when the phone copies SMS ("23:30")
	LastRun  string       `yaml:"last_run"` // when it last did (RFC 3339)
	Messages []smsMessage `yaml:"messages"`
}

// Phone is one phone's SMS capture as its file reports it.
type Phone struct {
	Device   string `json:"device"`
	Schedule string `json:"schedule"`
	LastRun  string `json:"last_run"`
	Messages int    `json:"messages"`
}

// Phones lists the phones that copy SMS into inbox/ and when they do.
func Phones(dir string) []Phone {
	files, _ := filepath.Glob(filepath.Join(dir, book.InboxDir, SMSGlob))
	sort.Strings(files)
	out := []Phone{}
	for _, f := range files {
		if book.IsConflictCopy(filepath.Base(f)) {
			continue
		}
		var sd smsDoc
		if _, err := store.Read(f, &sd); err != nil {
			continue
		}
		out = append(out, Phone{Device: sd.Device, Schedule: sd.Schedule, LastRun: sd.LastRun, Messages: len(sd.Messages)})
	}
	return out
}

type kronosDraft struct {
	ID       string       `yaml:"id"`
	Source   string       `yaml:"source"` // e.g. subscription
	Date     string       `yaml:"date"`
	Title    string       `yaml:"title"`
	Type     string       `yaml:"type"`
	Amount   money.Amount `yaml:"amount"`
	Account  string       `yaml:"account"`
	Category string       `yaml:"category"`
	Labels   []string     `yaml:"labels"`
	Notes    string       `yaml:"notes"`
}

type kronosDoc struct {
	Drafts []kronosDraft `yaml:"drafts"`
}

var hexID = regexp.MustCompile(`^[0-9a-f]{16}$`)

// SMSID is the stable id of a message: the phone's id when it sent one, else a
// hash of the message.
func SMSID(m smsMessage) string {
	if hexID.MatchString(m.ID) {
		return "sms-" + m.ID
	}
	h := sha256.Sum256([]byte(m.From + "\n" + m.At + "\n" + m.Body))
	return "sms-" + hex.EncodeToString(h[:8])
}

// draftID keeps ids short enough to be transaction ids too ("d-" + id).
func draftID(prefix, id string) string {
	id = prefix + book.Slug(id)
	if len(id) > 60 {
		h := sha256.Sum256([]byte(id))
		id = id[:40] + "-" + hex.EncodeToString(h[:6])
	}
	return id
}

// ReadSources reads every producer file. A broken file is reported and skipped,
// so one bad file never blocks the others.
func ReadSources(dir string, p *Parser, loc *time.Location) ([]Item, []error) {
	k, ek := ReadKronos(dir)
	s, es := ReadSMS(dir, p, loc)
	return append(k, s...), append(ek, es...)
}

// ReadKronos reads inbox/kronos.md (subscriptions due) into items.
func ReadKronos(dir string) ([]Item, []error) {
	var items []Item
	var errs []error

	var kd kronosDoc
	if _, err := store.Read(filepath.Join(dir, book.InboxDir, KronosFile), &kd); err != nil {
		errs = append(errs, err)
	}
	for _, k := range kd.Drafts {
		if k.ID == "" || k.Amount <= 0 {
			continue
		}
		kind := KindExpense
		if k.Type == KindIncome {
			kind = KindIncome
		}
		items = append(items, Item{
			ID: draftID("kronos-", k.ID), Source: SourceKronos, Date: k.Date, Kind: kind, Amount: k.Amount,
			Title: strings.TrimSpace(k.Title), Merchant: strings.TrimSpace(k.Title), AccountHint: strings.TrimSpace(k.Account),
			CategoryHint: strings.TrimSpace(k.Category), Labels: k.Labels, Notes: strings.TrimSpace(k.Notes), Rule: k.Source,
		})
	}
	return items, errs
}

// ReadSMS reads every inbox/sms_<device>.md into items (messages no rule reads are skipped).
func ReadSMS(dir string, p *Parser, loc *time.Location) ([]Item, []error) {
	var items []Item
	var errs []error
	files, _ := filepath.Glob(filepath.Join(dir, book.InboxDir, SMSGlob))
	sort.Strings(files)
	for _, f := range files {
		if book.IsConflictCopy(filepath.Base(f)) {
			continue // the phone rewrites its own file; a stale copy adds nothing new
		}
		var sd smsDoc
		if _, err := store.Read(f, &sd); err != nil {
			errs = append(errs, err)
			continue
		}
		for _, m := range sd.Messages {
			it, ok := smsItem(m, p, loc)
			if ok {
				items = append(items, it)
			}
		}
	}
	return items, errs
}

func smsItem(m smsMessage, p *Parser, loc *time.Location) (Item, bool) {
	ps, ok := p.Parse(m.Body)
	if !ok {
		return Item{}, false
	}
	it := Item{
		ID: SMSID(m), Source: SourceSMS, Kind: ps.Kind, Amount: ps.Amount, Merchant: ps.Merchant, Digits: ps.Digits,
		Date: ps.Date, Time: ps.Time, Raw: strings.TrimSpace(m.Body), Rule: ps.Rule, Generic: ps.Generic,
		Balance: ps.Balance, Available: ps.Available,
	}
	if at, err := time.Parse(time.RFC3339, m.At); err == nil {
		at = at.In(loc)
		if it.Date == "" {
			it.Date = at.Format("2006-01-02")
		}
		if it.Time == "" && it.Date == at.Format("2006-01-02") {
			it.Time = at.Format("15:04")
		}
	}
	if it.Date == "" {
		return Item{}, false
	}
	return it, true
}

// Key normalises a merchant or name for learning: "UPI_SANJAY KUMAR ROUT" and
// "Sanjay Kumar Rout" are one key; "SWIGGYPVTLTDFOOD1" loses the trailing digits.
func Key(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	var words []string
	for _, w := range strings.Fields(b.String()) {
		if strings.Trim(w, "0123456789") == "" {
			continue // a bare number (reference, terminal id)
		}
		words = append(words, w)
	}
	k := strings.Join(words, " ")
	return strings.TrimRight(k, "0123456789")
}

// Pretty turns a raw merchant into a readable title: "GYFTR VIA SMARTBUY" → "Gyftr Via Smartbuy",
// "SWIGGYPVTLTDFOOD1" → "Swiggypvtltdfood".
func Pretty(s string) string {
	words := strings.Fields(strings.NewReplacer("_", " ", "*", " ").Replace(s))
	for i, w := range words {
		if !strings.ContainsAny(strings.ToLower(w), "aeiouy") && strings.ToUpper(w) == w {
			continue // keep acronyms without vowels (HDFC, BKC) as they are
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.TrimRight(strings.Join(words, " "), "0123456789 ")
}
