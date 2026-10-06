// Package labels is the Labels feature (free tags on transactions, as in Bluecoins):
// create, rename/recolour, delete (removed from every transaction) and a label report.
package labels

import (
	"fmt"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
)

type input struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("POST /api/labels", create)
	rt.Handle("PUT /api/labels/{id}", update)
	rt.Handle("DELETE /api/labels/{id}", remove)
}

func create(c *httpx.Ctx) error {
	var in input
	if err := c.Decode(&in); err != nil {
		return err
	}
	var id string
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return fmt.Errorf("enter a label name")
		}
		for _, l := range d.Labels {
			if strings.EqualFold(l.Name, name) {
				return fmt.Errorf("label %q already exists", name)
			}
		}
		id = d.UniqueLabelID(name)
		d.Labels = append(d.Labels, model.Label{ID: id, Name: name, Color: strings.TrimSpace(in.Color)})
		ch.Labels = true
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
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return fmt.Errorf("enter a label name")
		}
		for i, l := range d.Labels {
			if l.ID == id {
				l.Name, l.Color = name, strings.TrimSpace(in.Color)
				d.Labels[i] = l
				ch.Labels = true
				return nil
			}
		}
		return httpx.NotFound("label")
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

// remove deletes the label and strips it from every transaction and reminder.
func remove(c *httpx.Ctx) error {
	id := c.R.PathValue("id")
	err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		found := false
		for i, l := range d.Labels {
			if l.ID == id {
				d.Labels = append(d.Labels[:i:i], d.Labels[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return httpx.NotFound("label")
		}
		ch.Labels = true
		for _, t := range d.Txns {
			if t.HasLabel(id) {
				t.Labels = without(t.Labels, id)
				d.SetTx(t, ch)
			}
		}
		for i, r := range d.Reminders {
			if len(r.Labels) > 0 {
				if n := without(r.Labels, id); len(n) != len(r.Labels) {
					r.Labels = n
					d.Reminders[i] = r
					ch.Reminders = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.OK(map[string]bool{"ok": true})
}

func without(s []string, v string) []string {
	var out []string
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
