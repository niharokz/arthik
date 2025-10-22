package services

import (
	"arthik/models"
	"arthik/utils"
)

// CalculateDashboardStats calculates all dashboard statistics
func CalculateDashboardStats() (models.DashboardStats, error) {
	stats := models.DashboardStats{}

	// Calculate assets and liabilities
	var totalAssets, totalLiabilities float64
	for _, acc := range accounts {
		if acc.IncludeInNetWorth {
			if acc.Category == "Assets" {
				totalAssets += acc.CurrentBalance
			} else if acc.Category == "Liabilities" {
				totalLiabilities += acc.CurrentBalance
			}
		}
	}

	stats.TotalAssets = totalAssets
	stats.TotalLiabilities = totalLiabilities
	stats.NetWorth = totalAssets - totalLiabilities

	// Calculate current month's income and expenses
	month := utils.GetCurrentMonth()
	transactions, err := LoadTransactions(month)
	if err != nil {
		return stats, err
	}

	var monthIncome, monthExpenses float64
	categoryExpenses := make(map[string]float64)

	for _, t := range transactions {
		fromCat, toCat := getAccountCategories(t.From, t.To)

		if fromCat == "Revenue" {
			monthIncome += t.Amount
		}
		if toCat == "Expenses" && t.To != "BackupExpenses" {
			monthExpenses += t.Amount
			categoryExpenses[t.To] += t.Amount
		}
	}

	stats.MonthIncome = monthIncome
	stats.MonthExpenses = monthExpenses
	stats.MonthSavings = monthIncome - monthExpenses

	// Calculate budget vs expenses
	stats.BudgetVsExpenses = calculateBudgetVsExpenses(categoryExpenses)

	// Load historical data
	stats.HistoricalData, _ = LoadMonthlyReports()

	return stats, nil
}

// getAccountCategories returns the categories for from and to accounts
func getAccountCategories(from, to string) (string, string) {
	var fromCat, toCat string
	
	for _, acc := range accounts {
		if acc.Name == from {
			fromCat = acc.Category
		}
		if acc.Name == to {
			toCat = acc.Category
		}
	}
	
	return fromCat, toCat
}

// calculateBudgetVsExpenses calculates budget vs actual expenses
func calculateBudgetVsExpenses(categoryExpenses map[string]float64) []models.BudgetExpense {
	var budgetVsExpenses []models.BudgetExpense
	
	for _, acc := range accounts {
		if acc.Category == "Expenses" && acc.Name != "BackupExpenses" {
			actual := categoryExpenses[acc.Name]
			budgetVsExpenses = append(budgetVsExpenses, models.BudgetExpense{
				Category: acc.Name,
				Budget:   acc.Budget,
				Actual:   actual,
			})
		}
	}
	
	return budgetVsExpenses
}

// CalculateNetWorth calculates the current net worth
func CalculateNetWorth() float64 {
	var totalAssets, totalLiabilities float64
	
	for _, acc := range accounts {
		if acc.IncludeInNetWorth {
			if acc.Category == "Assets" {
				totalAssets += acc.CurrentBalance
			} else if acc.Category == "Liabilities" {
				totalLiabilities += acc.CurrentBalance
			}
		}
	}
	
	return totalAssets - totalLiabilities
}

// CalculateMonthlyIncome calculates income for a given month
func CalculateMonthlyIncome(month string) (float64, error) {
	transactions, err := LoadTransactions(month)
	if err != nil {
		return 0, err
	}

	var income float64
	for _, t := range transactions {
		fromCat, _ := getAccountCategories(t.From, t.To)
		if fromCat == "Revenue" {
			income += t.Amount
		}
	}

	return income, nil
}

// CalculateMonthlyExpenses calculates expenses for a given month
func CalculateMonthlyExpenses(month string) (float64, error) {
	transactions, err := LoadTransactions(month)
	if err != nil {
		return 0, err
	}

	var expenses float64
	for _, t := range transactions {
		_, toCat := getAccountCategories(t.From, t.To)
		if toCat == "Expenses" && t.To != "BackupExpenses" {
			expenses += t.Amount
		}
	}

	return expenses, nil
}

// CalculateSavingsRate calculates the savings rate percentage
func CalculateSavingsRate(income, expenses float64) float64 {
	if income == 0 {
		return 0
	}
	return ((income - expenses) / income) * 100
}

// GetAssetDistribution returns asset distribution by account
func GetAssetDistribution() []models.Account {
	var assetAccounts []models.Account
	
	for _, acc := range accounts {
		if acc.Category == "Assets" && acc.CurrentBalance > 0 {
			assetAccounts = append(assetAccounts, acc)
		}
	}
	
	return assetAccounts
}