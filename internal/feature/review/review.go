// Package review is the Inbox feature: transactions proposed by SMS and Kronos,
// waiting for you to accept, edit, match or reject them, plus the rules Arthik
// learned from your decisions.
package review

import (
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/inbox"
	"gitlab.com/niharokz/arthik/internal/model"
)

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/inbox", list)
	rt.Handle("POST /api/inbox/{id}/accept", accept)
	rt.Handle("POST /api/inbox/{id}/match", match)
	rt.Handle("POST /api/inbox/{id}/reject", decide(inbox.Reject))
	rt.Handle("POST /api/inbox/{id}/restore", decide(inbox.Restore))
	rt.Handle("POST /api/inbox/{id}/unpair", decide(inbox.Unpair))
	rt.Handle("GET /api/inbox/rules", rules)
	rt.Handle("DELETE /api/inbox/rules/{id}", deleteRule)
}

// list: ?status=pending (default) | done | all
func list(c *httpx.Ctx) error {
	var statuses []string
	switch c.Q("status") {
	case "", "pending":
		statuses = []string{model.DraftPending}
	case "done":
		statuses = []string{model.DraftAccepted, model.DraftMatched, model.DraftRejected}
	case "all":
	default:
		return httpx.Errorf(400, "status must be pending, done or all")
	}
	var out map[string]any
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		out = map[string]any{
			"items":   inbox.Views(d, comp, statuses...),
			"pending": inbox.Pending(d),
			"status":  inbox.StatusOf(c.Book.Dir),
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

type acceptInput struct {
	book.Entry
	RememberDigits bool `json:"remember_digits"`
}

func accept(c *httpx.Ctx) error {
	var in acceptInput
	if err := c.Decode(&in); err != nil {
		return err
	}
	var id string
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		var err error
		id, err = inbox.Accept(d, ch, c.R.PathValue("id"), in.Entry, in.RememberDigits)
		return err
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]string{"id": id})
}

func match(c *httpx.Ctx) error {
	var in struct {
		Txn string `json:"txn"`
	}
	if err := c.Decode(&in); err != nil {
		return err
	}
	return done(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		return inbox.Match(d, ch, c.R.PathValue("id"), in.Txn)
	}))
}

func decide(fn func(d *book.Data, ch *book.Change, id string) error) httpx.HandlerFunc {
	return func(c *httpx.Ctx) error {
		return done(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
			return fn(d, ch, c.R.PathValue("id"))
		}))
	}
}

func rules(c *httpx.Ctx) error {
	out := []model.Rule{}
	err := c.Book.View(func(d *book.Data, _ *book.Computed) error {
		out = append(out, d.Rules...)
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

func deleteRule(c *httpx.Ctx) error {
	return done(c, c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		return inbox.DeleteRule(d, ch, c.R.PathValue("id"))
	}))
}

func done(c *httpx.Ctx, err error) error {
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}
