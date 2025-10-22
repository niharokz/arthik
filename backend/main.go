package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"arthik/config"
	"arthik/handlers"
	"arthik/middleware"
	"arthik/services"
)

func main() {
	// Initialize directories
	initDirectories()

	// Initialize settings and load data
	if err := services.InitializeSettings(); err != nil {
		log.Fatal("Failed to initialize settings:", err)
	}

	settings := services.GetSettings()
	middleware.SetJWTSecret([]byte(settings.EncryptionKey))

	// Load accounts
	if err := services.LoadAccounts(); err != nil {
		log.Println("No accounts found, will create defaults")
		services.InitializeDefaultAccounts()
	}

	// Start cleanup goroutine for rate limiters
	go middleware.CleanupRateLimiters()

	// Setup routes with middleware chain
	setupRoutes()

	// Start server
	fmt.Println("🚀 arthik.app starting on http://localhost:8081")
//	fmt.Println("⚠️  WARNING: Using HTTP. For production, use HTTPS with TLS certificates")
	log.Fatal(http.ListenAndServe(config.ServerPort, nil))
}

func initDirectories() {
	os.MkdirAll(config.LedgerDir, 0755)
	os.MkdirAll(config.NotesDir, 0755)
}

func setupRoutes() {
	// Serve static files from frontend directory
	http.HandleFunc("/styles.css", serveStaticFile("styles.css", "text/css"))
	http.HandleFunc("/js/", handleJSFiles)
	
	// Home page
	http.HandleFunc("/", middleware.SecurityHeaders(handlers.HomeHandler))

	// Public routes
	http.HandleFunc("/api/login", 
		middleware.SecurityHeaders(
			middleware.RateLimit(handlers.LoginHandler)))

	// Protected routes
	http.HandleFunc("/api/accounts", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.AccountsHandler))))

	http.HandleFunc("/api/accounts/", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.AccountDetailHandler))))

	http.HandleFunc("/api/transactions", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.TransactionsHandler))))

	http.HandleFunc("/api/transactions/", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.TransactionDetailHandler))))

	http.HandleFunc("/api/dashboard", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.DashboardHandler))))

	http.HandleFunc("/api/settings", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.SettingsHandler))))

	http.HandleFunc("/api/recurrence", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.RecurrenceHandler))))

	http.HandleFunc("/api/recurrence/", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.RecurrenceDetailHandler))))

	http.HandleFunc("/api/recurrence/apply/", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.ApplyRecurrenceHandler))))

	http.HandleFunc("/api/notes", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.NotesHandler))))

	http.HandleFunc("/api/notes/", 
		middleware.SecurityHeaders(
			middleware.RateLimit(
				middleware.Auth(handlers.NoteDetailHandler))))
}

func handleJSFiles(w http.ResponseWriter, r *http.Request) {
	// Get the file path (remove leading /)
	path := r.URL.Path[1:] // removes the leading /
	fullPath := "../frontend/" + path
	
	// Set correct MIME type
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	
	// Serve the file
	http.ServeFile(w, r, fullPath)
}

func serveStaticFile(filename, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, "../frontend/"+filename)
	}
}
