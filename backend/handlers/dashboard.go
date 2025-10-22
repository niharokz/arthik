package handlers

import (
	"encoding/json"
	"net/http"

	"arthik/services"
	"arthik/utils"
)

// DashboardHandler handles dashboard statistics requests
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	stats, err := services.CalculateDashboardStats()
	if err != nil {
		utils.LogAudit(ip, "admin", "GET_DASHBOARD_FAILED", "", false)
		http.Error(w, "Failed to calculate dashboard stats", http.StatusInternalServerError)
		return
	}

	utils.LogAudit(ip, "admin", "GET_DASHBOARD", "", true)
	json.NewEncoder(w).Encode(stats)
}