package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"arthik/config"
	"arthik/models"
	"arthik/utils"
)

// LoadTransactions loads transactions for a given month
func LoadTransactions(month string) ([]models.Transaction, error) {
	filename := config.GetTransactionFilePath(month)
	file, err := os.Open(filename)
	if err != nil {
		return []models.Transaction{}, nil // Return empty if file doesn't exist
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	transactions := []models.Transaction{}
	for i, record := range records {
		if i == 0 || len(record) < 5 {
			continue
		}
		
		amount, _ := strconv.ParseFloat(record[4], 64)
		date := utils.ParseTransactionID(record[0])
		
		transactions = append(transactions, models.Transaction{
			ID:          record[0],
			From:        record[1],
			To:          record[2],
			Description: record[3],
			Amount:      amount,
			Date:        date,
		})
	}

	return transactions, nil
}

// LoadAllTransactions loads transactions from all available month files
func LoadAllTransactions() ([]models.Transaction, error) {
	var allTransactions []models.Transaction
	
	// Read all files in the ledger directory
	files, err := os.ReadDir(config.LedgerDir)
	if err != nil {
		// If directory doesn't exist, return empty slice
		if os.IsNotExist(err) {
			return []models.Transaction{}, nil
		}
		return nil, err
	}
	
	// Load transactions from each file
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		// Only process CSV files that match pattern: transactions_YYYYMM.csv
		filename := file.Name()
		if len(filename) < 20 || filename[:13] != "transactions_" || filename[len(filename)-4:] != ".csv" {
			continue
		}
		
		// Extract month from filename (format: transactions_YYYYMM.csv)
		month := filename[13 : len(filename)-4]
		transactions, err := LoadTransactions(month)
		if err != nil {
			continue // Skip files that can't be read
		}
		
		allTransactions = append(allTransactions, transactions...)
	}
	
	return allTransactions, nil
}

// SaveTransaction saves a transaction to the appropriate month's file
func SaveTransaction(t models.Transaction) error {
	month := utils.GetMonthFromDate(t.Date)
	filename := config.GetTransactionFilePath(month)

	var records [][]string
	if file, err := os.Open(filename); err == nil {
		reader := csv.NewReader(file)
		records, _ = reader.ReadAll()
		file.Close()
	} else {
		records = [][]string{{"tranid", "FROM", "TO", "DESCRIPTION", "AMOUNT"}}
	}

	// Check if transaction already exists (update case)
	found := false
	for i, record := range records {
		if i > 0 && record[0] == t.ID {
			records[i] = []string{t.ID, t.From, t.To, t.Description, fmt.Sprintf("%.2f", t.Amount)}
			found = true
			break
		}
	}

	// Add new transaction if not found
	if !found {
		records = append(records, []string{
			t.ID,
			t.From,
			t.To,
			t.Description,
			fmt.Sprintf("%.2f", t.Amount),
		})
	}

	// Write back to file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(records)
}

// DeleteTransaction deletes a transaction
func DeleteTransaction(id string) error {
	date := utils.ParseTransactionID(id)
	month := utils.GetMonthFromDate(date)
	filename := config.GetTransactionFilePath(month)

	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	file.Close()
	if err != nil {
		return err
	}

	var newRecords [][]string
	for _, record := range records {
		if len(record) > 0 && record[0] != id {
			newRecords = append(newRecords, record)
		}
	}

	file, err = os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	return writer.WriteAll(newRecords)
}

// ApplyTransaction applies a transaction to account balances
func ApplyTransaction(t models.Transaction) error {
	// Deduct from 'from' account
	if err := UpdateAccountBalance(t.From, -t.Amount); err != nil {
		return err
	}
	
	// Add to 'to' account
	if err := UpdateAccountBalance(t.To, t.Amount); err != nil {
		// Rollback the first update
		UpdateAccountBalance(t.From, t.Amount)
		return err
	}
	
	return SaveAccounts()
}

// ReverseTransaction reverses a transaction's effect on account balances
func ReverseTransaction(id string) error {
	date := utils.ParseTransactionID(id)
	month := utils.GetMonthFromDate(date)
	transactions, err := LoadTransactions(month)
	if err != nil {
		return err
	}

	for _, t := range transactions {
		if t.ID == id {
			// Reverse the transaction
			UpdateAccountBalance(t.From, t.Amount)
			UpdateAccountBalance(t.To, -t.Amount)
			return SaveAccounts()
		}
	}

	return fmt.Errorf("transaction not found")
}

// GetTransactionsByDateRange returns transactions within a date range
func GetTransactionsByDateRange(start, end time.Time) ([]models.Transaction, error) {
	var allTransactions []models.Transaction
	
	// Iterate through months in the range
	current := start
	for current.Before(end) || current.Equal(end) {
		month := utils.GetMonthFromDate(current)
		transactions, err := LoadTransactions(month)
		if err != nil {
			return nil, err
		}
		
		// Filter by date range
		for _, t := range transactions {
			if (t.Date.After(start) || t.Date.Equal(start)) && 
			   (t.Date.Before(end) || t.Date.Equal(end)) {
				allTransactions = append(allTransactions, t)
			}
		}
		
		// Move to next month
		current = current.AddDate(0, 1, 0)
	}
	
	return allTransactions, nil
}