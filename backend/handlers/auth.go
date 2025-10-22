package handlers

import (
	"encoding/json"
	"net/http"

	"arthik/middleware"
	"arthik/models"
	"arthik/services"
	"arthik/utils"
)

// LoginHandler handles user login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := utils.GetClientIP(r)

	// Check brute force protection
	if !middleware.CheckLoginAttempts(ip) {
		utils.LogAudit(ip, "unknown", "LOGIN_BLOCKED", "brute_force", false)
		http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.LogAudit(ip, "unknown", "LOGIN_INVALID_REQUEST", "", false)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate password input
	if req.Password == "" {
		utils.LogAudit(ip, "unknown", "LOGIN_EMPTY_PASSWORD", "", false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.LoginResponse{
			Success: false,
			Message: "Password is required",
		})
		return
	}

	// Get settings to verify password
	settings := services.GetSettings()
	success := middleware.VerifyPassword(req.Password, settings.PasswordHash)

	if success {
		// Generate JWT token
		token, err := middleware.GenerateToken()
		if err != nil {
			utils.LogAudit(ip, "admin", "TOKEN_GENERATION_FAILED", "", false)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		middleware.ResetLoginAttempts(ip)
		utils.LogAudit(ip, "admin", "LOGIN_SUCCESS", "", true)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.LoginResponse{
			Success: true,
			Token:   token,
		})
	} else {
		utils.LogAudit(ip, "unknown", "LOGIN_FAILED", "invalid_password", false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.LoginResponse{
			Success: false,
			Message: "Invalid password",
		})
	}
}

// HomeHandler serves the main HTML page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../frontend/index.html")
}
