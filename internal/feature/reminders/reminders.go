// Package reminders is the Scheduled feature: recurring/scheduled transactions
// (bills, salary, SIPs …) that Arthik auto-posts on each due date, plus the
// "upcoming" list that merges them with future-dated transactions.
package reminders

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/recur"
	"gitlab.com/niharokz/arthik/internal/view"
)

type input struct {
	Title     string       `json:"title"`
	Type      string       `json:"type"`
	Amount    money.Amount `json:"amount"`
	Account   string       `json:"account"`
	ToAccount string       `json:"to_account"`
	Category  string       `json:"category"`
	Labels    []string     `json:"labels"`
	Notes     string       `json:"notes"`
	Status    string       `json:"status"`
	Repeat    string       `json:"repeat"`
	Next      string       `json:"next"`
	EndDate   string       `json:"end_date"`
	Remaining int          `json:"remaining"`
	Paused    bool         `json:"paused"`
}

// apply validates in against d (by building a sample transaction) and copies it onto r.
func (in input) apply(d *book.Data, ch *book.Change, r *model.Reminder) error {
	if !book.ReminderTypeOK(in.Type) {
		return fmt.Errorf("type must be expense, income or transfer")
	}
	if _, err := recur.Parse(in.Repeat); err != nil {
		return err
	}
	next, err := dates.Parse(in.Next)
	if err != nil {
		return fmt.Errorf("next date: %v", err)
	}
	if in.EndDate != "" {
		end, err := dates.Parse(in.EndDate)
		if err != nil {
			return fmt.Errorf("end date: %v", err)
		}
		if end.Before(next) {
			return fmt.Errorf("end date is before the next date")
		}
	}
	if in.Remaining < 0 {
		return fmt.Errorf("number of times cannot be negative")
	}
	sample := book.Entry{ID: "r-check", Type: in.Type, Date: in.Next, Title: in.Title, Amount: in.Amount,
		Account: in.Account, ToAccount: in.ToAccount, Category: in.Category, Status: in.Status}
	if _, err := d.BuildTx(sample); err != nil {
		return err
	}
	if r.Next != in.Next {
		r.AnchorDay = 0 // the date was moved by hand: start a new anchor
	}
	r.Title, r.Type, r.Amount = strings.TrimSpace(in.Title), in.Type, in.Amount
	r.Account, r.ToAccount, r.Category = in.Account, in.ToAccount, in.Category
	if in.Type == model.TxTransfer {
		r.Category = ""
	} else {
		r.ToAccount = ""
	}
	r.Labels = d.EnsureLabels(in.Labels, ch)
	r.Notes, r.Status, r.Repeat, r.Next = strings.TrimSpace(in.Notes), in.Status, in.Repeat, in.Next
	r.EndDate, r.Remaining, r.Paused, r.Done = in.EndDate, in.Remaining, in.Paused, false
	return nil
}

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("POST /api/reminders", create)
	rt.Handle("PUT /api/reminders/{id}", update)
	rt.Handle("DELETE /api/reminders/{id}", remove)
	rt.Handle("POST /api/reminders/{id}/skip", skip)
	rt.Handle("GET /api/upcoming", upcoming)
}

func create(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	var id string
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		var r model.Reminder
		if err := in.apply(d, ch, &r); err != nil {
			return err
		}
		r.ID = d.UniqueReminderID(r.Title)
		id = r.ID
		d.Reminders = append(d.Reminders, r)
		ch.Reminders = true
		d.PostDue(c.Book.Today(), ch) // a start date in the past back-fills right away
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]string{"id": id})
}

func update(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	id := c.R.PathValue("id")
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		for i, r := range d.Reminders {
			if r.ID != id {
				continue
			}
			if err := in.apply(d, ch, &r); err != nil {
				return err
			}
			d.Reminders[i] = r
			ch.Reminders = true
			d.PostDue(c.Book.Today(), ch)
			return nil
		}
		return httpx.NotFound("reminder")
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

// remove deletes the reminder. Transactions it already posted stay (they happened).
func remove(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		for i, r := range d.Reminders {
			if r.ID == id {
				d.Reminders = append(d.Reminders[:i:i], d.Reminders[i+1:]...)
				ch.Reminders = true
				return nil
			}
		}
		return httpx.NotFound("reminder")
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

// skip moves the reminder to its following date without posting the current one.
func skip(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		for i, r := range d.Reminders {
			if r.ID != id {
				continue
			}
			rule, err := recur.Parse(r.Repeat)
			if err != nil {
				return err
			}
			next, err := dates.Parse(r.Next)
			if err != nil {
				return err
			}
			if !rule.Repeats() {
				r.Done = true
			} else {
				next, r.AnchorDay = rule.Next(next, r.AnchorDay)
				r.Next = next.String()
				if r.EndDate != "" {
					if end, err := dates.Parse(r.EndDate); err == nil && next.After(end) {
						r.Done = true
					}
				}
				if r.Remaining > 0 {
					r.Remaining--
					if r.Remaining == 0 {
						r.Done = true
					}
				}
			}
			d.Reminders[i] = r
			ch.Reminders = true
			return nil
		}
		return httpx.NotFound("reminder")
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

// upcoming merges future-dated transactions and reminder occurrences for the next
// ?days= days (default: settings.upcoming_days), soonest first.
func upcoming(c *httpx.Ctx) error {
	var items []view.Tx
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		days := c.QInt("days", d.Settings.UpcomingDays)
		if days < 1 || days > 3660 {
			days = d.Settings.UpcomingDays
		}
		from, to := comp.Today.AddDays(1), comp.Today.AddDays(days)
		for _, t := range d.Txns {
			if td, err := dates.Parse(t.Date); err == nil && td.InRange(from, to) && comp.Valid[t.ID] {
				items = append(items, view.TxOf(t, d, comp))
			}
		}
		for _, e := range d.Upcoming(from, to) {
			if _, posted := d.Tx(e.ID); !posted {
				items = append(items, view.UpcomingOf(e, d))
			}
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].Date < items[j].Date })
		return nil
	})
	if err != nil {
		return err
	}
	if items == nil {
		items = []view.Tx{}
	}
	return c.OK(items)
}
