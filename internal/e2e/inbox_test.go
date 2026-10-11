package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/config"
	"gitlab.com/niharokz/arthik/internal/inbox"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

var inboxCfg = config.Inbox{MaxAgeDays: 30, KeepDays: 120, PairDays: 2}

// writeFile writes a producer file the way the phone app or Kronos would.
func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func drafts(b *book.Book) map[string]model.Draft {
	out := map[string]model.Draft{}
	_ = b.View(func(d *book.Data, _ *book.Computed) error {
		for _, x := range d.Drafts {
			out[x.ID] = x
		}
		return nil
	})
	return out
}

// SMS captured on the phone and a Kronos subscription become drafts on nimory;
// the phone accepts one while offline as nimory imports another; after syncing,
// both decisions survive, the transaction exists once and the rule was learned.
func TestInboxAcrossDevices(t *testing.T) {
	m, main, phone := setup(t)
	err := main.Mutate(func(d *book.Data, ch *book.Change) error {
		d.Accounts = append(d.Accounts, model.Account{ID: "card", Name: "HDFC Regalia Gold CC", Type: "credit_card", OpeningBalance: money.FromRupees(-5000), SMSDigits: []string{"4275"}})
		d.Categories = append(d.Categories, model.Category{ID: "utility", Name: "Utilities", Kind: model.KindExpense})
		ch.Accounts, ch.Categories = true, true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mainRun := &inbox.Runner{Book: main, Cfg: inboxCfg, Loc: time.UTC}
	phoneRun := &inbox.Runner{Book: phone, Cfg: inboxCfg, Loc: time.UTC}

	writeFile(t, m.dirs[1], "inbox/sms_phone.md", `device: phone
messages:
  - {id: "1111111111111111", from: "AX-HDFCBK", at: "2026-10-05T20:34:40Z", body: "Spent Rs.2000 On HDFC Bank Card 4275 At GYFTR VIA SMARTBUY On 2026-10-05:20:34:36.Not You? Call"}
  - {id: "2222222222222222", from: "AX-HDFCBK", at: "2026-10-05T21:00:00Z", body: "Your OTP is 123456 for txn of Rs 99"}
`)
	settle(t, m, main, phone)
	phoneRun.Tick(ctx) // a replica never imports
	if n := len(drafts(phone)); n != 0 {
		t.Fatalf("phone imported %d drafts", n)
	}
	mainRun.Tick(ctx)
	settle(t, m, main, phone)
	got := drafts(phone)
	gyftr, ok := got["sms-1111111111111111"]
	if len(got) != 1 || !ok || gyftr.Account != "card" || gyftr.Amount != money.FromRupees(2000) || gyftr.Date != "2026-10-05" {
		t.Fatalf("drafts on phone: %+v", got)
	}

	// --- offline window ---
	err = phone.Mutate(func(d *book.Data, ch *book.Change) error {
		_, err := inbox.Accept(d, ch, gyftr.ID, book.Entry{Category: "food", Title: "Amazon gift card"}, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	writeFile(t, m.dirs[0], "inbox/kronos.md", `updated: 2026-10-06 02:12:00

drafts:
  - id: sub-elcty-bangalore-2026-10-06
    source: subscription
    date: '2026-10-06'
    title: Elcty (Bangalore)
    amount: 800.0
    account: HDFC Regalia Gold CC
    category: utility
`)
	mainRun.Tick(ctx)
	touchAll(m.dirs[1], time.Now().Add(time.Minute)) // phone saved last
	if c := m.sync(t); len(c) == 0 {
		t.Fatal("expected a conflict copy of drafts.md")
	}
	settle(t, m, main, phone)

	for _, b := range []*book.Book{main, phone} {
		got := drafts(b)
		if got["sms-1111111111111111"].Status != model.DraftAccepted || got["kronos-sub-elcty-bangalore-2026-10-06"].Status != model.DraftPending {
			t.Fatalf("%s drafts: %+v", b.Name, got)
		}
		if k := got["kronos-sub-elcty-bangalore-2026-10-06"]; k.Account != "card" || k.Category != "utility" || k.Via != model.ViaSource {
			t.Fatalf("%s kronos draft: %+v", b.Name, k)
		}
		n := 0
		for _, s := range snapshot(b) {
			if strings.HasPrefix(s, "d-sms-1111111111111111 Amazon gift card 2000.00") {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%s: accepted transaction appears %d times: %v", b.Name, n, snapshot(b))
		}
		var rule model.Rule
		_ = b.View(func(d *book.Data, _ *book.Computed) error { rule, _ = d.Rule("gyftr via smartbuy"); return nil })
		if rule.Category != "food" || rule.Title != "Amazon gift card" {
			t.Fatalf("%s did not learn: %+v", b.Name, rule)
		}
	}
	for _, d := range m.dirs {
		if c := conflictFiles(d); len(c) != 0 {
			t.Fatalf("conflict copies left in %s: %v", d, c)
		}
	}

	// The next GYFTR message fills itself in from what was learned.
	writeFile(t, m.dirs[1], "inbox/sms_phone.md", `device: phone
messages:
  - {id: "3333333333333333", from: "AX-HDFCBK", at: "2026-10-06T10:00:00Z", body: "Spent Rs.500 On HDFC Bank Card 4275 At GYFTR VIA SMARTBUY On 2026-10-06:10:00:00."}
`)
	settle(t, m, main, phone)
	mainRun.Tick(ctx)
	settle(t, m, main, phone)
	if x := drafts(phone)["sms-3333333333333333"]; x.Category != "food" || x.Title != "Amazon gift card" || x.Via != model.ViaLearned {
		t.Fatalf("learned rule not applied: %+v", x)
	}
}
