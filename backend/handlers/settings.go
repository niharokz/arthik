package handlers

import (
	"encoding/json"
	"net/http"

	"arthik/middleware"
	"arthik/models"
	"arthik/services"
	"arthik/utils"
)

// SettingsHandler handles settings retrieval and updates
func SettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "GET" {
		settings := services.GetSettings()
		response := map[string]string{
			"theme":      settings.Theme,
			"dateFormat": settings.DateFormat,
		}
		utils.LogAudit(ip, "admin", "GET_SETTINGS", "", true)
		json.NewEncoder(w).Encode(response)
		
	} else if r.Method == "POST" {
		var req models.SettingsUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.LogAudit(ip, "admin", "UPDATE_SETTINGS_INVALID", "", false)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		settings := services.GetSettings()

		// Update theme
		if req.Theme != "" {
			if req.Theme == "light" || req.Theme == "dark" {
				settings.Theme = req.Theme
			}
		}

		// Update date format
		if req.DateFormat != "" {
			settings.DateFormat = req.DateFormat
		}

		// Handle password change
		if req.OldPassword != "" && req.NewPassword != "" {
			// Verify old password
			if !middleware.VerifyPassword(req.OldPassword, settings.PasswordHash) {
				utils.LogAudit(ip, "admin", "CHANGE_PASSWORD_FAILED", "invalid_old_password", false)
				json.NewEncoder(w).Encode(models.GenericResponse{
					Success: false,
					Error:   "Invalid old password",
				})
				return
			}

			// Validate new password length
			if err := utils.ValidatePassword(req.NewPassword); err != nil {
				utils.LogAudit(ip, "admin", "CHANGE_PASSWORD_FAILED", "weak_password", false)
				json.NewEncoder(w).Encode(models.GenericResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}

			// Hash new password
			hash, err := middleware.HashPassword(req.NewPassword)
			if err != nil {
				utils.LogAudit(ip, "admin", "CHANGE_PASSWORD_FAILED", "hash_error", false)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			settings.PasswordHash = hash
			utils.LogAudit(ip, "admin", "CHANGE_PASSWORD_SUCCESS", "", true)
		}

		// Save settings
		if err := services.SaveSettings(settings); err != nil {
			utils.LogAudit(ip, "admin", "UPDATE_SETTINGS_FAILED", "", false)
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "UPDATE_SETTINGS_SUCCESS", "", true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}