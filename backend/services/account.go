package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"arthik/config"
	"arthik/models"
)

var accounts []models.Account

// LoadAccounts loads accounts from CSV file
func LoadAccounts() error {
	file, err := os.Open(config.AccountsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	accounts = []models.Account{}
	for i, record := range records {
		if i == 0 || len(record) < 4 {
			continue
		}
		
		balance, _ := strconv.ParseFloat(record[3], 64)
		acc := models.Account{
			Name:              record[0],
			Category:          record[1],
			IncludeInNetWorth: record[2] == "yes",
			CurrentBalance:    balance,
		}

		if len(record) > 4 && record[1] == "Liabilities" {
			if len(record) > 4 {
				acc.DueDate = record[4]
			}
			if len(record) > 5 {
				acc.LastPaymentDate = record[5]
			}
		} else if len(record) > 4 && record[1] == "Expenses" {
			if len(record) > 4 {
				budget, _ := strconv.ParseFloat(record[4], 64)
				acc.Budget = budget
			}
		}

		accounts = append(accounts, acc)
	}

	return nil
}

// SaveAccounts saves accounts to CSV file
func SaveAccounts() error {
	file, err := os.Create(config.AccountsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"accountName", "accountCategory", "includeInNetWorth", "currentBalance", "dueDate", "lastPaymentDate", "budget"})

	for _, acc := range accounts {
		includeStr := "no"
		if acc.IncludeInNetWorth {
			includeStr = "yes"
		}

		row := []string{
			acc.Name,
			acc.Category,
			includeStr,
			fmt.Sprintf("%.2f", acc.CurrentBalance),
		}

		if acc.Category == "Liabilities" {
			row = append(row, acc.DueDate, acc.LastPaymentDate, "")
		} else if acc.Category == "Expenses" {
			row = append(row, "", "", fmt.Sprintf("%.2f", acc.Budget))
		} else {
			row = append(row, "", "", "")
		}

		writer.Write(row)
	}

	return nil
}

// GetAllAccounts returns all accounts
func GetAllAccounts() []models.Account {
	return accounts
}

// GetAccountByName returns an account by name
func GetAccountByName(name string) (*models.Account, error) {
	for i := range accounts {
		if accounts[i].Name == name {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("account not found")
}

// AddAccount adds a new account
func AddAccount(acc models.Account) error {
	// Check if account already exists
	for _, existing := range accounts {
		if existing.Name == acc.Name {
			return fmt.Errorf("account already exists")
		}
	}

	accounts = append(accounts, acc)
	return SaveAccounts()
}

// UpdateAccount updates an existing account
func UpdateAccount(oldName string, newAcc models.Account) error {
	for i := range accounts {
		if accounts[i].Name == oldName {
			accounts[i] = newAcc
			return SaveAccounts()
		}
	}
	return fmt.Errorf("account not found")
}

// DeleteAccount deletes an account
func DeleteAccount(name string) error {
	var newAccounts []models.Account
	found := false
	
	for _, acc := range accounts {
		if acc.Name != name {
			newAccounts = append(newAccounts, acc)
		} else {
			found = true
		}
	}
	
	if !found {
		return fmt.Errorf("account not found")
	}
	
	accounts = newAccounts
	return SaveAccounts()
}

// UpdateAccountBalance updates an account's balance
func UpdateAccountBalance(name string, delta float64) error {
	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].CurrentBalance += delta
			return nil
		}
	}
	return fmt.Errorf("account not found")
}

// GetAccountsByCategory returns accounts filtered by category
func GetAccountsByCategory(category string) []models.Account {
	var filtered []models.Account
	for _, acc := range accounts {
		if acc.Category == category {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}

// AccountExists checks if an account exists by name
func AccountExists(name string) bool {
	for _, acc := range accounts {
		if acc.Name == name {
			return true
		}
	}
	return false
}

// InitializeDefaultAccounts creates default accounts if none exist
func InitializeDefaultAccounts() error {
	if len(accounts) > 0 {
		return nil
	}

	for _, def := range config.DefaultAccounts {
		accounts = append(accounts, models.Account{
			Name:              def.Name,
			Category:          def.Category,
			IncludeInNetWorth: def.IncludeInNetWorth,
			CurrentBalance:    0,
		})
	}

	return SaveAccounts()
}