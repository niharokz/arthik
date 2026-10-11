package inbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/money"
)

func mustParser(t *testing.T, user string) *Parser {
	t.Helper()
	p, err := NewParser([]byte(user))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Real messages from the banks in use, read by the built-in rules.
func TestParseBankSamples(t *testing.T) {
	p := mustParser(t, "")
	cases := []struct {
		body string
		want Parsed
	}{
		{"Your A/C XXXXX810353 is credited with INR 400.00 on 10/10/26 23:53. Your new balance is INR 10,41,284.21. Team IDFC FIRST Bank",
			Parsed{Rule: "account-first", Kind: KindIncome, Amount: money.MustParse("400"), Digits: "0353", Date: "2026-10-10", Time: "23:53", Balance: money.MustParse("1041284.21")}},
		{"INR 20.00 spent on YES BANK Card X3334 @UPI_SANJAY KUMAR ROUT 10-10-2026 09:57:36 pm. Avl Lmt INR 233,870.00. SMS BLKCC 3334 to 9840909000 if not you",
			Parsed{Rule: "card-spent", Kind: KindExpense, Amount: money.MustParse("20"), Digits: "3334", Merchant: "SANJAY KUMAR ROUT", Date: "2026-10-10", Time: "21:57", Available: money.MustParse("233870")}},
		{"Spent Rs.2000 On HDFC Bank Card 4275 At GYFTR VIA SMARTBUY On 2026-10-09:20:34:36.Not You? To Block+Reissue Call 18002586161/SMS BLOCK CC 4275 to 7308080808",
			Parsed{Rule: "card-spent-verb-first", Kind: KindExpense, Amount: money.MustParse("2000"), Digits: "4275", Merchant: "GYFTR VIA SMARTBUY", Date: "2026-10-09", Time: "20:34"}},
		{"Rs.131.00 spent on your SBI Credit Card ending 6338 at SWIGGYPVTLTDFOOD1 on 09/10/26. Trxn. not done by you? Report at https://sbicard.com/Dispute",
			Parsed{Rule: "card-spent", Kind: KindExpense, Amount: money.MustParse("131"), Digits: "6338", Merchant: "SWIGGYPVTLTDFOOD1", Date: "2026-10-09"}},
		{"Payment of Rs 5,000 received towards your SBI Credit Card ending 6338 on 08/10/26",
			Parsed{Rule: "card-payment", Kind: KindTransferIn, Amount: money.MustParse("5000"), Digits: "6338", Date: "2026-10-08"}},
		{"Rs.500.00 debited from A/c XX1234 on 09-10-26 to VPA shop@okaxis. Not you? call",
			Parsed{Rule: "amount-first", Kind: KindExpense, Amount: money.MustParse("500"), Digits: "1234", Merchant: "shop", Date: "2026-10-09"}},
		{"A/c XX1234 debited by Rs.500.00 on 09Oct26 trf to SANJAY KUMAR Refno 1234",
			Parsed{Rule: "account-first", Kind: KindExpense, Amount: money.MustParse("500"), Digits: "1234", Merchant: "SANJAY KUMAR", Date: "2026-10-09"}},
		{"Dear Customer, Rs 250.00 has been refunded to your card ending 4275 by AMAZON on 11-10-2026",
			Parsed{Rule: "generic", Generic: true, Kind: KindIncome, Amount: money.MustParse("250"), Digits: "4275", Merchant: "AMAZON", Date: "2026-10-11"}},
	}
	for _, c := range cases {
		got, ok := p.Parse(c.body)
		if !ok || got != c.want {
			t.Errorf("%.40s…\n got %+v (ok=%v)\nwant %+v", c.body, got, ok, c.want)
		}
	}
}

func TestParseIgnoresNonTransactions(t *testing.T) {
	p := mustParser(t, "")
	for _, body := range []string{
		"123456 is your OTP for txn of Rs 500 at AMAZON. Do not share.",
		"Get a pre-approved loan of Rs 5,00,000 today. Apply now",
		"Rs 1,200 will be debited from your A/c XX0353 on 15-10-26 for your SIP",
		"Your HDFC card bill of Rs 18,450 is due on 13/10/26. Minimum amount due Rs 920",
		"Txn of Rs 500 on card 4275 declined due to insufficient balance",
		"Hello! Your parcel is out for delivery.",
	} {
		if got, ok := p.Parse(body); ok {
			t.Errorf("imported %q as %+v", body, got)
		}
	}
}

// The user's file goes first, can switch built-ins off and add words.
func TestUserRules(t *testing.T) {
	p := mustParser(t, `
disable: [card-spent]
rules:
  - name: my-bank
    type: expense
    match: '(?i)paid\s+{cur}\s*(?P<amount>{amount})\s+to\s+(?P<merchant>\w+)'
`)
	got, ok := p.Parse("You paid INR 99 to TEASHOP using card 1234")
	if !ok || got.Rule != "my-bank" || got.Merchant != "TEASHOP" || got.Amount != money.MustParse("99") {
		t.Fatalf("user rule: %+v %v", got, ok)
	}
	got, _ = p.Parse("Rs.131.00 spent on your SBI Credit Card ending 6338 at SWIGGY on 09/10/26.")
	if got.Rule == "card-spent" {
		t.Fatal("disabled rule still used")
	}
	if _, err := NewParser([]byte("rules:\n  - name: bad\n    match: 'no amount group'\n")); err == nil {
		t.Fatal("rule without an amount group accepted")
	}
	if _, err := NewParser([]byte("rules: [")); err == nil {
		t.Fatal("broken YAML accepted")
	}
}

func TestKeyAndPretty(t *testing.T) {
	for in, want := range map[string]string{
		"UPI_SANJAY KUMAR ROUT": "upi sanjay kumar rout",
		"Sanjay  Kumar Rout":    "sanjay kumar rout",
		"SWIGGYPVTLTDFOOD1":     "swiggypvtltdfood",
		"GYFTR VIA SMARTBUY":    "gyftr via smartbuy",
		"AMAZON 12345":          "amazon",
		"":                      "",
	} {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"GYFTR VIA SMARTBUY": "Gyftr Via Smartbuy", "SWIGGYPVTLTDFOOD1": "Swiggypvtltdfood", "HDFC BANK": "HDFC Bank"} {
		if got := Pretty(in); got != want {
			t.Errorf("Pretty(%q) = %q, want %q", in, got, want)
		}
	}
}

// testdata/sms_from_phone.md was written by the Android app's SmsStore (quotes,
// backslash, newline and a control character in the body): the engine must read it.
func TestReadsFileWrittenByPhone(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/sms_from_phone.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inbox", "sms_phone.md"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	items, errs := ReadSources(dir, mustParser(t, ""), time.UTC)
	if len(errs) > 0 || len(items) != 1 {
		t.Fatalf("items %+v errs %v", items, errs)
	}
	it := items[0]
	if it.ID != "sms-42826d02ee9f0b6f" || it.Amount != money.MustParse("2000") || it.Digits != "4275" || !strings.Contains(it.Raw, `"GYFTR"`) || !strings.Contains(it.Raw, "\n") {
		t.Fatalf("item %+v", it)
	}
}

// testdata/kronos_from_kronos.md was written by Kronos's arthik_subscriptions job.
func TestReadsFileWrittenByKronos(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "inbox"), 0o755)
	raw, err := os.ReadFile("testdata/kronos_from_kronos.md")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "inbox", "kronos.md"), raw, 0o644)
	items, errs := ReadSources(dir, mustParser(t, ""), time.UTC)
	if len(errs) > 0 || len(items) != 1 {
		t.Fatalf("items %+v errs %v", items, errs)
	}
	it := items[0]
	if it.ID != "kronos-sub-jio-fibrenet-2026-10-14" || it.Amount != money.MustParse("707.82") || it.AccountHint != "Amazon Pay ICICI Credit Card" ||
		it.CategoryHint != "utility" || it.Date != "2026-10-14" || it.Title != "Jio Fibrenet" || it.Notes != "Subscription · monthly" {
		t.Fatalf("item %+v", it)
	}
}
