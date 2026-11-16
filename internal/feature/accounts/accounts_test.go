package accounts

import (
	"testing"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/model"
)

// Deleting an account takes it off open drafts, reminders and learned rules;
// reviewed drafts keep what they recorded.
func TestForget(t *testing.T) {
	b, err := book.Open(t.TempDir(), "test", false)
	if err != nil {
		t.Fatal(err)
	}
	err = b.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = []model.Account{{ID: "old", Name: "Old", Type: "bank"}, {ID: "keep", Name: "Keep", Type: "bank"}}
		d.Drafts = []model.Draft{
			{ID: "a", Status: model.DraftPending, Account: "old"},
			{ID: "b", Status: model.DraftPending, Account: "keep", ToAccount: "old"},
			{ID: "c", Status: model.DraftNotice, Account: "old"},
			{ID: "d", Status: model.DraftRejected, Account: "old"},
		}
		d.Rules = []model.Rule{{ID: "r1", Match: "tea", Account: "old"}, {ID: "r2", Match: "rent", Account: "keep"}}
		d.Reindex()
		ch.Accounts, ch.Drafts, ch.Rules = true, true, true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = b.Mutate(func(d *book.Data, ch *book.Change) error { forget(d, ch, "old"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	_ = b.View(func(d *book.Data, _ *book.Computed) error {
		want := map[string][2]string{"a": {"", ""}, "b": {"keep", ""}, "c": {"", ""}, "d": {"old", ""}}
		for _, x := range d.Drafts {
			if got := [2]string{x.Account, x.ToAccount}; got != want[x.ID] {
				t.Errorf("draft %s: accounts %v, want %v", x.ID, got, want[x.ID])
			}
		}
		for _, r := range d.Rules {
			if (r.ID == "r1" && r.Account != "") || (r.ID == "r2" && r.Account != "keep") {
				t.Errorf("rule %s: account %q", r.ID, r.Account)
			}
		}
		return nil
	})
}
