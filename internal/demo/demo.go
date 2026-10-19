// Package demo generates the read-only demo dataset: about six months of realistic,
// made-up finances (salary, rent, groceries, two credit cards, a car loan, a SIP,
// market gains/losses, budgets, labels, a few future-dated items and an inbox of
// drafts as if they came from SMS and Kronos).
//
// The data is generated relative to today and refreshed once a month (on start-up
// and by a daily check), so the demo always looks current. It is deterministic for
// a given month.
package demo

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/inbox"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

const markerFile = ".demo-seed"

// seedVersion changes whenever the generated data changes shape, so an upgrade
// regenerates the demo instead of waiting for the next month.
const seedVersion = "2"

func marker() string { return dates.Today().Format("2006-01") + " v" + seedVersion }

// NeedsSeed reports whether dir has no demo data for the current month and version.
func NeedsSeed(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, markerFile))
	return err != nil || strings.TrimSpace(string(b)) != marker()
}

// Seed (re)creates the demo dataset in dir. It only ever deletes Arthik's own file
// names inside dir, never anything else.
func Seed(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, book.TxDir), 0o755); err != nil {
		return err
	}
	for _, f := range []string{book.SettingsFile, book.AccountsFile, book.CategoriesFile, book.LabelsFile, book.MoneyFile, book.DraftsFile, book.RulesFile, "reminders.md"} {
		_ = os.Remove(filepath.Join(dir, f))
	}
	old, _ := filepath.Glob(filepath.Join(dir, book.TxDir, "transaction_*.md"))
	for _, f := range old {
		_ = os.Remove(f)
	}
	b, err := book.Open(dir, "demo-seed", false)
	if err != nil {
		return err
	}
	today := dates.Today()
	start := dates.New(today.Year(), today.Month(), 1).AddMonths(-6, 1)
	rng := rand.New(rand.NewSource(int64(today.Year()*100 + int(today.Month()))))

	err = b.Mutate(func(d *book.Data, ch *book.Change) error {
		ch.Settings, ch.Accounts, ch.Categories, ch.Labels, ch.Drafts, ch.Rules = true, true, true, true, true, true
		d.Settings = model.DefaultSettings()
		d.Accounts = accounts(start)
		d.Categories = categories()
		d.Labels = []model.Label{{ID: "home", Name: "home"}, {ID: "office", Name: "office"}, {ID: "goa-trip", Name: "Goa trip", Color: "#d08a3a"}, {ID: "reimbursable", Name: "reimbursable"}}
		d.Reindex()
		if err := daily(d, ch, rng, start, today); err != nil {
			return err
		}
		if err := monthly(d, ch, start, today); err != nil {
			return err
		}
		if err := future(d, ch, today); err != nil {
			return err
		}
		inboxDrafts(d, ch, today)
		return nil
	})
	if err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, markerFile), []byte(marker()+"\n"), 0o644)
}

func amt(r int64) money.Amount { return money.FromRupees(r) }

func accounts(start dates.Date) []model.Account {
	od := start.AddDays(-1).String()
	return []model.Account{
		{ID: "savings", Name: "Savings Bank", Type: "bank", OpeningBalance: amt(185000), OpeningDate: od, Color: "#3a807a", SMSDigits: []string{"9001"}},
		{ID: "salary", Name: "Salary Account", Type: "bank", OpeningBalance: amt(42000), OpeningDate: od, Color: "#4f6fa8", SMSDigits: []string{"1122"}},
		{ID: "cash", Name: "Cash", Type: "cash", OpeningBalance: amt(6500), OpeningDate: od},
		{ID: "wallet", Name: "UPI Wallet", Type: "wallet", OpeningBalance: amt(1200), OpeningDate: od},
		{ID: "mutual-funds", Name: "Mutual Funds", Type: "investment", OpeningBalance: amt(320000), OpeningDate: od, Color: "#7a5ea8"},
		{ID: "fixed-deposit", Name: "Fixed Deposit", Type: "deposit", OpeningBalance: amt(200000), OpeningDate: od},
		{ID: "rewards-card", Name: "Rewards Card", Type: "credit_card", OpeningBalance: amt(-18450), OpeningDate: od, CreditLimit: amt(250000), StatementDay: 18, DueDay: 7, Color: "#b4553f", SMSDigits: []string{"4111"}},
		{ID: "travel-card", Name: "Travel Card", Type: "credit_card", OpeningBalance: amt(-4200), OpeningDate: od, CreditLimit: amt(150000), StatementDay: 2, DueDay: 22, SMSDigits: []string{"3222"}},
		{ID: "car-loan", Name: "Car Loan", Type: "loan", OpeningBalance: amt(-385000), OpeningDate: od},
	}
}

func cat(id, name, kind, parent, icon string, budget int64) model.Category {
	c := model.Category{ID: id, Name: name, Kind: kind, Parent: parent, Icon: icon}
	if budget > 0 {
		c.Budget, c.BudgetPeriod = amt(budget), "monthly"
	}
	return c
}

func categories() []model.Category {
	e, i := model.KindExpense, model.KindIncome
	return []model.Category{
		cat("food", "Food", e, "", "🍲", 16000),
		cat("food-groceries", "Groceries", e, "food", "🛒", 8000),
		cat("food-restaurants", "Restaurants", e, "food", "🍽️", 4500),
		cat("food-coffee", "Coffee & snacks", e, "food", "☕", 1500),
		cat("transport", "Transport", e, "", "🚗", 6000),
		cat("transport-fuel", "Fuel", e, "transport", "⛽", 0),
		cat("transport-cab", "Cab & auto", e, "transport", "🚕", 0),
		cat("transport-metro", "Metro & bus", e, "transport", "🚇", 0),
		cat("housing", "Housing", e, "", "🏠", 0),
		cat("housing-rent", "Rent", e, "housing", "🔑", 0),
		cat("housing-electricity", "Electricity", e, "housing", "💡", 0),
		cat("housing-internet", "Internet", e, "housing", "📶", 0),
		cat("housing-maintenance", "Maintenance", e, "housing", "🧰", 0),
		cat("shopping", "Shopping", e, "", "🛍️", 7000),
		cat("shopping-clothes", "Clothes", e, "shopping", "👕", 0),
		cat("shopping-electronics", "Electronics", e, "shopping", "🎧", 0),
		cat("shopping-home", "Home items", e, "shopping", "🪴", 0),
		cat("health", "Health", e, "", "🩺", 0),
		cat("health-medicine", "Medicine", e, "health", "💊", 0),
		cat("health-gym", "Gym", e, "health", "🏋️", 0),
		cat("entertainment", "Entertainment", e, "", "🎬", 2500),
		cat("entertainment-subscriptions", "Subscriptions", e, "entertainment", "📺", 0),
		cat("entertainment-movies", "Movies & events", e, "entertainment", "🎟️", 0),
		cat("bills", "Bills", e, "", "🧾", 0),
		cat("bills-mobile", "Mobile", e, "bills", "📱", 0),
		cat("bills-insurance", "Insurance", e, "bills", "🛡️", 0),
		cat("travel", "Travel", e, "", "✈️", 0),
		cat("gifts", "Gifts", e, "", "🎁", 0),
		cat("loan-interest", "Loan interest", e, "", "🏛️", 0),
		cat("salary-in", "Salary", i, "", "💼", 0),
		cat("interest", "Interest", i, "", "🏦", 0),
		cat("cashback", "Cashback & rewards", i, "", "🎉", 0),
		cat("other-income", "Other income", i, "", "➕", 0),
		cat("market", "Market", model.KindBoth, "", "📊", 0),
	}
}

// bill is a recurring transaction posted on day of every "every"-th month.
type bill struct {
	day, every, offset int // offset: first month (0 = the start month)
	e                  book.Entry
}

func bills() []bill {
	exp, inc, tr := model.TxExpense, model.TxIncome, model.TxTransfer
	return []bill{
		{1, 1, 0, book.Entry{Title: "Salary", Type: inc, Amount: amt(128500), Account: "salary", Category: "salary-in"}},
		{2, 1, 0, book.Entry{Title: "Move to savings", Type: tr, Amount: amt(75000), Account: "salary", ToAccount: "savings"}},
		{3, 1, 0, book.Entry{Title: "Rent", Type: exp, Amount: amt(24000), Account: "savings", Category: "housing-rent", Labels: []string{"home"}}},
		{5, 1, 0, book.Entry{Title: "Index fund SIP", Type: tr, Amount: amt(15000), Account: "salary", ToAccount: "mutual-funds"}},
		{10, 1, 0, book.Entry{Title: "Car loan EMI", Type: tr, Amount: amt(12500), Account: "savings", ToAccount: "car-loan"}},
		{10, 1, 0, book.Entry{Title: "Car loan interest", Type: exp, Amount: amt(2950), Account: "car-loan", Category: "loan-interest"}},
		{12, 1, 0, book.Entry{Title: "Fibre internet", Type: exp, Amount: amt(799), Account: "rewards-card", Category: "housing-internet", Labels: []string{"home"}}},
		{15, 1, 0, book.Entry{Title: "Streaming subscription", Type: exp, Amount: amt(649), Account: "rewards-card", Category: "entertainment-subscriptions"}},
		{20, 1, 0, book.Entry{Title: "Mobile recharge", Type: exp, Amount: amt(299), Account: "wallet", Category: "bills-mobile"}},
		{8, 3, 0, book.Entry{Title: "Gym membership", Type: exp, Amount: amt(4500), Account: "savings", Category: "health-gym"}},
		{28, 3, 2, book.Entry{Title: "FD interest", Type: inc, Amount: amt(3550), Account: "fixed-deposit", Category: "interest"}},
	}
}

// monthly posts the recurring bills from start up to today.
func monthly(d *book.Data, ch *book.Change, start, today dates.Date) error {
	for m := 0; ; m++ {
		first := start.AddMonths(m, 1)
		if first.After(today) {
			return nil
		}
		for _, b := range bills() {
			day := dates.New(first.Year(), first.Month(), 1).AddMonths(0, b.day)
			if m%b.every != b.offset%b.every || m < b.offset || day.After(today) {
				continue
			}
			e := b.e
			e.Date, e.Status, e.ID = day.String(), model.StatusCleared, d.NewTxID(day.String())
			t, err := d.BuildTx(e)
			if err != nil {
				return fmt.Errorf("%s %s: %w", e.Date, e.Title, err)
			}
			t.Labels = d.EnsureLabels(t.Labels, ch)
			d.SetTx(t, ch)
		}
	}
}

// inboxDrafts fills the review inbox with what the phone and Kronos would send.
func inboxDrafts(d *book.Data, ch *book.Change, today dates.Date) {
	d.SetRule(model.Rule{ID: "swiggy", Match: "swiggy", Title: "Swiggy", Category: "food-restaurants", Uses: 6}, ch)
	d.SetRule(model.Rule{ID: "ramesh-kumar", Match: "ramesh kumar", Title: "Milk (Ramesh)", Category: "food-groceries", Labels: []string{"home"}, Uses: 11}, ch)
	day := func(n int) string { return today.AddDays(-n).String() }
	sms := func(id, date, tm, raw string) inbox.Item {
		return inbox.Item{ID: "sms-demo-" + id, Source: inbox.SourceSMS, Date: date, Time: tm, Raw: raw}
	}
	items := []inbox.Item{}
	add := func(it inbox.Item, kind string, amount int64, digits, merchant string) {
		it.Kind, it.Amount, it.Digits, it.Merchant = kind, amt(amount), digits, merchant
		items = append(items, it)
	}
	add(sms("1", day(0), "13:42", "Rs.486.00 spent on your Rewards Card ending 4111 at SWIGGY on "+today.Format("02/01/06")+"."), inbox.KindExpense, 486, "4111", "SWIGGY")
	add(sms("2", day(0), "07:05", "INR 60.00 debited from A/c XX9001 to VPA ramesh kumar. UPI Ref 4121"), inbox.KindExpense, 60, "9001", "RAMESH KUMAR")
	add(sms("3", day(1), "19:20", "INR 1,249.00 spent on Travel Card X3222 @DECATHLON SPORTS "+today.AddDays(-1).Format("02-01-2006")+" 07:20:11 pm."), inbox.KindExpense, 1249, "3222", "DECATHLON SPORTS")
	add(sms("4", day(1), "10:02", "Rs 18,450.00 debited from A/c XX9001 towards card payment."), inbox.KindExpense, 18450, "9001", "")
	add(sms("5", day(1), "10:03", "Payment of Rs 18,450.00 received towards your Rewards Card ending 4111."), inbox.KindTransferIn, 18450, "4111", "")
	add(sms("6", day(2), "16:47", "INR 2,000.00 spent on card 9911 at GYFTR VIA SMARTBUY on "+today.AddDays(-2).Format("2006-01-02")+"."), inbox.KindExpense, 2000, "9911", "GYFTR VIA SMARTBUY")
	items = append(items, inbox.Item{ID: "kronos-sub-demo-internet", Source: inbox.SourceKronos, Date: day(0), Kind: inbox.KindExpense, Amount: amt(799),
		Title: "Fibre internet", Merchant: "Fibre internet", AccountHint: "Rewards Card", CategoryHint: "bills/internet", Notes: "Subscription · monthly", Rule: "subscription"})
	inbox.Ingest(d, ch, items, inbox.Options{Today: today, Now: time.Now(), MaxAge: 30, KeepDays: 120, PairDays: 2})
	if x, ok := d.Draft("sms-demo-3"); ok { // what the local AI would have suggested
		x.Category, x.Via, x.Title, x.AITried = "shopping-clothes", model.ViaAI, "Decathlon", true
		d.SetDraft(x, ch)
	}
}

type pick struct {
	title    string
	category string
	min, max int64
	accounts []string
	labels   []string
}

// daily adds everyday spending, card bill payments, top-ups and market moves.
func daily(d *book.Data, ch *book.Change, rng *rand.Rand, start, today dates.Date) error {
	add := func(e book.Entry) error {
		e.ID = d.NewTxID(e.Date)
		t, err := d.BuildTx(e)
		if err != nil {
			return fmt.Errorf("%s %s: %w", e.Date, e.Title, err)
		}
		d.SetTx(t, ch)
		return nil
	}
	between := func(a, b int64) money.Amount {
		v := a + rng.Int63n(b-a+1)
		if v > 300 {
			v = v / 10 * 10 // round rupees like real bills
		}
		return amt(v)
	}
	weekly := []pick{
		{"Big Basket order", "food-groceries", 900, 2600, []string{"rewards-card", "savings"}, []string{"home"}},
		{"Vegetables & fruits", "food-groceries", 180, 650, []string{"cash", "wallet"}, nil},
		{"Fuel", "transport-fuel", 1200, 2600, []string{"rewards-card"}, nil},
		{"Metro card recharge", "transport-metro", 300, 500, []string{"wallet"}, []string{"office"}},
	}
	often := []pick{
		{"Coffee", "food-coffee", 120, 380, []string{"wallet", "cash"}, nil},
		{"Lunch", "food-restaurants", 220, 650, []string{"wallet"}, []string{"office"}},
		{"Cab", "transport-cab", 140, 560, []string{"wallet", "travel-card"}, nil},
		{"Dinner out", "food-restaurants", 900, 3200, []string{"rewards-card", "travel-card"}, nil},
		{"Pharmacy", "health-medicine", 150, 900, []string{"wallet", "cash"}, nil},
		{"Snacks", "food-coffee", 60, 240, []string{"cash"}, nil},
	}
	rare := []pick{
		{"T-shirts", "shopping-clothes", 900, 3200, []string{"rewards-card"}, nil},
		{"Headphones", "shopping-electronics", 1800, 7500, []string{"rewards-card"}, nil},
		{"Plants & pots", "shopping-home", 400, 1600, []string{"wallet"}, []string{"home"}},
		{"Movie tickets", "entertainment-movies", 500, 1400, []string{"travel-card"}, nil},
		{"Birthday gift", "gifts", 800, 3500, []string{"savings", "rewards-card"}, nil},
		{"House cleaning supplies", "housing-maintenance", 300, 1100, []string{"cash"}, []string{"home"}},
	}
	status := func(day dates.Date) string {
		switch {
		case today.DaysUntil(day) < -45:
			return model.StatusReconciled
		case today.DaysUntil(day) < -5:
			return model.StatusCleared
		}
		return model.StatusUncleared
	}
	use := func(p pick, day dates.Date) error {
		return add(book.Entry{Type: model.TxExpense, Date: day.String(), Time: fmt.Sprintf("%02d:%02d", 8+rng.Intn(13), rng.Intn(60)),
			Title: p.title, Amount: between(p.min, p.max), Account: p.accounts[rng.Intn(len(p.accounts))],
			Category: p.category, Labels: p.labels, Status: status(day)})
	}
	for day := start; !day.After(today); day = day.AddDays(1) {
		wd := int(day.Weekday())
		for i, p := range weekly {
			if wd == (i*2+6)%7 {
				if err := use(p, day); err != nil {
					return err
				}
			}
		}
		for _, p := range often {
			if rng.Intn(100) < 22 {
				if err := use(p, day); err != nil {
					return err
				}
			}
		}
		if rng.Intn(100) < 5 {
			if err := use(rare[rng.Intn(len(rare))], day); err != nil {
				return err
			}
		}
		// Electricity on the 6th, card bills on their due days (paid in full), wallet top-ups.
		switch day.Day() {
		case 6:
			if err := add(book.Entry{Type: model.TxExpense, Date: day.String(), Title: "Electricity bill", Amount: between(1100, 2400),
				Account: "savings", Category: "housing-electricity", Labels: []string{"home"}, Status: status(day)}); err != nil {
				return err
			}
		case 7, 22:
			card := "rewards-card"
			if day.Day() == 22 {
				card = "travel-card"
			}
			if owed := -balanceAt(d, card, day.AddDays(-15)); owed > 0 {
				if err := add(book.Entry{Type: model.TxTransfer, Date: day.String(), Title: "Card bill payment", Amount: owed,
					Account: "savings", ToAccount: card, Status: status(day)}); err != nil {
					return err
				}
			}
		case 1, 11, 21:
			if err := add(book.Entry{Type: model.TxTransfer, Date: day.String(), Title: "Wallet top-up", Amount: amt(4000),
				Account: "salary", ToAccount: "wallet", Status: status(day)}); err != nil {
				return err
			}
		case 14, 28:
			if err := add(book.Entry{Type: model.TxTransfer, Date: day.String(), Title: "ATM withdrawal", Amount: amt(2000),
				Account: "savings", ToAccount: "cash", Status: status(day)}); err != nil {
				return err
			}
		}
		// Month end: mutual funds marked to market (gain or loss → the "Market" category).
		if day.AddDays(1).Day() == 1 {
			move := amt(rng.Int63n(19000) - 6000)
			e := book.Entry{Date: day.String(), Title: "Mutual funds — market value", Account: "mutual-funds", Category: "market", Amount: move, Type: model.TxIncome, Status: model.StatusReconciled}
			if move < 0 {
				e.Type, e.Amount = model.TxExpense, -move
			}
			if move != 0 {
				if err := add(e); err != nil {
					return err
				}
			}
			if err := add(book.Entry{Type: model.TxIncome, Date: day.String(), Title: "Card cashback", Amount: between(80, 420),
				Account: "rewards-card", Category: "cashback", Status: status(day)}); err != nil {
				return err
			}
		}
	}
	// One trip with a label and a split bill.
	trip := today.AddDays(-40)
	if trip.After(start) {
		_ = add(book.Entry{Type: model.TxExpense, Date: trip.String(), Title: "Flights to Goa", Amount: amt(9800), Account: "travel-card", Category: "travel", Labels: []string{"goa-trip"}, Status: status(trip)})
		_ = add(book.Entry{Type: model.TxExpense, Date: trip.AddDays(1).String(), Title: "Beach shack dinner", Account: "travel-card", Labels: []string{"goa-trip"}, Status: status(trip),
			Splits: []book.Split{{Category: "food-restaurants", Amount: amt(2400)}, {Category: "entertainment-movies", Amount: amt(600)}}})
		_ = add(book.Entry{Type: model.TxExpense, Date: trip.AddDays(2).String(), Title: "Scooter rental", Amount: amt(1500), Account: "cash", Category: "travel", Labels: []string{"goa-trip"}, Status: status(trip)})
		_ = add(book.Entry{Type: model.TxIncome, Date: trip.AddDays(9).String(), Title: "Team lunch refund", Amount: amt(1850), Account: "salary", Category: "other-income", Labels: []string{"reimbursable"}, Status: status(trip)})
	}
	return nil
}

// future adds a few future-dated transactions (they count from their date on).
func future(d *book.Data, ch *book.Change, today dates.Date) error {
	for _, e := range []book.Entry{
		{Type: model.TxExpense, Date: today.AddDays(6).String(), Title: "Concert tickets", Amount: amt(3500), Account: "rewards-card", Category: "entertainment-movies"},
		{Type: model.TxExpense, Date: today.AddDays(18).String(), Title: "Laptop (pre-order)", Amount: amt(84990), Account: "rewards-card", Category: "shopping-electronics"},
		{Type: model.TxTransfer, Date: today.AddDays(25).String(), Title: "Top up fixed deposit", Amount: amt(50000), Account: "savings", ToAccount: "fixed-deposit"},
	} {
		e.ID = d.NewTxID(e.Date)
		t, err := d.BuildTx(e)
		if err != nil {
			return err
		}
		d.SetTx(t, ch)
	}
	return nil
}

// balanceAt is a simple running balance used to size card bill payments.
func balanceAt(d *book.Data, account string, at dates.Date) money.Amount {
	a, _ := d.Account(account)
	bal := a.OpeningBalance
	for _, t := range d.Txns {
		td, err := dates.Parse(t.Date)
		if err != nil || td.After(at) {
			continue
		}
		for _, p := range t.Postings {
			if p.Account == account {
				bal += p.Amount
			}
		}
	}
	return bal
}
