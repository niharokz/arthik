package book

import (
	"context"
	"log"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/recur"
)

// Future-dated transactions need no job: they are stored like any other transaction
// and simply start counting once their date is today or earlier. The scheduler only
// has to (a) rebuild computed values when the day changes and (b) auto-post
// reminders whose next date has arrived.

// maxCatchUp bounds how many occurrences one reminder may post in one pass
// (e.g. a daily reminder created with a start date years ago).
const maxCatchUp = 1000

// ReminderTxID is the deterministic id of a reminder's occurrence, so a re-run can
// never post the same occurrence twice.
func ReminderTxID(reminderID string, d dates.Date) string {
	return "r-" + strings.TrimPrefix(reminderID, "r-") + "-" + d.Format("20060102")
}

// PostDue posts every due occurrence (next <= today) of every active reminder and
// advances the reminders. It is called by the scheduler and right after a reminder
// is created or edited, so a start date in the past back-fills immediately.
func (d *Data) PostDue(today dates.Date, ch *Change) (posted int) {
	for i := range d.Reminders {
		r := d.Reminders[i] // copy; written back below
		if r.Paused || r.Done || r.Next == "" {
			continue
		}
		rule, err := recur.Parse(r.Repeat)
		if err != nil {
			continue // reported by validation in the UI; leave the record alone
		}
		next, err := dates.Parse(r.Next)
		if err != nil {
			continue
		}
		var end dates.Date
		if r.EndDate != "" {
			end, _ = dates.Parse(r.EndDate)
		}
		changed := false
		for n := 0; n < maxCatchUp && !next.After(today) && !r.Done; n++ {
			if !end.IsZero() && next.After(end) {
				r.Done = true
				changed = true
				break
			}
			id := ReminderTxID(r.ID, next)
			if _, exists := d.Tx(id); !exists {
				e := Entry{ID: id, Type: r.Type, Date: next.String(), Title: r.Title, Amount: r.Amount,
					Account: r.Account, ToAccount: r.ToAccount, Category: r.Category,
					Labels: r.Labels, Notes: r.Notes, Status: r.Status, Reminder: r.ID}
				t, err := d.BuildTx(e)
				if err != nil {
					log.Printf("WARN reminder %s could not post %s: %v", r.ID, next, err)
					break // try again next tick (e.g. account was renamed); don't skip silently
				}
				t.Labels = d.EnsureLabels(t.Labels, ch)
				d.SetTx(t, ch)
				posted++
			}
			r.Posted++
			changed = true
			if r.Remaining > 0 {
				r.Remaining--
				if r.Remaining == 0 {
					r.Done = true
				}
			}
			if !rule.Repeats() {
				r.Done = true
				break
			}
			next, r.AnchorDay = rule.Next(next, r.AnchorDay)
			r.Next = next.String()
			if !end.IsZero() && next.After(end) {
				r.Done = true
			}
		}
		if changed {
			d.Reminders[i] = r
			ch.Reminders = true
		}
	}
	return posted
}

// Upcoming lists reminder occurrences between from and to (inclusive) without
// posting anything — used for the dashboard and "Scheduled" screens.
func (d *Data) Upcoming(from, to dates.Date) []Entry {
	var out []Entry
	for _, r := range d.Reminders {
		if r.Paused || r.Done {
			continue
		}
		rule, err := recur.Parse(r.Repeat)
		if err != nil {
			continue
		}
		next, err := dates.Parse(r.Next)
		if err != nil {
			continue
		}
		var end dates.Date
		if r.EndDate != "" {
			end, _ = dates.Parse(r.EndDate)
		}
		anchor, left := r.AnchorDay, r.Remaining
		for n := 0; n < 400 && !next.After(to); n++ {
			if !end.IsZero() && next.After(end) {
				break
			}
			if !next.Before(from) {
				out = append(out, Entry{ID: ReminderTxID(r.ID, next), Type: r.Type, Date: next.String(), Title: r.Title,
					Amount: r.Amount, Account: r.Account, ToAccount: r.ToAccount, Category: r.Category,
					Labels: r.Labels, Notes: r.Notes, Reminder: r.ID})
			}
			if left > 0 {
				left--
				if left == 0 {
					break
				}
			}
			if !rule.Repeats() {
				break
			}
			next, anchor = rule.Next(next, anchor)
		}
	}
	return out
}

// RunScheduler posts due reminders now and then every interval, and rebuilds the
// computed values whenever the calendar day changes (so a future-dated transaction
// starts counting on its date without anyone touching it).
func (b *Book) RunScheduler(ctx context.Context, interval time.Duration) {
	b.tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.tick()
		}
	}
}

func (b *Book) tick() {
	today := b.Today()
	b.mu.RLock()
	dayChanged := b.comp == nil || !b.comp.Today.Equal(today)
	due := false
	if b.data != nil {
		for _, r := range b.data.Reminders {
			if r.Paused || r.Done {
				continue
			}
			if n, err := dates.Parse(r.Next); err == nil && !n.After(today) {
				due = true
				break
			}
		}
	}
	loadErr := b.loadErr
	b.mu.RUnlock()

	if due && loadErr == nil {
		if b.ReadOnly {
			// Demo: post in memory only, never touch its files.
			b.mu.Lock()
			var ch Change
			if n := b.data.PostDue(today, &ch); n > 0 {
				b.data.sortTxns()
				b.data.reindex()
			}
			b.recomputeLocked(false)
			b.mu.Unlock()
			return
		}

		var posted int
		err := b.Mutate(func(d *Data, ch *Change) error {
			posted = d.PostDue(today, ch)
			return nil
		})
		if err != nil {
			log.Printf("[%s] WARN scheduled posting failed: %v", b.Name, err)
		} else if posted > 0 {
			log.Printf("[%s] auto-posted %d scheduled transaction(s)", b.Name, posted)
			return // Mutate already rebuilt everything
		}
	}
	if dayChanged {
		b.Recompute()
	}
}

// ReminderTypeOK reports whether a reminder's type is one the scheduler can post.
func ReminderTypeOK(t string) bool {
	return t == model.TxExpense || t == model.TxIncome || t == model.TxTransfer
}
