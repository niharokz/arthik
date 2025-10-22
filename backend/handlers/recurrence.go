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

// RecurrenceHandler handles recurring transactions listing and creation
func RecurrenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "GET" {
		recurrences, err := services.LoadRecurrences()
		if err != nil {
			utils.LogAudit(ip, "admin", "GET_RECURRENCES_FAILED", "", false)
			http.Error(w, "Failed to load recurrences", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "GET_RECURRENCES", "", true)
		json.NewEncoder(w).Encode(recurrences)
		
	} else if r.Method == "POST" {
		var rec models.Recurrence
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			utils.LogAudit(ip, "admin", "ADD_RECURRENCE_INVALID", "", false)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate amount
		if err := utils.ValidateAmount(rec.Amount); err != nil {
			utils.LogAudit(ip, "admin", "ADD_RECURRENCE_INVALID_AMOUNT", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate description
		sanitizedDesc, err := utils.ValidateAndSanitize(rec.Description, 1000)
		if err != nil {
			utils.LogAudit(ip, "admin", "ADD_RECURRENCE_INVALID_DESC", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rec.Description = sanitizedDesc

		// Validate day of month
		if err := utils.ValidateDayOfMonth(rec.DayOfMonth); err != nil {
			utils.LogAudit(ip, "admin", "ADD_RECURRENCE_INVALID_DAY", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate ID if not provided
		if rec.ID == "" {
			rec.ID = utils.GenerateRecurrenceID()
		}

		// Save recurrence
		if err := services.SaveRecurrence(rec); err != nil {
			utils.LogAudit(ip, "admin", "ADD_RECURRENCE_FAILED", rec.ID, false)
			http.Error(w, "Failed to save recurrence", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "ADD_RECURRENCE_SUCCESS", rec.ID, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}

// RecurrenceDetailHandler handles individual recurrence operations
func RecurrenceDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "DELETE" {
		id := strings.TrimPrefix(r.URL.Path, "/api/recurrence/")
		if id == "" {
			http.Error(w, "Recurrence ID required", http.StatusBadRequest)
			return
		}

		if err := services.DeleteRecurrence(id); err != nil {
			utils.LogAudit(ip, "admin", "DELETE_RECURRENCE_FAILED", id, false)
			http.Error(w, "Failed to delete recurrence", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "DELETE_RECURRENCE", id, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}

// ApplyRecurrenceHandler handles applying a recurring transaction
func ApplyRecurrenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/recurrence/apply/")
	if id == "" {
		http.Error(w, "Recurrence ID required", http.StatusBadRequest)
		return
	}

	recurrences, err := services.LoadRecurrences()
	if err != nil {
		utils.LogAudit(ip, "admin", "APPLY_RECURRENCE_FAILED", id, false)
		http.Error(w, "Failed to load recurrences", http.StatusInternalServerError)
		return
	}

	// Find and apply the recurrence
	for _, rec := range recurrences {
		if rec.ID == id {
			// Create transaction
			t := models.Transaction{
				ID:          utils.GenerateTransactionID(),
				From:        rec.From,
				To:          rec.To,
				Description: rec.Description + " (Recurring)",
				Amount:      rec.Amount,
				Date:        time.Now(),
			}

			// Save and apply transaction
			if err := services.SaveTransaction(t); err != nil {
				utils.LogAudit(ip, "admin", "APPLY_RECURRENCE_SAVE_FAILED", id, false)
				http.Error(w, "Failed to save transaction", http.StatusInternalServerError)
				return
			}

			if err := services.ApplyTransaction(t); err != nil {
				utils.LogAudit(ip, "admin", "APPLY_RECURRENCE_APPLY_FAILED", id, false)
				http.Error(w, "Failed to apply transaction", http.StatusInternalServerError)
				return
			}

			// Update next date
			currentNext := rec.NextDate
			nextMonth := currentNext.AddDate(0, 1, 0)
			rec.NextDate = time.Date(nextMonth.Year(), nextMonth.Month(), rec.DayOfMonth, 0, 0, 0, 0, time.Local)
			
			if err := services.SaveRecurrence(rec); err != nil {
				utils.LogAudit(ip, "admin", "UPDATE_RECURRENCE_FAILED", id, false)
			}

			// Update monthly report
			services.UpdateMonthlyReport(t.Date)

			utils.LogAudit(ip, "admin", "APPLY_RECURRENCE", id, true)
			json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
			return
		}
	}

	http.Error(w, "Recurrence not found", http.StatusNotFound)
}