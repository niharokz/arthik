// Package review is the Inbox feature: transactions proposed by SMS and Kronos,
// waiting for you to accept, edit, match or reject them, plus the rules Arthik
// learned from your decisions.
package review

import (
	"sort"
	"strings"

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
	rt.Handle("POST /api/inbox/import", importNow)
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
			"items":    inbox.Views(d, comp, statuses...),
			"pending":  inbox.Pending(d),
			"status":   inbox.StatusOf(c.Book.Dir),
			"main":     inbox.Runs(c.Book.Dir),
			"phones":   inbox.Phones(c.Book.Dir),
			"sms_time": d.Settings.InboxSMSTime,
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

// importNow reads the SMS and Kronos files right away ("Check now").
func importNow(c *httpx.Ctx) error {
	added, ok, err := inbox.ImportFor(c.Book.Dir)
	if !ok {
		return httpx.Errorf(409, "this device doesn't import the inbox — your main device (nimory) does")
	}
	if err != nil {
		return err
	}
	return c.OK(map[string]any{"added": added, "last_capture": inbox.LastCapture(c.Book.Dir)})
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

// rules: ?q= (merchant, title or category) &offset= &limit= (default 50, max 200),
// most used first. Replies {items, total, offset, limit}.
func rules(c *httpx.Ctx) error {
	var out RulePage
	err := c.Book.View(func(d *book.Data, _ *book.Computed) error {
		out = PageRules(d, c.Q("q"), c.QInt("offset", 0), c.QInt("limit", 50))
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(out)
}

// RulePage is one page of learned rules.
type RulePage struct {
	Items  []model.Rule `json:"items"`
	Total  int          `json:"total"` // rules matching q
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

// PageRules filters the learned rules by q and returns one page of them.
func PageRules(d *book.Data, q string, offset, limit int) RulePage {
	limit = min(max(limit, 1), 200)
	q = strings.ToLower(strings.TrimSpace(q))
	var hits []model.Rule
	for _, r := range d.Rules {
		if q == "" || ruleHas(d, r, q) {
			hits = append(hits, r)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Uses != b.Uses {
			return a.Uses > b.Uses
		}
		if a.Updated != b.Updated {
			return a.Updated > b.Updated
		}
		return a.Match < b.Match
	})
	offset = min(max(offset, 0), len(hits))
	end := min(offset+limit, len(hits))
	return RulePage{Items: append([]model.Rule{}, hits[offset:end]...), Total: len(hits), Offset: offset, Limit: limit}
}

func ruleHas(d *book.Data, r model.Rule, q string) bool {
	cat, _ := d.Category(r.Category)
	for _, s := range []string{r.Match, r.Title, cat.Name} {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
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
