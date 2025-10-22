package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"arthik/models"
	"arthik/services"
	"arthik/utils"
)

// AccountsHandler handles account listing and creation
func AccountsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "GET" {
		accounts := services.GetAllAccounts()
		utils.LogAudit(ip, "admin", "GET_ACCOUNTS", "", true)
		json.NewEncoder(w).Encode(accounts)
		
	} else if r.Method == "POST" {
		var acc models.Account
		if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
			utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID", "", false)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate account name
		if err := utils.ValidateAccountName(acc.Name); err != nil {
			utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID_NAME", acc.Name, false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Sanitize name
		sanitizedName, err := utils.ValidateAndSanitize(acc.Name, 500)
		if err != nil {
			utils.LogAudit(ip, "admin", "ADD_ACCOUNT_SANITIZE_ERROR", acc.Name, false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		acc.Name = sanitizedName

		// Validate category
		if err := utils.ValidateCategory(acc.Category); err != nil {
			utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID_CATEGORY", acc.Name, false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate budget if provided
		if acc.Budget > 0 {
			if err := utils.ValidateAmount(acc.Budget); err != nil {
				utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID_BUDGET", acc.Name, false)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Validate dates if provided
		if acc.DueDate != "" {
			if err := utils.ValidateDate(acc.DueDate); err != nil {
				utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID_DATE", acc.Name, false)
				http.Error(w, "Invalid due date format", http.StatusBadRequest)
				return
			}
		}

		if acc.LastPaymentDate != "" {
			if err := utils.ValidateDate(acc.LastPaymentDate); err != nil {
				utils.LogAudit(ip, "admin", "ADD_ACCOUNT_INVALID_DATE", acc.Name, false)
				http.Error(w, "Invalid last payment date format", http.StatusBadRequest)
				return
			}
		}

		// Add account
		if err := services.AddAccount(acc); err != nil {
			utils.LogAudit(ip, "admin", "ADD_ACCOUNT_FAILED", acc.Name, false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		utils.LogAudit(ip, "admin", "ADD_ACCOUNT_SUCCESS", acc.Name, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}

// AccountDetailHandler handles individual account operations
func AccountDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "DELETE" {
		name := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
		name, _ = url.QueryUnescape(name)
		
		if name == "" {
			http.Error(w, "Account name required", http.StatusBadRequest)
			return
		}

		if err := services.DeleteAccount(name); err != nil {
			utils.LogAudit(ip, "admin", "DELETE_ACCOUNT_FAILED", name, false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		utils.LogAudit(ip, "admin", "DELETE_ACCOUNT", name, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}