package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"arthik/models"
	"arthik/services"
	"arthik/utils"
)

// TransactionsHandler handles transaction listing and creation
func TransactionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "GET" {
		// Load all transactions from all months
		transactions, err := services.LoadAllTransactions()
		if err != nil {
			utils.LogAudit(ip, "admin", "GET_TRANSACTIONS_FAILED", "all", false)
			http.Error(w, "Failed to load transactions", http.StatusInternalServerError)
			return
		}

		// Sort by date (newest first)
		for i := 0; i < len(transactions); i++ {
			for j := i + 1; j < len(transactions); j++ {
				if transactions[j].Date.After(transactions[i].Date) {
					transactions[i], transactions[j] = transactions[j], transactions[i]
				}
			}
		}

		utils.LogAudit(ip, "admin", "GET_TRANSACTIONS", "all", true)
		json.NewEncoder(w).Encode(transactions)
		
	} else if r.Method == "POST" {
		var req models.TransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.LogAudit(ip, "admin", "ADD_TRANSACTION_INVALID", "", false)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate amount
		if err := utils.ValidateAmount(req.Amount); err != nil {
			utils.LogAudit(ip, "admin", "ADD_TRANSACTION_INVALID_AMOUNT", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate and sanitize description
		sanitizedDesc, err := utils.ValidateAndSanitize(req.Description, 1000)
		if err != nil {
			utils.LogAudit(ip, "admin", "ADD_TRANSACTION_INVALID_DESC", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate accounts exist
		fromExists := services.AccountExists(req.From)
		toExists := services.AccountExists(req.To)

		if !fromExists || !toExists {
			utils.LogAudit(ip, "admin", "ADD_TRANSACTION_INVALID_ACCOUNT", "", false)
			http.Error(w, "Invalid account(s)", http.StatusBadRequest)
			return
		}

		// Create transaction
		var t models.Transaction
		if req.Date != "" && req.Time != "" {
			parsedTime, err := utils.FormatDateTime(req.Date, req.Time)
			if err != nil {
				utils.LogAudit(ip, "admin", "ADD_TRANSACTION_INVALID_DATETIME", "", false)
				http.Error(w, "Invalid date/time format", http.StatusBadRequest)
				return
			}
			t.Date = parsedTime
		} else {
			t.Date = time.Now()
		}

		t.ID = "TRAN" + t.Date.Format("020106150405")
		t.From = req.From
		t.To = req.To
		t.Description = sanitizedDesc
		t.Amount = req.Amount

		// If updating existing transaction, reverse it first
		if req.ID != "" && req.ID != t.ID {
			services.ReverseTransaction(req.ID)
			services.DeleteTransaction(req.ID)
		} else if req.ID != "" {
			services.ReverseTransaction(req.ID)
		}

		// Save transaction
		if err := services.SaveTransaction(t); err != nil {
			utils.LogAudit(ip, "admin", "ADD_TRANSACTION_FAILED", t.ID, false)
			http.Error(w, "Failed to save transaction", http.StatusInternalServerError)
			return
		}

		// Apply transaction to accounts
		if err := services.ApplyTransaction(t); err != nil {
			utils.LogAudit(ip, "admin", "APPLY_TRANSACTION_FAILED", t.ID, false)
			http.Error(w, "Failed to apply transaction", http.StatusInternalServerError)
			return
		}

		// Update monthly report
		services.UpdateMonthlyReport(t.Date)

		utils.LogAudit(ip, "admin", "ADD_TRANSACTION_SUCCESS", t.ID, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}

// TransactionDetailHandler handles individual transaction operations
func TransactionDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "DELETE" {
		id := strings.TrimPrefix(r.URL.Path, "/api/transactions/")
		if id == "" {
			http.Error(w, "Transaction ID required", http.StatusBadRequest)
			return
		}

		// Reverse the transaction
		if err := services.ReverseTransaction(id); err != nil {
			utils.LogAudit(ip, "admin", "REVERSE_TRANSACTION_FAILED", id, false)
		}

		// Delete the transaction
		if err := services.DeleteTransaction(id); err != nil {
			utils.LogAudit(ip, "admin", "DELETE_TRANSACTION_FAILED", id, false)
			http.Error(w, "Failed to delete transaction", http.StatusInternalServerError)
			return
		}

		// Update monthly report
		date := utils.ParseTransactionID(id)
		services.UpdateMonthlyReport(date)

		utils.LogAudit(ip, "admin", "DELETE_TRANSACTION", id, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}