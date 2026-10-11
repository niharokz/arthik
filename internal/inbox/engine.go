package inbox

import (
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

// Options bound what an import does.
type Options struct {
	Today    dates.Date
	Now      time.Time
	MaxAge   int // days: older source items are ignored
	KeepDays int // days: reviewed drafts are forgotten after this
	PairDays int // days: a debit and a matching credit this close become one transfer
}

// Ingest adds a draft for every source item not seen before, pairs transfers and
// forgets old reviewed drafts. It returns how many drafts were added.
func Ingest(d *book.Data, ch *book.Change, items []Item, o Options) int {
	oldest := o.Today.AddDays(-o.MaxAge)
	added := 0
	for _, it := range items {
		if _, seen := d.Draft(it.ID); seen {
			continue
		}
		if _, done := d.Tx(TxID(it.ID)); done {
			continue // accepted long ago, draft already forgotten
		}
		day, err := dates.Parse(it.Date)
		if err != nil || day.Before(oldest) || day.After(o.Today.AddDays(1)) {
			continue
		}
		d.SetDraft(newDraft(d, it), ch)
		added++
	}
	pair(d, ch, o.PairDays)
	purge(d, ch, o.Now, o.KeepDays)
	return added
}

// TxID is the transaction id an accepted draft becomes (so accepting twice, on two
// devices, still makes one transaction).
func TxID(draftID string) string { return "d-" + draftID }

func newDraft(d *book.Data, it Item) model.Draft {
	x := model.Draft{
		ID: it.ID, Source: it.Source, Status: model.DraftPending, Date: it.Date, Time: it.Time,
		Amount: it.Amount, Merchant: it.Merchant, Digits: it.Digits, AccountHint: it.AccountHint,
		CategoryHint: it.CategoryHint, Labels: it.Labels, Notes: it.Notes, Raw: it.Raw, Rule: it.Rule,
		Balance: it.Balance, Available: it.Available,
	}
	if it.Generic {
		x.Rule = "generic"
	}
	acc, _ := d.AccountByDigits(it.Digits)
	if acc == "" && it.AccountHint != "" {
		acc = accountByName(d, it.AccountHint)
	}
	switch it.Kind {
	case KindTransferIn:
		x.Type, x.ToAccount = model.TxTransfer, acc
		x.Title = "Card payment"
		return x
	case KindIncome:
		x.Type, x.Account = model.TxIncome, acc
	default:
		x.Type, x.Account = model.TxExpense, acc
	}
	x.Title = it.Title
	Categorise(d, &x)
	return x
}

// Categorise fills title, category (and, from a learned rule, account and labels)
// in priority order: learned rule → a past transaction with the same name → the
// category the source named → the fallback category in settings.
func Categorise(d *book.Data, x *model.Draft) {
	if x.Type != model.TxExpense && x.Type != model.TxIncome {
		return
	}
	x.Category, x.Via = "", ""
	key := Key(x.Merchant)
	if r, ok := d.Rule(key); ok && key != "" {
		if r.Title != "" {
			x.Title = r.Title
		}
		if categoryFits(d, r.Category, x.Type) {
			x.Category, x.Via = r.Category, model.ViaLearned
		}
		if x.Account == "" && x.Digits == "" && validAccount(d, r.Account) {
			x.Account = r.Account
		}
		if len(x.Labels) == 0 {
			x.Labels = r.Labels
		}
	}
	if x.Category == "" && key != "" {
		if t, ok := pastMatch(d, key, x.Type); ok {
			x.Category, x.Via = t.Category, model.ViaPast
			if x.Title == "" {
				x.Title = t.Title
			}
		}
	}
	if x.Category == "" && x.CategoryHint != "" {
		if id := categoryByName(d, x.CategoryHint, x.Type); id != "" {
			x.Category, x.Via = id, model.ViaSource
		}
	}
	if x.Category == "" {
		def := d.Settings.InboxExpenseCategory
		if x.Type == model.TxIncome {
			def = d.Settings.InboxIncomeCategory
		}
		if categoryFits(d, def, x.Type) {
			x.Category, x.Via = def, model.ViaDefault
		}
	}
	if x.Title == "" {
		x.Title = fallbackTitle(d, x)
	}
}

func fallbackTitle(d *book.Data, x *model.Draft) string {
	if x.Merchant != "" {
		return Pretty(x.Merchant)
	}
	name := "account …" + x.Digits
	if a, ok := d.Account(x.Account); ok {
		name = a.Name
	}
	if x.Type == model.TxIncome {
		return "Received in " + name
	}
	return "Paid from " + name
}

// pastMatch finds the most recent expense/income you entered whose title matches key.
func pastMatch(d *book.Data, key, typ string) (book.Entry, bool) {
	for i := len(d.Txns) - 1; i >= 0; i-- {
		t := d.Txns[i]
		if t.Type != typ || strings.HasPrefix(t.ID, "d-") {
			continue // drafts you accepted are covered by learned rules
		}
		k := Key(t.Title)
		if k == "" || !(k == key || len(k) >= 4 && strings.Contains(key, k) || len(key) >= 4 && strings.Contains(k, key)) {
			continue
		}
		e := book.EntryOf(t)
		if e.Category != "" && categoryFits(d, e.Category, typ) {
			return e, true
		}
	}
	return book.Entry{}, false
}

func validAccount(d *book.Data, id string) bool {
	_, ok := d.Account(id)
	return id != "" && ok
}

func categoryFits(d *book.Data, id, typ string) bool {
	c, ok := d.Category(id)
	if id == "" || !ok {
		return false
	}
	return c.Kind == model.KindBoth || c.Kind == typ
}

// accountByName matches a producer's account name to an account id or name.
func accountByName(d *book.Data, name string) string {
	for _, a := range d.Accounts {
		if strings.EqualFold(a.ID, name) || strings.EqualFold(a.Name, name) {
			return a.ID
		}
	}
	return ""
}

// categoryByName matches "utility", "tech/software" or a category id to a category
// of the right kind (the last path part wins: "tech/software" → "Software").
func categoryByName(d *book.Data, name, typ string) string {
	parts := strings.Split(name, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		want := strings.TrimSpace(parts[i])
		for _, c := range d.Categories {
			if (strings.EqualFold(c.ID, want) || strings.EqualFold(c.Name, want) || Key(c.Name) == Key(want)) && categoryFits(d, c.ID, typ) {
				return c.ID
			}
		}
	}
	return ""
}

// pair turns a debit from one of your accounts and a matching credit on another of
// your accounts (a card payment, a transfer between your banks) into one transfer.
func pair(d *book.Data, ch *book.Change, days int) {
	isPending := func(x model.Draft) bool {
		return x.Status == model.DraftPending && !x.Edited && x.Pair == "" && x.Source == SourceSMS
	}
	for _, out := range d.Drafts {
		if !isPending(out) || out.Type != model.TxExpense || out.Account == "" {
			continue
		}
		if a, _ := d.Account(out.Account); a.Class() != model.ClassAsset {
			continue // money leaving a card is spending, not a transfer
		}
		od, err := dates.Parse(out.Date)
		if err != nil {
			continue
		}
		for _, in := range d.Drafts {
			if !isPending(in) || in.ID == out.ID || in.Amount != out.Amount {
				continue
			}
			to := ""
			switch {
			case in.Type == model.TxTransfer && in.Account == "" && in.ToAccount != "":
				to = in.ToAccount // card/loan payment received
			case in.Type == model.TxIncome && in.Account != "" && in.Account != out.Account:
				to = in.Account // arrived in another of your accounts
			}
			if to == "" || to == out.Account {
				continue
			}
			id, err := dates.Parse(in.Date)
			if err != nil || abs(od.DaysUntil(id)) > days {
				continue
			}
			name := to
			if a, ok := d.Account(to); ok {
				name = a.Name
			}
			merged := out
			merged.Type, merged.ToAccount, merged.Category, merged.Via, merged.Pair = model.TxTransfer, to, "", "", in.ID
			merged.Title = "Transfer to " + name
			if in.Type == model.TxTransfer {
				merged.Title = "Payment to " + name
			}
			in.Status, in.Pair = model.DraftMerged, out.ID
			d.SetDraft(merged, ch)
			d.SetDraft(in, ch)
			break
		}
	}
}

// Unpair splits a paired transfer back into its two drafts.
func Unpair(d *book.Data, ch *book.Change, id string) error {
	x, ok := d.Draft(id)
	if !ok {
		return errNotFound
	}
	if x.Status == model.DraftMerged {
		if x, ok = d.Draft(x.Pair); !ok {
			return errNotFound
		}
	}
	if x.Pair == "" || x.Status != model.DraftPending {
		return errf("this draft is not a paired transfer")
	}
	other, ok := d.Draft(x.Pair)
	if ok && other.Status == model.DraftMerged {
		other.Status, other.Pair = model.DraftPending, ""
		other.Edited = true // keep the two apart from now on
		d.SetDraft(other, ch)
	}
	x.Type, x.ToAccount, x.Pair, x.Title, x.Edited = model.TxExpense, "", "", "", true
	Categorise(d, &x)
	d.SetDraft(x, ch)
	return nil
}

// purge forgets reviewed drafts older than keep days (pending ones are kept).
func purge(d *book.Data, ch *book.Change, now time.Time, keep int) {
	if keep <= 0 {
		return
	}
	cut := now.Add(-time.Duration(keep) * 24 * time.Hour)
	kept := d.Drafts[:0:0]
	for _, x := range d.Drafts {
		if x.Status != model.DraftPending {
			if t, err := time.Parse(time.RFC3339, x.Updated); err == nil && t.Before(cut) {
				ch.Drafts = true
				continue
			}
		}
		kept = append(kept, x)
	}
	if ch.Drafts {
		d.Drafts = kept
		d.Reindex()
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// signed is the effect a draft has on its account (+ in, − out).
func signed(x model.Draft) money.Amount {
	if x.Type == model.TxIncome {
		return x.Amount
	}
	return -x.Amount
}
