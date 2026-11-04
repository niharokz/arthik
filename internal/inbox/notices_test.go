package inbox

import (
	"testing"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

func notices(t *testing.T, b *book.Book) (list []Notice, active int) {
	t.Helper()
	_ = b.View(func(d *book.Data, c *book.Computed) error {
		list, active = Notices(d, c, false), ActiveNotices(d, c)
		return nil
	})
	return
}

// Kronos bills remind; the bank SMS is what becomes the transaction.
func TestKronosIsANotice(t *testing.T) {
	b := newBook(t)
	rent := Item{ID: "kronos-rent", Source: SourceKronos, Date: "2026-10-10", Kind: KindExpense, Amount: money.FromRupees(14000), Title: "Rent"}
	ingest(t, b, rent, spend("sms-1", "2026-10-10", 14000, "9001", "RABINARAYAN"))
	if x := draft(t, b, "kronos-rent"); x.Status != model.DraftNotice {
		t.Fatalf("kronos draft status = %q", x.Status)
	}
	list, active := notices(t, b)
	if len(list) != 1 || active != 1 || list[0].Proof == nil || list[0].Proof.ID != "sms-1" {
		t.Fatalf("notices %+v active %d", list, active)
	}
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		_, err := Accept(d, ch, "sms-1", book.Entry{Category: "misc", Title: "Rent"}, false)
		return err
	})
	if list, active = notices(t, b); active != 0 || list[0].Paid == nil {
		t.Fatalf("after the SMS is accepted the bill is paid: %+v active %d", list, active)
	}
	mutate(t, b, func(d *book.Data, ch *book.Change) error { return Dismiss(d, ch, "kronos-rent") })
	if list, _ = notices(t, b); len(list) != 0 {
		t.Fatalf("dismissed notice still listed: %+v", list)
	}
}

// Kronos drafts left pending by an older version become notices on the next import.
func TestOldKronosDraftsBecomeNotices(t *testing.T) {
	b := newBook(t)
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		d.SetDraft(model.Draft{ID: "kronos-old", Source: SourceKronos, Status: model.DraftPending, Date: "2026-10-09", Type: model.TxExpense, Amount: 100, Title: "Water"}, ch)
		return nil
	})
	ingest(t, b)
	if x := draft(t, b, "kronos-old"); x.Status != model.DraftNotice {
		t.Fatalf("status = %q", x.Status)
	}
}
