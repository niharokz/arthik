// Package demo generates the read-only demo dataset: about six months of realistic,
// made-up finances (salary, rent, groceries, two credit cards, a car loan, a SIP,
// market gains/losses, budgets, labels, reminders and a few future-dated items).
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

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
)

const markerFile = ".demo-seed"

// NeedsSeed reports whether dir has no demo data for the current month.
func NeedsSeed(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, markerFile))
	return err != nil || strings.TrimSpace(string(b)) != dates.Today().Format("2006-01")
}

// Seed (re)creates the demo dataset in dir. It only ever deletes Arthik's own file
// names inside dir, never anything else.
func Seed(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, book.TxDir), 0o755); err != nil {
		return err
	}
	for _, f := range []string{book.SettingsFile, book.AccountsFile, book.CategoriesFile, book.LabelsFile, book.RemindersFile, book.MoneyFile} {
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
		ch.Settings, ch.Accounts, ch.Categories, ch.Labels, ch.Reminders = true, true, true, true, true
		d.Settings = model.DefaultSettings()
		d.Accounts = accounts(start)
		d.Categories = categories()
		d.Labels = []model.Label{{ID: "home", Name: "home"}, {ID: "office", Name: "office"}, {ID: "goa-trip", Name: "Goa trip", Color: "#d08a3a"}, {ID: "reimbursable", Name: "reimbursable"}}
		d.Reminders = reminders(start)
		d.Reindex()
		if err := daily(d, ch, rng, start, today); err != nil {
			return err
		}
		d.PostDue(today, ch) // posts every recurring occurrence from start up to today
		return future(d, ch, today)
	})
	if err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, markerFile), []byte(today.Format("2006-01")+"\n"), 0o644)
}

func amt(r int64) money.Amount { return money.FromRupees(r) }

func accounts(start dates.Date) []model.Account {
	od := start.AddDays(-1).String()
	return []model.Account{
		{ID: "savings", Name: "Savings Bank", Type: "bank", OpeningBalance: amt(185000), OpeningDate: od, Color: "#3a807a"},
		{ID: "salary", Name: "Salary Account", Type: "bank", OpeningBalance: amt(42000), OpeningDate: od, Color: "#4f6fa8"},
		{ID: "cash", Name: "Cash", Type: "cash", OpeningBalance: amt(6500), OpeningDate: od},
		{ID: "wallet", Name: "UPI Wallet", Type: "wallet", OpeningBalance: amt(1200), OpeningDate: od},
		{ID: "mutual-funds", Name: "Mutual Funds", Type: "investment", OpeningBalance: amt(320000), OpeningDate: od, Color: "#7a5ea8"},
		{ID: "fixed-deposit", Name: "Fixed Deposit", Type: "deposit", OpeningBalance: amt(200000), OpeningDate: od},
		{ID: "rewards-card", Name: "Rewards Card", Type: "credit_card", OpeningBalance: amt(-18450), OpeningDate: od, CreditLimit: amt(250000), StatementDay: 18, DueDay: 7, Color: "#b4553f"},
		{ID: "travel-card", Name: "Travel Card", Type: "credit_card", OpeningBalance: amt(-4200), OpeningDate: od, CreditLimit: amt(150000), StatementDay: 2, DueDay: 22},
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

func reminders(start dates.Date) []model.Reminder {
	at := func(day int) string { return dates.New(start.Year(), start.Month(), day).String() }
	return []model.Reminder{
		{ID: "r-salary", Title: "Salary", Type: model.TxIncome, Amount: amt(128500), Account: "salary", Category: "salary-in", Repeat: "monthly", Next: at(1), Status: model.StatusCleared},
		{ID: "r-rent", Title: "Rent", Type: model.TxExpense, Amount: amt(24000), Account: "savings", Category: "housing-rent", Labels: []string{"home"}, Repeat: "monthly", Next: at(3), Status: model.StatusCleared},
		{ID: "r-sip", Title: "Index fund SIP", Type: model.TxTransfer, Amount: amt(15000), Account: "salary", ToAccount: "mutual-funds", Repeat: "monthly", Next: at(5), Status: model.StatusCleared},
		{ID: "r-to-savings", Title: "Move to savings", Type: model.TxTransfer, Amount: amt(75000), Account: "salary", ToAccount: "savings", Repeat: "monthly", Next: at(2), Status: model.StatusCleared},
		{ID: "r-car-emi", Title: "Car loan EMI", Type: model.TxTransfer, Amount: amt(12500), Account: "savings", ToAccount: "car-loan", Repeat: "monthly", Next: at(10), Status: model.StatusCleared},
		{ID: "r-car-interest", Title: "Car loan interest", Type: model.TxExpense, Amount: amt(2950), Account: "car-loan", Category: "loan-interest", Repeat: "monthly", Next: at(10)},
		{ID: "r-internet", Title: "Fibre internet", Type: model.TxExpense, Amount: amt(799), Account: "rewards-card", Category: "housing-internet", Labels: []string{"home"}, Repeat: "monthly", Next: at(12)},
		{ID: "r-streaming", Title: "Streaming subscription", Type: model.TxExpense, Amount: amt(649), Account: "rewards-card", Category: "entertainment-subscriptions", Repeat: "monthly", Next: at(15)},
		{ID: "r-mobile", Title: "Mobile recharge", Type: model.TxExpense, Amount: amt(299), Account: "wallet", Category: "bills-mobile", Repeat: "monthly", Next: at(20)},
		{ID: "r-gym", Title: "Gym membership", Type: model.TxExpense, Amount: amt(4500), Account: "savings", Category: "health-gym", Repeat: "quarterly", Next: at(8)},
		{ID: "r-fd-interest", Title: "FD interest", Type: model.TxIncome, Amount: amt(3550), Account: "fixed-deposit", Category: "interest", Repeat: "quarterly", Next: dates.New(start.Year(), start.Month(), 28).AddMonths(2, 28).String()},
		{ID: "r-insurance", Title: "Health insurance premium", Type: model.TxExpense, Amount: amt(18600), Account: "savings", Category: "bills-insurance", Repeat: "yearly", Next: dates.New(start.Year(), start.Month(), 25).AddMonths(7, 25).String()},
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
