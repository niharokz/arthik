package inbox

import (
	"errors"
	"fmt"
	"net/http"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
)

var errNotFound = httpx.NotFound("draft")

func errf(format string, a ...any) error { return httpx.Errorf(http.StatusConflict, format, a...) }

// pendingDraft returns a draft that can still be decided.
func pendingDraft(d *book.Data, id string) (model.Draft, error) {
	x, ok := d.Draft(id)
	if !ok {
		return x, errNotFound
	}
	if x.Status != model.DraftPending {
		return x, errf("this draft was already %s", x.Status)
	}
	return x, nil
}

// Accept turns a draft into a transaction. e holds the final values from the
// review form; empty fields fall back to the draft. rememberDigits stores the SMS
// digits on the chosen account so the next message finds it by itself.
func Accept(d *book.Data, ch *book.Change, id string, e book.Entry, rememberDigits bool) (string, error) {
	x, err := pendingDraft(d, id)
	if err != nil {
		return "", err
	}
	fill(&e, x)
	e.ID = TxID(x.ID)
	if _, exists := d.Tx(e.ID); !exists { // accepted on another device already
		t, err := d.BuildTx(e)
		if err != nil {
			return "", err
		}
		t.Labels = d.EnsureLabels(t.Labels, ch)
		d.SetTx(t, ch)
		e.Labels = t.Labels
	}
	if rememberDigits && x.Digits != "" {
		target := e.Account
		if x.Type == model.TxTransfer && x.Account == "" {
			target = e.ToAccount // a card payment: the digits are the card's
		}
		if err := rememberAccountDigits(d, ch, target, x.Digits); err != nil {
			return "", err
		}
	}
	learn(d, ch, x, e)
	settle(d, ch, x, e, model.DraftAccepted, e.ID)
	return e.ID, nil
}

// Match links a draft to a transaction you already entered (nothing is created).
func Match(d *book.Data, ch *book.Change, id, txID string) error {
	x, err := pendingDraft(d, id)
	if err != nil {
		return err
	}
	t, ok := d.Tx(txID)
	if !ok {
		return httpx.NotFound("transaction")
	}
	e := book.EntryOf(t)
	learn(d, ch, x, e)
	settle(d, ch, x, e, model.DraftMatched, txID)
	return nil
}

// Reject sets a draft (and its paired half) aside.
func Reject(d *book.Data, ch *book.Change, id string) error {
	x, err := pendingDraft(d, id)
	if err != nil {
		return err
	}
	settle(d, ch, x, book.Entry{}, model.DraftRejected, "")
	return nil
}

// Restore brings a rejected draft back for review.
func Restore(d *book.Data, ch *book.Change, id string) error {
	x, ok := d.Draft(id)
	if !ok {
		return errNotFound
	}
	if x.Status != model.DraftRejected {
		return errf("only rejected drafts can be restored")
	}
	x.Status = model.DraftPending
	d.SetDraft(x, ch)
	if p, ok := d.Draft(x.Pair); ok && p.Status == model.DraftRejected {
		p.Status = model.DraftMerged
		d.SetDraft(p, ch)
	}
	return nil
}

// fill completes a review entry with the draft's values.
func fill(e *book.Entry, x model.Draft) {
	if e.Type == "" {
		e.Type = x.Type
	}
	if e.Date == "" {
		e.Date, e.Time = x.Date, x.Time
	}
	if e.Title == "" {
		e.Title = x.Title
	}
	if e.Amount == 0 && len(e.Splits) == 0 {
		e.Amount = x.Amount
	}
	if e.Account == "" {
		e.Account = x.Account
	}
	if e.ToAccount == "" {
		e.ToAccount = x.ToAccount
	}
	if e.Category == "" && len(e.Splits) == 0 {
		e.Category = x.Category
	}
	if e.Labels == nil {
		e.Labels = x.Labels
	}
	if e.Notes == "" {
		e.Notes = x.Notes
	}
}

// settle records the decision on the draft and on its paired half.
func settle(d *book.Data, ch *book.Change, x model.Draft, e book.Entry, status, txID string) {
	if status != model.DraftRejected {
		x.Type, x.Date, x.Time, x.Title, x.Amount = e.Type, e.Date, e.Time, e.Title, e.Amount
		x.Account, x.ToAccount, x.Category, x.Labels, x.Notes = e.Account, e.ToAccount, e.Category, e.Labels, e.Notes
	}
	x.Status, x.Txn, x.Edited = status, txID, true
	d.SetDraft(x, ch)
	if p, ok := d.Draft(x.Pair); ok && p.Status == model.DraftMerged {
		p.Status, p.Txn = status, txID
		if status == model.DraftRejected {
			p.Status = model.DraftRejected
		}
		d.SetDraft(p, ch)
	}
}

// learn remembers what this merchant became, so the next draft fills itself in.
func learn(d *book.Data, ch *book.Change, x model.Draft, e book.Entry) {
	key := Key(x.Merchant)
	if key == "" || (e.Type != model.TxExpense && e.Type != model.TxIncome) || e.Category == "" {
		return
	}
	r, _ := d.Rule(key)
	r.ID, r.Match = book.Slug(key), key
	r.Title, r.Category, r.Labels = e.Title, e.Category, e.Labels
	r.Account = ""
	if x.Digits == "" { // the source names no account: remember the one you chose
		r.Account = e.Account
	}
	r.Uses++
	d.SetRule(r, ch)
}

// rememberAccountDigits moves digits onto account (taking them off any other).
func rememberAccountDigits(d *book.Data, ch *book.Change, account, digits string) error {
	if _, ok := d.Account(account); !ok {
		return errors.New("choose the account these SMS digits belong to")
	}
	if len(digits) > 4 {
		digits = digits[len(digits)-4:]
	}
	for i, a := range d.Accounts {
		var keep []string
		for _, x := range a.SMSDigits {
			if x != digits {
				keep = append(keep, x)
			}
		}
		if a.ID == account {
			keep = append(keep, digits)
		}
		if len(keep) != len(a.SMSDigits) {
			a.SMSDigits = keep
			d.Accounts[i] = a
			ch.Accounts = true
		}
	}
	return nil
}

// DeleteRule forgets a learned rule.
func DeleteRule(d *book.Data, ch *book.Change, id string) error {
	if !d.DeleteRule(id, ch) {
		return fmt.Errorf("rule not found")
	}
	return nil
}
