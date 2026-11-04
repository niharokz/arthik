package inbox

import (
	"sort"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
)

// paidWindow is how many days around a bill's due date a payment counts as it.
const paidWindow = 7

// Notice is a reminder (a Kronos bill) with what Arthik found about its payment.
type Notice struct {
	model.Draft
	Paid  *Same `json:"paid,omitempty"`  // a transaction that pays it
	Proof *Same `json:"proof,omitempty"` // an SMS draft in the inbox that looks like the payment
}

// Notices lists reminders, newest due date first; dismissed ones only when asked.
func Notices(d *book.Data, c *book.Computed, withDismissed bool) []Notice {
	out := []Notice{}
	for _, x := range d.Drafts {
		if x.Status != model.DraftNotice && !(withDismissed && x.Status == model.DraftDismissed) {
			continue
		}
		n := Notice{Draft: x, Paid: paidBy(d, c, x)}
		if n.Paid == nil {
			n.Proof = proofOf(d, x)
		}
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// ActiveNotices counts open reminders not paid yet (the bell's badge).
func ActiveNotices(d *book.Data, c *book.Computed) int {
	n := 0
	for _, x := range d.Drafts {
		if x.Status == model.DraftNotice && paidBy(d, c, x) == nil {
			n++
		}
	}
	return n
}

// Dismiss closes a reminder; Reopen brings it back.
func Dismiss(d *book.Data, ch *book.Change, id string) error {
	return setNotice(d, ch, id, model.DraftNotice, model.DraftDismissed)
}
func Reopen(d *book.Data, ch *book.Change, id string) error {
	return setNotice(d, ch, id, model.DraftDismissed, model.DraftNotice)
}

func setNotice(d *book.Data, ch *book.Change, id, from, to string) error {
	x, ok := d.Draft(id)
	if !ok {
		return errNotFound
	}
	if x.Status != from {
		return errf("this reminder is %s", x.Status)
	}
	x.Status = to
	d.SetDraft(x, ch)
	return nil
}

// near reports whether day is within the paid window of the reminder's due date.
func near(due, day string) bool {
	a, err1 := dates.Parse(due)
	b, err2 := dates.Parse(day)
	if err1 != nil || err2 != nil {
		return false
	}
	n := a.DaysUntil(b)
	return n >= -paidWindow && n <= paidWindow
}

// paidBy finds a recorded transaction of the same type and amount near the due date.
func paidBy(d *book.Data, c *book.Computed, x model.Draft) *Same {
	var best *Same
	for _, t := range d.Txns {
		if !c.Valid[t.ID] || t.Type != x.Type || !near(x.Date, t.Date) {
			continue
		}
		e := book.EntryOf(t)
		if e.Amount != x.Amount {
			continue
		}
		s := &Same{ID: t.ID, Title: t.Title, Date: t.Date, Amount: e.Amount}
		if best == nil || closer(x.Date, s.Date, best.Date) {
			best = s
		}
	}
	return best
}

// proofOf finds a pending SMS draft with the same amount near the due date.
func proofOf(d *book.Data, x model.Draft) *Same {
	for _, o := range d.Drafts {
		if o.Status == model.DraftPending && o.Source == SourceSMS && o.Type == x.Type && o.Amount == x.Amount && near(x.Date, o.Date) {
			return &Same{ID: o.ID, Title: o.Title, Date: o.Date, Amount: o.Amount}
		}
	}
	return nil
}

func closer(due, a, b string) bool {
	da, _ := dates.Parse(due)
	x, _ := dates.Parse(a)
	y, _ := dates.Parse(b)
	return abs(da.DaysUntil(x)) < abs(da.DaysUntil(y))
}
