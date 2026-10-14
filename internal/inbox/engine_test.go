package inbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/ai"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/config"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

const today = "2026-10-11"

var opts = Options{Today: dates.MustParse(today), Now: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), MaxAge: 30, KeepDays: 120, PairDays: 2}

func newBook(t *testing.T) *book.Book {
	t.Helper()
	b, err := book.Open(t.TempDir(), "test", false)
	if err != nil {
		t.Fatal(err)
	}
	b.Today = func() dates.Date { return dates.MustParse(today) }
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		d.Accounts = []model.Account{
			{ID: "idfc", Name: "IDFC Select Savings", Type: "bank", OpeningBalance: money.FromRupees(10000), SMSDigits: []string{"0353"}},
			{ID: "hdfc", Name: "HDFC Regalia Gold CC", Type: "credit_card", OpeningBalance: money.FromRupees(-1000), CreditLimit: money.FromRupees(50000), SMSDigits: []string{"4275"}},
			{ID: "cash", Name: "Cash", Type: "cash"},
		}
		d.Categories = []model.Category{
			{ID: "food", Name: "Food", Kind: model.KindExpense},
			{ID: "gifts", Name: "Gifts", Kind: model.KindExpense},
			{ID: "utility", Name: "Utilities", Kind: model.KindExpense},
			{ID: "misc", Name: "Miscellaneous", Kind: model.KindExpense},
			{ID: "salary", Name: "Salary", Kind: model.KindIncome},
		}
		d.Settings.InboxExpenseCategory = "misc"
		ch.Accounts, ch.Categories, ch.Settings = true, true, true
		return nil
	})
	return b
}

func mutate(t *testing.T, b *book.Book, fn func(d *book.Data, ch *book.Change) error) {
	t.Helper()
	if err := b.Mutate(fn); err != nil {
		t.Fatal(err)
	}
}

func draft(t *testing.T, b *book.Book, id string) model.Draft {
	t.Helper()
	var x model.Draft
	_ = b.View(func(d *book.Data, _ *book.Computed) error { x, _ = d.Draft(id); return nil })
	return x
}

func ingest(t *testing.T, b *book.Book, items ...Item) int {
	t.Helper()
	n := 0
	mutate(t, b, func(d *book.Data, ch *book.Change) error { n = Ingest(d, ch, items, opts); return nil })
	return n
}

func spend(id, date string, rupees int64, digits, merchant string) Item {
	return Item{ID: id, Source: SourceSMS, Date: date, Kind: KindExpense, Amount: money.FromRupees(rupees), Digits: digits, Merchant: merchant}
}

func addTx(t *testing.T, b *book.Book, e book.Entry) {
	t.Helper()
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		if e.ID == "" {
			e.ID = d.NewTxID(e.Date)
		}
		tx, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		d.SetTx(tx, ch)
		return nil
	})
}

// learned rule → your past transaction → the source's category → the fallback.
func TestCategoryPriority(t *testing.T) {
	b := newBook(t)
	addTx(t, b, book.Entry{Type: model.TxExpense, Date: "2026-10-01", Title: "Swiggy", Amount: 200, Account: "cash", Category: "food"})
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		d.SetRule(model.Rule{ID: "gyftr-via-smartbuy", Match: "gyftr via smartbuy", Title: "Amazon gift card", Category: "gifts"}, ch)
		return nil
	})
	ingest(t, b,
		spend("a", "2026-10-09", 2000, "4275", "GYFTR VIA SMARTBUY"),
		spend("b", "2026-10-09", 131, "6338", "SWIGGYPVTLTDFOOD1"),
		Item{ID: "c", Source: SourceKronos, Date: today, Kind: KindExpense, Amount: money.MustParse("800"), Title: "Elcty (Bangalore)", Merchant: "Elcty (Bangalore)", AccountHint: "idfc select savings", CategoryHint: "household/utilities"},
		spend("d", "2026-10-10", 20, "3334", "SANJAY KUMAR ROUT"),
	)
	for id, want := range map[string]struct{ cat, via, title, acc string }{
		"a": {"gifts", model.ViaLearned, "Amazon gift card", "hdfc"},
		"b": {"food", model.ViaPast, "Swiggy", ""},
		"c": {"utility", model.ViaSource, "Elcty (Bangalore)", "idfc"},
		"d": {"misc", model.ViaDefault, "Sanjay Kumar Rout", ""},
	} {
		x := draft(t, b, id)
		if x.Category != want.cat || x.Via != want.via || x.Title != want.title || x.Account != want.acc {
			t.Errorf("%s: got cat=%s via=%s title=%q acc=%s, want %+v", id, x.Category, x.Via, x.Title, x.Account, want)
		}
	}
}

func TestIngestOnceAndOnlyRecent(t *testing.T) {
	b := newBook(t)
	if n := ingest(t, b, spend("a", "2026-10-09", 10, "4275", "X"), spend("old", "2026-08-01", 10, "4275", "X")); n != 1 {
		t.Fatalf("added %d, want 1 (the old one is past max age)", n)
	}
	if n := ingest(t, b, spend("a", "2026-10-09", 10, "4275", "X")); n != 0 {
		t.Fatalf("re-import added %d", n)
	}
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		_, err := Accept(d, ch, "a", book.Entry{Category: "food"}, false)
		return err
	})
	// Even after the draft is forgotten, its transaction stops a re-import.
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		purge(d, ch, opts.Now.Add(200*24*time.Hour), 120)
		return nil
	})
	if x := draft(t, b, "a"); x.ID != "" {
		t.Fatal("old reviewed draft not purged")
	}
	if n := ingest(t, b, spend("a", "2026-10-09", 10, "4275", "X")); n != 0 {
		t.Fatal("accepted draft came back after purge")
	}
}

// A debit from savings and a card "payment received" become one transfer.
func TestPairAndUnpair(t *testing.T) {
	b := newBook(t)
	ingest(t, b,
		spend("out", "2026-10-09", 18450, "0353", ""),
		Item{ID: "in", Source: SourceSMS, Date: "2026-10-10", Kind: KindTransferIn, Amount: money.FromRupees(18450), Digits: "4275"},
		spend("other", "2026-10-09", 18450, "4275", "SHOP"), // spending on the card is never a transfer source
	)
	out, in := draft(t, b, "out"), draft(t, b, "in")
	if out.Type != model.TxTransfer || out.Account != "idfc" || out.ToAccount != "hdfc" || out.Pair != "in" || in.Status != model.DraftMerged {
		t.Fatalf("not paired: out=%+v in=%+v", out, in)
	}
	if x := draft(t, b, "other"); x.Type != model.TxExpense || x.Pair != "" {
		t.Fatalf("card spend paired: %+v", x)
	}
	mutate(t, b, func(d *book.Data, ch *book.Change) error { return Unpair(d, ch, "in") })
	out, in = draft(t, b, "out"), draft(t, b, "in")
	if out.Type != model.TxExpense || out.Pair != "" || in.Status != model.DraftPending || in.Type != model.TxTransfer {
		t.Fatalf("not unpaired: out=%+v in=%+v", out, in)
	}
	ingest(t, b) // must not pair them again
	if draft(t, b, "out").Pair != "" {
		t.Fatal("re-paired after unpair")
	}
}

// Accepting creates one transaction, learns the merchant and remembers digits.
func TestAcceptLearnsAndIsIdempotent(t *testing.T) {
	b := newBook(t)
	ingest(t, b, spend("g", "2026-10-09", 2000, "9911", "GYFTR VIA SMARTBUY"))
	var id string
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		var err error
		id, err = Accept(d, ch, "g", book.Entry{Account: "hdfc", Category: "food", Title: "Amazon gift card", Labels: []string{"gift"}}, true)
		return err
	})
	_ = b.View(func(d *book.Data, c *book.Computed) error {
		tx, ok := d.Tx(id)
		if !ok || tx.Title != "Amazon gift card" || tx.Date != "2026-10-09" || !c.Valid[id] {
			t.Fatalf("transaction %+v ok=%v", tx, ok)
		}
		r, _ := d.Rule("gyftr via smartbuy")
		if r.Category != "food" || r.Title != "Amazon gift card" || r.Account != "" || r.Uses != 1 {
			t.Fatalf("rule %+v", r)
		}
		if acc, _ := d.AccountByDigits("9911"); acc != "hdfc" {
			t.Fatalf("digits not remembered (got %q)", acc)
		}
		return nil
	})
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		if _, err := Accept(d, ch, "g", book.Entry{}, false); err == nil {
			t.Error("accepted twice")
		}
		return nil
	})
	// The next message from the same merchant is filled in, account from the digits.
	ingest(t, b, spend("g2", today, 500, "9911", "GYFTR VIA SMARTBUY"))
	if x := draft(t, b, "g2"); x.Via != model.ViaLearned || x.Category != "food" || x.Account != "hdfc" || len(x.Labels) != 1 {
		t.Fatalf("learned draft %+v", x)
	}
}

// Accepted on the phone and nimory: the second accept finds the transaction.
func TestAcceptWhenTransactionExists(t *testing.T) {
	b := newBook(t)
	ingest(t, b, spend("s", "2026-10-09", 131, "4275", "SWIGGY"))
	addTx(t, b, book.Entry{ID: TxID("s"), Type: model.TxExpense, Date: "2026-10-09", Title: "Swiggy", Amount: money.FromRupees(131), Account: "hdfc", Category: "food"})
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		_, err := Accept(d, ch, "s", book.Entry{Category: "food"}, false)
		return err
	})
	_ = b.View(func(d *book.Data, _ *book.Computed) error {
		if len(d.Txns) != 1 {
			t.Fatalf("%d transactions", len(d.Txns))
		}
		return nil
	})
}

func TestMatchRejectRestore(t *testing.T) {
	b := newBook(t)
	addTx(t, b, book.Entry{Type: model.TxExpense, Date: "2026-10-08", Title: "Dinner", Amount: money.FromRupees(486), Account: "hdfc", Category: "food"})
	ingest(t, b, spend("m", "2026-10-09", 486, "4275", "ZOMATO"), spend("r", "2026-10-09", 99, "4275", "SPAM"))
	var v []View
	_ = b.View(func(d *book.Data, c *book.Computed) error { v = Views(d, c, model.DraftPending); return nil })
	var same *Same
	for _, x := range v {
		if x.ID == "m" {
			same = x.Same
		}
	}
	if same == nil || same.Title != "Dinner" {
		t.Fatalf("duplicate not spotted: %+v", v)
	}
	mutate(t, b, func(d *book.Data, ch *book.Change) error { return Match(d, ch, "m", same.ID) })
	mutate(t, b, func(d *book.Data, ch *book.Change) error { return Reject(d, ch, "r") })
	if x := draft(t, b, "m"); x.Status != model.DraftMatched || x.Txn != same.ID {
		t.Fatalf("match %+v", x)
	}
	_ = b.View(func(d *book.Data, _ *book.Computed) error {
		if r, _ := d.Rule("zomato"); r.Category != "food" || r.Title != "Dinner" {
			t.Fatalf("match did not teach: %+v", r)
		}
		if len(d.Txns) != 1 {
			t.Fatal("match created a transaction")
		}
		return nil
	})
	mutate(t, b, func(d *book.Data, ch *book.Change) error { return Restore(d, ch, "r") })
	if x := draft(t, b, "r"); x.Status != model.DraftPending {
		t.Fatalf("restore %+v", x)
	}
}

// The balance an SMS reports is checked against what Arthik would show.
func TestBalanceWarning(t *testing.T) {
	b := newBook(t)
	ok := Item{ID: "ok", Source: SourceSMS, Date: "2026-10-09", Kind: KindIncome, Amount: money.FromRupees(400), Digits: "0353", Balance: money.FromRupees(10400)}
	off := Item{ID: "off", Source: SourceSMS, Date: "2026-10-10", Kind: KindIncome, Amount: money.FromRupees(100), Digits: "0353", Balance: money.FromRupees(99999)}
	ingest(t, b, ok, off)
	_ = b.View(func(d *book.Data, c *book.Computed) error {
		for _, v := range Views(d, c) {
			hasBal := strings.Contains(strings.Join(v.Warnings, " "), "balance")
			if (v.ID == "ok") == hasBal {
				t.Errorf("%s warnings: %v", v.ID, v.Warnings)
			}
		}
		return nil
	})
}

// Producer files on disk → drafts, through the runner (no AI configured).
func TestRunnerImportsFiles(t *testing.T) {
	b := newBook(t)
	dir := filepath.Join(b.Dir, book.InboxDir)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sms_phone.md"), []byte(`device: phone
messages:
  - {id: "1111111111111111", from: "AX-HDFCBK", at: "2026-10-09T20:34:40+05:30", body: "Spent Rs.2000 On HDFC Bank Card 4275 At GYFTR VIA SMARTBUY On 2026-10-09:20:34:36."}
  - {from: "AX-HDFCBK", at: "2026-10-09T21:00:00+05:30", body: "Spent Rs.50 On HDFC Bank Card 4275 At TEA On 2026-10-09:21:00:00."}
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "kronos.md"), []byte("updated: x\n\ndrafts:\n  - {id: sub-jio-2026-10-11, date: '2026-10-11', title: Jio, amount: 707.82, account: Cash, category: utility}\n"), 0o644)
	r := &Runner{Book: b, Cfg: config.Inbox{MaxAgeDays: 30, KeepDays: 120, PairDays: 2}, Loc: time.UTC}
	r.Tick(context.Background())
	var n int
	_ = b.View(func(d *book.Data, _ *book.Computed) error { n = len(d.Drafts); return nil })
	if n != 3 {
		t.Fatalf("%d drafts, want 3", n)
	}
	if x := draft(t, b, "kronos-sub-jio-2026-10-11"); x.Amount != money.MustParse("707.82") || x.Account != "cash" || x.Category != "utility" {
		t.Fatalf("kronos draft %+v", x)
	}
	if x := draft(t, b, "sms-1111111111111111"); x.Account != "hdfc" || x.Time != "20:34" {
		t.Fatalf("sms draft %+v", x)
	}
	r.Tick(context.Background()) // nothing changed: nothing new
	_ = b.View(func(d *book.Data, _ *book.Computed) error { n = len(d.Drafts); return nil })
	if n != 3 {
		t.Fatalf("second tick: %d drafts", n)
	}
}

// The AI only fills drafts that have nothing better, and only with a real category.
func TestRunnerAI(t *testing.T) {
	answer := `{"category":"gifts","title":"Gyftr"}`
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Model    string                     `json:"model"`
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" || !strings.Contains(req.Messages[1].Content, "gifts: Gifts") {
			http.Error(w, "bad request", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": answer}})
	}))
	defer srv.Close()
	b := newBook(t)
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		d.SetRule(model.Rule{ID: "swiggy", Match: "swiggy", Category: "food"}, ch)
		return nil
	})
	ingest(t, b, spend("g", "2026-10-09", 2000, "4275", "GYFTR VIA SMARTBUY"), spend("s", "2026-10-09", 99, "4275", "SWIGGY"))
	r := &Runner{Book: b, AI: ai.New(srv.URL, "test-model", 5*time.Second)}
	r.categoriseBatch(context.Background(), 5)
	if x := draft(t, b, "g"); x.Category != "gifts" || x.Via != model.ViaAI || x.Title != "Gyftr" || !x.AITried {
		t.Fatalf("AI draft %+v", x)
	}
	if calls != 1 {
		t.Fatalf("AI asked %d times (the learned draft must be skipped)", calls)
	}
	// A made-up category is ignored; the draft keeps its fallback.
	answer = `{"category":"crypto","title":""}`
	ingest(t, b, spend("x", today, 10, "4275", "SOMETHING NEW"))
	r.categoriseBatch(context.Background(), 5)
	if x := draft(t, b, "x"); x.Category != "misc" || x.Via != model.ViaDefault || !x.AITried {
		t.Fatalf("invented category used: %+v", x)
	}
	// Unreachable AI: nothing is marked, the runner pauses.
	srv.Close()
	ingest(t, b, spend("y", today, 10, "4275", "ANOTHER"))
	r.categoriseBatch(context.Background(), 5)
	if x := draft(t, b, "y"); x.AITried || r.aiPause.IsZero() {
		t.Fatalf("failure handling: %+v pause=%v", x, r.aiPause)
	}
}

// A file change the signature misses (same size and mtime, e.g. a copy that kept
// the old time) is still picked up by the periodic re-scan, and "Check now" imports at once.
func TestRunnerRescan(t *testing.T) {
	b := newBook(t)
	dir := filepath.Join(b.Dir, book.InboxDir)
	_ = os.MkdirAll(dir, 0o755)
	f := filepath.Join(dir, "sms_phone.md")
	msg := func(id, day string) string {
		return `  - {id: "` + id + `", from: "AX-HDFCBK", at: "` + day + `T20:00:00+05:30", body: "Spent Rs.50 On HDFC Bank Card 4275 At TEA On ` + day + `:20:00:00."}` + "\n"
	}
	write := func(body string, at time.Time) {
		t.Helper()
		if err := os.WriteFile(f, []byte("device: phone\nmessages:\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(f, at, at)
	}
	count := func() (n int) {
		_ = b.View(func(d *book.Data, _ *book.Computed) error { n = len(d.Drafts); return nil })
		return n
	}
	stamp := time.Date(2026, 10, 10, 23, 30, 0, 0, time.UTC)
	write(msg("aaaaaaaaaaaaaaaa", "2026-10-10"), stamp)
	r := &Runner{Book: b, Cfg: config.Inbox{MaxAgeDays: 30, KeepDays: 120, PairDays: 2, Rescan: 10 * time.Minute}, Loc: time.UTC}
	r.Tick(context.Background())
	if count() != 1 {
		t.Fatalf("first import: %d drafts", count())
	}
	// Same length and mtime, different message: invisible to the file check.
	write(msg("bbbbbbbbbbbbbbbb", "2026-10-11"), stamp)
	r.Tick(context.Background())
	if count() != 1 {
		t.Fatalf("imported before the re-scan was due: %d", count())
	}
	r.lastScan = time.Now().Add(-11 * time.Minute)
	r.Tick(context.Background())
	if count() != 2 {
		t.Fatalf("re-scan missed the change: %d drafts", count())
	}
	// "Check now" through the dataset registry.
	runners.Store(b.Dir, r)
	defer runners.Delete(b.Dir)
	write(msg("aaaaaaaaaaaaaaaa", "2026-10-10")+msg("cccccccccccccccc", "2026-10-11"), stamp)
	if added, ok, err := ImportFor(b.Dir); err != nil || !ok || added != 1 {
		t.Fatalf("check now: added %d ok %v err %v", added, ok, err)
	}
	if _, ok, _ := ImportFor("/nowhere"); ok {
		t.Fatal("no runner must report ok=false")
	}
}

func TestSMSDue(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	at := func(s string) time.Time { v, _ := time.ParseInLocation("2006-01-02 15:04", s, ist); return v }
	cases := []struct {
		now, last string
		want      bool
	}{
		{"2026-10-13 23:44", "2026-10-12 23:45", false}, // not yet today; yesterday's done
		{"2026-10-13 23:45", "2026-10-12 23:45", true},  // time reached
		{"2026-10-13 23:50", "2026-10-13 23:45", false}, // done for today
		{"2026-10-14 09:00", "2026-10-12 23:45", true},  // server was down at 23:45: catch up once
		{"2026-10-14 09:00", "2026-10-14 08:00", false}, // a manual run after the slot counts
		{"2026-10-14 09:00", "", true},                  // never imported
	}
	for _, c := range cases {
		last := time.Time{}
		if c.last != "" {
			last = at(c.last)
		}
		if got := SMSDue(at(c.now), "23:45", last); got != c.want {
			t.Errorf("now %s last %q: got %v", c.now, c.last, got)
		}
	}
	if SMSDue(at("2026-10-14 09:00"), "nope", time.Time{}) {
		t.Error("a bad time must never be due")
	}
}

// With a daily SMS time, Kronos drafts still come at once but SMS wait for the
// time (or "Check now"); the time of the last SMS import is kept in drafts.md.
func TestRunnerSMSSchedule(t *testing.T) {
	b := newBook(t)
	ist := time.FixedZone("IST", 5*3600+1800)
	mutate(t, b, func(d *book.Data, ch *book.Change) error {
		d.Settings.InboxSMSTime, ch.Settings = "23:45", true
		d.SMSImport, ch.Drafts = book.Stamp(time.Date(2026, 10, 10, 23, 50, 0, 0, ist)), true
		return nil
	})
	dir := filepath.Join(b.Dir, book.InboxDir)
	_ = os.MkdirAll(dir, 0o755)
	sms := func(ids ...string) {
		body := "device: phone\nmessages:\n"
		for _, id := range ids {
			body += `  - {id: "` + id + `", from: "AX-HDFCBK", at: "2026-10-11T20:00:00+05:30", body: "Spent Rs.50 On HDFC Bank Card 4275 At TEA On 2026-10-11:20:00:00."}` + "\n"
		}
		_ = os.WriteFile(filepath.Join(dir, "sms_phone.md"), []byte(body), 0o644)
	}
	sms("aaaaaaaaaaaaaaaa")
	_ = os.WriteFile(filepath.Join(dir, "kronos.md"), []byte("drafts:\n  - {id: sub-jio-2026-10-11, date: '2026-10-11', title: Jio, amount: 707.82}\n"), 0o644)
	clock := time.Date(2026, 10, 11, 22, 0, 0, 0, ist)
	r := &Runner{Book: b, Cfg: config.Inbox{MaxAgeDays: 30, KeepDays: 120, PairDays: 2, Rescan: 10 * time.Minute}, Loc: ist, Now: func() time.Time { return clock }}
	ids := func() (out []string) {
		_ = b.View(func(d *book.Data, _ *book.Computed) error {
			for _, x := range d.Drafts {
				out = append(out, x.ID)
			}
			return nil
		})
		return out
	}
	r.Tick(context.Background())
	if got := ids(); len(got) != 1 || got[0] != "kronos-sub-jio-2026-10-11" {
		t.Fatalf("22:00: want only the Kronos draft, got %v", got)
	}
	clock = time.Date(2026, 10, 11, 23, 46, 0, 0, ist)
	r.Tick(context.Background())
	if got := ids(); len(got) != 2 {
		t.Fatalf("23:46: SMS not imported: %v", got)
	}
	var last string
	_ = b.View(func(d *book.Data, _ *book.Computed) error { last = d.SMSImport; return nil })
	if !book.ParseStamp(last).Equal(clock) {
		t.Fatalf("last SMS import = %s", last)
	}
	sms("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb") // arrives after the daily run
	clock = clock.Add(10 * time.Minute)
	r.Tick(context.Background())
	if got := ids(); len(got) != 2 {
		t.Fatalf("SMS imported before the next day's time: %v", got)
	}
	if added, err := r.Import(); err != nil || added != 1 { // "Check now" reads SMS anyway
		t.Fatalf("check now: %d %v", added, err)
	}
	if st := StatusOf(b.Dir); st.SMSTime != "" { // not registered: zero status is fine
		t.Fatalf("status %+v", st)
	}
	if r.status.SMSTime != "23:45" || !r.status.NextSMS.Equal(time.Date(2026, 10, 12, 23, 45, 0, 0, ist)) {
		t.Fatalf("schedule shown = %q next %v", r.status.SMSTime, r.status.NextSMS)
	}
	// The stamp survives a reload from disk.
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	_ = b.View(func(d *book.Data, _ *book.Computed) error { last = d.SMSImport; return nil })
	if last == "" {
		t.Fatal("sms_imported was not saved in drafts.md")
	}
}

// The phone's own timings come from the header of its SMS file.
func TestPhones(t *testing.T) {
	b := newBook(t)
	dir := filepath.Join(b.Dir, book.InboxDir)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sms_phone.md"), []byte("device: \"phone\"\nschedule: \"23:30\"\nlast_run: \"2026-10-13T23:30:02+05:30\"\nmessages:\n  []\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "sms_phone.sync-conflict-20261013-111111-ABCDEFG.md"), []byte("device: x\nmessages: []\n"), 0o644)
	got := Phones(b.Dir)
	if len(got) != 1 || got[0].Device != "phone" || got[0].Schedule != "23:30" || got[0].LastRun != "2026-10-13T23:30:02+05:30" {
		t.Fatalf("phones = %+v", got)
	}
}
