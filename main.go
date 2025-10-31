package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DATA_DIR              = "./data"
	LOG_DIR               = "./logs"
	SESSION_TIMEOUT       = 30 * time.Minute
	MAX_LOGIN_ATTEMPTS    = 5
	LOGIN_LOCKOUT_MINUTES = 15
	MAX_REQUEST_SIZE      = 10 * 1024 * 1024 // 10MB
	CSRF_TOKEN_LENGTH     = 32
)

var (
	PASSWORD_HASH     string
	READONLY_PASSWORD string
	sessions          = make(map[string]*Session)
	sessionMutex      sync.RWMutex
	loginAttempts     = make(map[string]*LoginAttempt)
	loginAttemptsMux  sync.RWMutex
	fileMutex         sync.Mutex
	csrfTokens        = make(map[string]time.Time)
	csrfMutex         sync.RWMutex
	readOnlyMode      = false
)

type Session struct {
	Token      string
	CreatedAt  time.Time
	LastAccess time.Time
	CSRFToken  string
}

type LoginAttempt struct {
	Count      int
	LastAttempt time.Time
}

type Transaction struct {
	TranDate    string  `json:"tranDate"`
	TranTime    string  `json:"tranTime"`
	From        string  `json:"from"`
	To          string  `json:"to"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

type Account struct {
	Name    string  `json:"account"`
	Type    string  `json:"type"`
	Amount  float64 `json:"amount"`
	IINW    string  `json:"iinw"`
	Budget  float64 `json:"budget"`
	DueDate string  `json:"dueDate"`
}

type Record struct {
	Date        string  `json:"date"`
	NetWorth    float64 `json:"netWorth"`
	Assets      float64 `json:"assets"`
	Liabilities float64 `json:"liabilities"`
	Expenses    float64 `json:"expenses"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	// Command line flags
	passwordFlag := flag.String("p", "", "Set password for login")
	readOnlyFlag := flag.Bool("r", false, "Run in read-only mode (no edits allowed)")
	flag.Parse()

	// Handle password flag
	if *passwordFlag != "" {
		READONLY_PASSWORD = *passwordFlag
		hasher := sha256.New()
		hasher.Write([]byte(*passwordFlag))
		PASSWORD_HASH = hex.EncodeToString(hasher.Sum(nil))
		log.Printf("Using password from command line flag")
	} else {
		// Load password hash from environment variable
		PASSWORD_HASH = os.Getenv("ARTHIK_PASSWORD_HASH")
		if PASSWORD_HASH == "" {
			// Default for development only - CHANGE IN PRODUCTION
			PASSWORD_HASH = "240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9" // admin123
			log.Println("WARNING: Using default password. Set ARTHIK_PASSWORD_HASH environment variable in production!")
		}
	}

	// Handle read-only mode
	if *readOnlyFlag {
		readOnlyMode = true
		log.Println("Running in READ-ONLY mode - no modifications allowed")
	}

	initDirectories()
	initializeData()
	go startDailyBatch()
	go cleanupSessions()
	go cleanupLoginAttempts()

	// Setup routes with middleware
	mux := http.NewServeMux()
	
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", requireAuth(handleLogout))
	mux.HandleFunc("/api/dashboard", requireAuth(handleDashboard))
	mux.HandleFunc("/api/transactions", requireAuth(handleTransactions))
	mux.HandleFunc("/api/accounts", requireAuth(handleAccounts))
	mux.HandleFunc("/api/settings", requireAuth(handleSettings))
	mux.HandleFunc("/api/readonly-info", handleReadonlyInfo)
	mux.HandleFunc("/health", handleHealth)

	// Static files
	fs := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fs)

	// Wrap with security middleware
	handler := securityHeaders(limitRequestSize(mux))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Server starting on :8080")
	log.Fatal(server.ListenAndServe())
}

// Security middleware
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' https://cdn.jsdelivr.net;  script-src 'self' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:;")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		// CORS - restrict to specific origin in production
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "http://localhost:8080" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func limitRequestSize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MAX_REQUEST_SIZE)
		next.ServeHTTP(w, r)
	})
}

func requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			respondError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionMutex.RLock()
		session, exists := sessions[cookie.Value]
		sessionMutex.RUnlock()

		if !exists {
			respondError(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		// Check session timeout
		if time.Since(session.LastAccess) > SESSION_TIMEOUT {
			sessionMutex.Lock()
			delete(sessions, cookie.Value)
			sessionMutex.Unlock()
			respondError(w, "Session expired", http.StatusUnauthorized)
			return
		}

		// Verify CSRF token for state-changing operations
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			// Check if read-only mode
			if readOnlyMode {
				respondError(w, "Application is in read-only mode", http.StatusForbidden)
				return
			}

			csrfToken := r.Header.Get("X-CSRF-Token")
			if csrfToken == "" || csrfToken != session.CSRFToken {
				respondError(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		// Update last access time
		sessionMutex.Lock()
		session.LastAccess = time.Now()
		sessionMutex.Unlock()

		handler(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func handleReadonlyInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	response := map[string]interface{}{
		"readOnlyMode": readOnlyMode,
	}
	
	if readOnlyMode && READONLY_PASSWORD != "" {
		response["password"] = READONLY_PASSWORD
	}
	
	json.NewEncoder(w).Encode(response)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get client IP for rate limiting
	clientIP := getClientIP(r)

	// Check rate limiting
	loginAttemptsMux.Lock()
	attempt, exists := loginAttempts[clientIP]
	if exists {
		if attempt.Count >= MAX_LOGIN_ATTEMPTS {
			if time.Since(attempt.LastAttempt) < LOGIN_LOCKOUT_MINUTES*time.Minute {
				loginAttemptsMux.Unlock()
				logSecurityEvent("LOGIN_LOCKED", clientIP, "Too many failed attempts")
				respondError(w, "Too many login attempts. Try again later.", http.StatusTooManyRequests)
				return
			} else {
				// Reset after lockout period
				attempt.Count = 0
			}
		}
	} else {
		loginAttempts[clientIP] = &LoginAttempt{}
		attempt = loginAttempts[clientIP]
	}
	loginAttemptsMux.Unlock()

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		respondError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	password := data["password"]
	if password == "" {
		respondError(w, "Password required", http.StatusBadRequest)
		return
	}

	// Hash the provided password
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(hash), []byte(PASSWORD_HASH)) != 1 {
		loginAttemptsMux.Lock()
		attempt.Count++
		attempt.LastAttempt = time.Now()
		loginAttemptsMux.Unlock()

		logSecurityEvent("LOGIN_FAILED", clientIP, "Invalid password")
		respondError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Reset login attempts on successful login
	loginAttemptsMux.Lock()
	delete(loginAttempts, clientIP)
	loginAttemptsMux.Unlock()

	// Create session
	sessionToken, err := generateSecureToken(32)
	if err != nil {
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	csrfToken, err := generateSecureToken(CSRF_TOKEN_LENGTH)
	if err != nil {
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	session := &Session{
		Token:      sessionToken,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		CSRFToken:  csrfToken,
	}

	sessionMutex.Lock()
	sessions[sessionToken] = session
	sessionMutex.Unlock()

	// Set httpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(SESSION_TIMEOUT.Seconds()),
	})

	logSecurityEvent("LOGIN_SUCCESS", clientIP, "User logged in")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"csrfToken": csrfToken,
		"readOnly":  readOnlyMode,
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_token")
	if err == nil {
		sessionMutex.Lock()
		delete(sessions, cookie.Value)
		sessionMutex.Unlock()

		logSecurityEvent("LOGOUT", getClientIP(r), "User logged out")
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get CSRF token from session
	cookie, err := r.Cookie("session_token")
	if err != nil {
		respondError(w, "Session not found", http.StatusUnauthorized)
		return
	}

	sessionMutex.RLock()
	session, exists := sessions[cookie.Value]
	sessionMutex.RUnlock()

	if !exists {
		respondError(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	accounts, err := readAccounts()
	if err != nil {
		log.Printf("Error reading accounts: %v", err)
		respondError(w, "Failed to load accounts", http.StatusInternalServerError)
		return
	}

	records, err := readRecords()
	if err != nil {
		log.Printf("Error reading records: %v", err)
		respondError(w, "Failed to load records", http.StatusInternalServerError)
		return
	}

	transactions, err := readAllTransactions()
	if err != nil {
		log.Printf("Error reading transactions: %v", err)
		respondError(w, "Failed to load transactions", http.StatusInternalServerError)
		return
	}

	// Sort accounts by usage
	accounts = sortAccountsByUsage(accounts, transactions)

	netWorth := 0.0
	assets := 0.0
	liabilities := 0.0

	for _, acc := range accounts {
		if acc.IINW == "Yes" {
			if acc.Type == "ASSET" {
				netWorth += acc.Amount
				assets += acc.Amount
			} else if acc.Type == "LIABILITIES" {
				netWorth += acc.Amount  // acc.Amount is already negative
				liabilities += acc.Amount
			}
		}
	}

	currentMonth := time.Now().Format("01-2006")
	budgetData := calculateBudget(transactions, accounts, currentMonth)
	upcomingBills := getUpcomingBills(accounts)

	response := map[string]interface{}{
		"netWorth":      netWorth,
		"assets":        assets,
		"liabilities":   liabilities,
		"records":       records,
		"accounts":      accounts,
		"budget":        budgetData,
		"upcomingBills": upcomingBills,
		"csrfToken":     session.CSRFToken,
		"readOnly":      readOnlyMode,
	}

	json.NewEncoder(w).Encode(response)
}

func handleTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			var err error
			page, err = strconv.Atoi(p)
			if err != nil || page < 1 {
				page = 1
			}
		}

		// Limit page size
		pageSize := 30
		if page > 1000 {
			respondError(w, "Page number too large", http.StatusBadRequest)
			return
		}

		transactions, err := readAllTransactions()
		if err != nil {
			respondError(w, "Failed to load transactions", http.StatusInternalServerError)
			return
		}

		start := (page - 1) * pageSize
		end := start + pageSize
		if start > len(transactions) {
			start = len(transactions)
		}
		if end > len(transactions) {
			end = len(transactions)
		}
		result := transactions[start:end]

		json.NewEncoder(w).Encode(map[string]interface{}{
			"transactions": result,
			"total":        len(transactions),
			"page":         page,
		})

	case http.MethodPost:
		var tran Transaction
		if err := json.NewDecoder(r.Body).Decode(&tran); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		if err := validateTransaction(&tran); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := addTransaction(tran); err != nil {
			respondError(w, "Failed to add transaction", http.StatusInternalServerError)
			return
		}

		if err := recalculateAllData(); err != nil {
			log.Printf("Error recalculating data: %v", err)
		}

		logSecurityEvent("TRANSACTION_ADD", getClientIP(r), fmt.Sprintf("Added transaction: %s", tran.Description))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodPut:
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		if err := updateTransaction(data); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := recalculateAllData(); err != nil {
			log.Printf("Error recalculating data: %v", err)
		}

		logSecurityEvent("TRANSACTION_UPDATE", getClientIP(r), "Updated transaction")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		tranDate := sanitizeInput(data["tranDate"])
		tranTime := sanitizeInput(data["tranTime"])

		if tranDate == "" || tranTime == "" {
			respondError(w, "Transaction date and time required", http.StatusBadRequest)
			return
		}

		if err := deleteTransaction(tranDate, tranTime); err != nil {
			respondError(w, "Failed to delete transaction", http.StatusInternalServerError)
			return
		}

		if err := recalculateAllData(); err != nil {
			log.Printf("Error recalculating data: %v", err)
		}

		logSecurityEvent("TRANSACTION_DELETE", getClientIP(r), fmt.Sprintf("Deleted transaction: %s %s", tranDate, tranTime))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		accounts, err := readAccounts()
		if err != nil {
			respondError(w, "Failed to load accounts", http.StatusInternalServerError)
			return
		}
		
		// Sort accounts by usage
		transactions, err := readAllTransactions()
		if err == nil {
			accounts = sortAccountsByUsage(accounts, transactions)
		}
		
		json.NewEncoder(w).Encode(accounts)

	case http.MethodPost:
		var acc Account
		if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		if err := validateAccount(&acc); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := addAccount(acc); err != nil {
			respondError(w, "Failed to add account", http.StatusInternalServerError)
			return
		}

		logSecurityEvent("ACCOUNT_ADD", getClientIP(r), fmt.Sprintf("Added account: %s", acc.Name))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodPut:
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		// Get old account name for name change support
		oldAccount := ""
		if oldAccountVal, ok := data["oldAccount"].(string); ok {
			oldAccount = sanitizeInput(oldAccountVal)
		}
		
		// If oldAccount is not provided, use the account field (backward compatibility)
		if oldAccount == "" {
			if accountVal, ok := data["account"].(string); ok {
				oldAccount = sanitizeInput(accountVal)
			}
		}

		// Build account struct from data
		acc := Account{}
		if accountVal, ok := data["account"].(string); ok {
			acc.Name = sanitizeInput(accountVal)
		}
		if typeVal, ok := data["type"].(string); ok {
			acc.Type = sanitizeInput(typeVal)
		}
		if amountVal, ok := data["amount"].(float64); ok {
			acc.Amount = amountVal
		}
		if iinwVal, ok := data["iinw"].(string); ok {
			acc.IINW = sanitizeInput(iinwVal)
		}
		if budgetVal, ok := data["budget"].(float64); ok {
			acc.Budget = budgetVal
		}
		if dueDateVal, ok := data["dueDate"].(string); ok {
			acc.DueDate = sanitizeInput(dueDateVal)
		}

		if err := validateAccount(&acc); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update account (with support for name change)
		if err := updateAccountWithNameChange(oldAccount, acc); err != nil {
			respondError(w, "Failed to update account", http.StatusInternalServerError)
			return
		}

		logSecurityEvent("ACCOUNT_UPDATE", getClientIP(r), fmt.Sprintf("Updated account: %s", acc.Name))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			respondError(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		accountName := sanitizeInput(data["account"])
		if accountName == "" {
			respondError(w, "Account name required", http.StatusBadRequest)
			return
		}

		if err := deleteAccount(accountName); err != nil {
			respondError(w, "Failed to delete account", http.StatusInternalServerError)
			return
		}

		logSecurityEvent("ACCOUNT_DELETE", getClientIP(r), fmt.Sprintf("Deleted account: %s", accountName))
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		respondError(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	newPassword := data["newPassword"]
	if newPassword == "" {
		respondError(w, "New password required", http.StatusBadRequest)
		return
	}

	if len(newPassword) < 8 {
		respondError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Hash new password
	hasher := sha256.New()
	hasher.Write([]byte(newPassword))
	newHash := hex.EncodeToString(hasher.Sum(nil))

	// Update password hash
	PASSWORD_HASH = newHash

	// Invalidate all sessions except current
	cookie, _ := r.Cookie("session_token")
	currentSession := ""
	if cookie != nil {
		currentSession = cookie.Value
	}

	sessionMutex.Lock()
	for token := range sessions {
		if token != currentSession {
			delete(sessions, token)
		}
	}
	sessionMutex.Unlock()

	logSecurityEvent("PASSWORD_CHANGE", getClientIP(r), "Password changed successfully")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password changed. Please update ARTHIK_PASSWORD_HASH environment variable to: " + newHash,
	})
}

// Validation functions
func validateTransaction(t *Transaction) error {
	if t.TranDate == "" || t.TranTime == "" {
		return errors.New("transaction date and time required")
	}

	if !isValidDate(t.TranDate) {
		return errors.New("invalid date format (use DD-MM-YYYY)")
	}

	if !isValidTime(t.TranTime) {
		return errors.New("invalid time format (use HH:MM)")
	}

	t.From = sanitizeInput(t.From)
	t.To = sanitizeInput(t.To)
	t.Description = sanitizeInput(t.Description)

	if t.From == "" || t.To == "" {
		return errors.New("from and to accounts required")
	}

	if len(t.Description) > 100 {
		return errors.New("description too long (max 100 characters)")
	}

	if t.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if t.Amount > 999999999.99 {
		return errors.New("amount too large")
	}

	return nil
}

func validateAccount(a *Account) error {
	a.Name = sanitizeInput(a.Name)
	a.Type = sanitizeInput(a.Type)
	a.IINW = sanitizeInput(a.IINW)
	a.DueDate = sanitizeInput(a.DueDate)

	if a.Name == "" {
		return errors.New("account name required")
	}

	if len(a.Name) > 50 {
		return errors.New("account name too long (max 50 characters)")
	}

	validTypes := map[string]bool{"ASSET": true, "LIABILITIES": true, "INCOME": true, "EXPENSE": true}
	if !validTypes[a.Type] {
		return errors.New("invalid account type")
	}

	if a.IINW != "Yes" && a.IINW != "No" {
		return errors.New("invalid IINW value")
	}

	if a.DueDate != "" && !isValidDate(a.DueDate) {
		return errors.New("invalid due date format (use DD-MM-YYYY)")
	}

	if a.Amount < -999999999.99 || a.Amount > 999999999.99 {
		return errors.New("amount out of range")
	}

	if a.Budget < 0 || a.Budget > 999999999.99 {
		return errors.New("budget out of range")
	}

	return nil
}

func isValidDate(date string) bool {
	matched, _ := regexp.MatchString(`^\d{2}-\d{2}-\d{4}$`, date)
	if !matched {
		return false
	}

	parts := strings.Split(date, "-")
	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])

	if month < 1 || month > 12 || day < 1 || day > 31 || year < 2000 || year > 2100 {
		return false
	}

	return true
}

func isValidTime(timeStr string) bool {
	matched, _ := regexp.MatchString(`^\d{2}:\d{2}$`, timeStr)
	if !matched {
		return false
	}

	parts := strings.Split(timeStr, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])

	return hour >= 0 && hour < 24 && minute >= 0 && minute < 60
}

func sanitizeInput(input string) string {
	// Remove control characters and trim
	input = strings.TrimSpace(input)
	
	// HTML escape to prevent XSS
	input = html.EscapeString(input)
	
	// Prevent CSV injection
	if len(input) > 0 {
		firstChar := input[0]
		if firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@' {
			input = "'" + input
		}
	}
	
	return input
}

// File operations with proper error handling and locking
func initDirectories() {
	if err := os.MkdirAll(DATA_DIR, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	if err := os.MkdirAll(LOG_DIR, 0700); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}
}

func initializeData() {
	accountPath := filepath.Join(DATA_DIR, "account.csv")
	if _, err := os.Stat(accountPath); os.IsNotExist(err) {
		file, err := os.OpenFile(accountPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("Failed to create account file: %v", err)
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		writer.Write([]string{"Account", "Type", "Amount", "IINW", "Budget", "DueDate"})
		writer.Write([]string{"Salary", "INCOME", "-1000", "No", "0", ""})
		writer.Write([]string{"ICICIBank", "ASSET", "950", "Yes", "0", ""})
		writer.Write([]string{"Food", "EXPENSE", "50", "No", "500", ""})
		writer.Flush()
	}

	tranPath := filepath.Join(DATA_DIR, "tran_2025.csv")
	if _, err := os.Stat(tranPath); os.IsNotExist(err) {
		file, err := os.OpenFile(tranPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("Failed to create transaction file: %v", err)
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		writer.Write([]string{"TranDate", "TranTime", "From", "To", "Description", "Amount"})
		writer.Write([]string{"28-10-2025", "13:00", "Salary", "ICICIBank", "SalaryCredit", "1000"})
		writer.Write([]string{"29-10-2025", "17:00", "ICICIBank", "Food", "Dinner", "50"})
		writer.Flush()
	}

	recordPath := filepath.Join(DATA_DIR, "record.csv")
	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		file, err := os.OpenFile(recordPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("Failed to create record file: %v", err)
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		writer.Write([]string{"Date", "NetWorth", "Assets", "Liabilities", "Expenses"})
		writer.Flush()
	}

	if err := recalculateAllData(); err != nil {
		log.Printf("Error during initial calculation: %v", err)
	}
}

func readAccounts() ([]Account, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Open(filepath.Join(DATA_DIR, "account.csv"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var accounts []Account
	for i, record := range records {
		if i == 0 || len(record) < 6 {
			continue
		}
		amount, _ := strconv.ParseFloat(record[2], 64)
		budget, _ := strconv.ParseFloat(record[4], 64)
		accounts = append(accounts, Account{
			Name:    record[0],
			Type:    record[1],
			Amount:  amount,
			IINW:    record[3],
			Budget:  budget,
			DueDate: record[5],
		})
	}
	return accounts, nil
}

func sortAccountsByUsage(accounts []Account, transactions []Transaction) []Account {
	usageCount := make(map[string]int)
	
	// Count usage of each account in transactions
	for _, tran := range transactions {
		usageCount[tran.From]++
		usageCount[tran.To]++
	}
	
	// Sort accounts by usage (highest first)
	sort.Slice(accounts, func(i, j int) bool {
		return usageCount[accounts[i].Name] > usageCount[accounts[j].Name]
	})
	
	return accounts
}

func addAccount(acc Account) error {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.OpenFile(filepath.Join(DATA_DIR, "account.csv"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	err = writer.Write([]string{
		acc.Name,
		acc.Type,
		fmt.Sprintf("%.2f", acc.Amount),
		acc.IINW,
		fmt.Sprintf("%.2f", acc.Budget),
		acc.DueDate,
	})
	if err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func updateAccount(acc Account) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}

	// Update the account
	for i := range accounts {
		if accounts[i].Name == acc.Name {
			accounts[i] = acc
			break
		}
	}
	
	// Read all transactions to calculate usage and sort
	transactions, err := readAllTransactions()
	if err == nil {
		accounts = sortAccountsByUsage(accounts, transactions)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filepath.Join(DATA_DIR, "account.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Account", "Type", "Amount", "IINW", "Budget", "DueDate"})

	for _, a := range accounts {
		writer.Write([]string{
			a.Name,
			a.Type,
			fmt.Sprintf("%.2f", a.Amount),
			a.IINW,
			fmt.Sprintf("%.2f", a.Budget),
			a.DueDate,
		})
	}
	writer.Flush()
	return writer.Error()
}

func updateAccountWithNameChange(oldName string, acc Account) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}

	// Check if new name already exists (only if name is being changed)
	if oldName != acc.Name {
		for _, a := range accounts {
			if a.Name == acc.Name {
				return errors.New("Account with new name already exists")
			}
		}
	}

	// Update the account
	found := false
	for i := range accounts {
		if accounts[i].Name == oldName {
			accounts[i] = acc
			found = true
			break
		}
	}
	
	if !found {
		return errors.New("Account not found")
	}
	
	// Read all transactions to calculate usage and sort
	transactions, err := readAllTransactions()
	if err == nil {
		accounts = sortAccountsByUsage(accounts, transactions)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filepath.Join(DATA_DIR, "account.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Account", "Type", "Amount", "IINW", "Budget", "DueDate"})

	for _, a := range accounts {
		writer.Write([]string{
			a.Name,
			a.Type,
			fmt.Sprintf("%.2f", a.Amount),
			a.IINW,
			fmt.Sprintf("%.2f", a.Budget),
			a.DueDate,
		})
	}
	writer.Flush()
	
	return writer.Error()
}

func deleteAccount(name string) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}

	// Filter out the account to delete
	var filteredAccounts []Account
	for _, a := range accounts {
		if a.Name != name {
			filteredAccounts = append(filteredAccounts, a)
		}
	}
	
	// Read all transactions to calculate usage and sort
	transactions, err := readAllTransactions()
	if err == nil {
		filteredAccounts = sortAccountsByUsage(filteredAccounts, transactions)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filepath.Join(DATA_DIR, "account.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Account", "Type", "Amount", "IINW", "Budget", "DueDate"})

	for _, a := range filteredAccounts {
		writer.Write([]string{
			a.Name,
			a.Type,
			fmt.Sprintf("%.2f", a.Amount),
			a.IINW,
			fmt.Sprintf("%.2f", a.Budget),
			a.DueDate,
		})
	}
	writer.Flush()
	return writer.Error()
}

func readAllTransactions() ([]Transaction, error) {
	var allTransactions []Transaction

	files, err := filepath.Glob(filepath.Join(DATA_DIR, "tran_*.csv"))
	if err != nil {
		return nil, err
	}

	for _, filePath := range files {
		transactions, err := readTransactionsFromFile(filePath)
		if err != nil {
			log.Printf("Error reading file %s: %v", filePath, err)
			continue
		}
		allTransactions = append(allTransactions, transactions...)
	}

	sort.Slice(allTransactions, func(i, j int) bool {
		if allTransactions[i].TranDate == allTransactions[j].TranDate {
			return allTransactions[i].TranTime > allTransactions[j].TranTime
		}
		return compareDates(allTransactions[i].TranDate, allTransactions[j].TranDate)
	})

	return allTransactions, nil
}

func addTransaction(tran Transaction) error {
	year := tran.TranDate[6:10]
	filePath := filepath.Join(DATA_DIR, "tran_"+year+".csv")

	// Read existing transactions from the file
	existingTransactions, err := readTransactionsFromFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Add new transaction
	existingTransactions = append(existingTransactions, tran)

	// Sort transactions by date and time (newest first)
	sort.Slice(existingTransactions, func(i, j int) bool {
		if existingTransactions[i].TranDate == existingTransactions[j].TranDate {
			return existingTransactions[i].TranTime > existingTransactions[j].TranTime
		}
		return compareDates(existingTransactions[i].TranDate, existingTransactions[j].TranDate)
	})

	// Write all transactions back to file
	return writeTransactionsToFile(filePath, existingTransactions)
}

func updateTransaction(data map[string]interface{}) error {
	oldDate, ok := data["oldTranDate"].(string)
	if !ok {
		return errors.New("invalid oldTranDate")
	}
	oldTime, ok := data["oldTranTime"].(string)
	if !ok {
		return errors.New("invalid oldTranTime")
	}

	newDate, ok := data["tranDate"].(string)
	if !ok {
		return errors.New("invalid tranDate")
	}
	newTime, ok := data["tranTime"].(string)
	if !ok {
		return errors.New("invalid tranTime")
	}
	from, ok := data["from"].(string)
	if !ok {
		return errors.New("invalid from")
	}
	to, ok := data["to"].(string)
	if !ok {
		return errors.New("invalid to")
	}
	description, ok := data["description"].(string)
	if !ok {
		return errors.New("invalid description")
	}
	amount, ok := data["amount"].(float64)
	if !ok {
		return errors.New("invalid amount")
	}

	if err := deleteTransaction(oldDate, oldTime); err != nil {
		return err
	}

	tran := Transaction{
		TranDate:    newDate,
		TranTime:    newTime,
		From:        from,
		To:          to,
		Description: description,
		Amount:      amount,
	}

	if err := validateTransaction(&tran); err != nil {
		return err
	}

	return addTransaction(tran)
}

func deleteTransaction(date, time string) error {
	if len(date) < 10 {
		return errors.New("invalid date format")
	}

	year := date[6:10]
	filePath := filepath.Join(DATA_DIR, "tran_"+year+".csv")

	transactions, err := readTransactionsFromFile(filePath)
	if err != nil {
		return err
	}

	var filtered []Transaction
	for _, t := range transactions {
		if !(t.TranDate == date && t.TranTime == time) {
			filtered = append(filtered, t)
		}
	}

	return writeTransactionsToFile(filePath, filtered)
}

func readTransactionsFromFile(filePath string) ([]Transaction, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var transactions []Transaction
	for i, record := range records {
		if i == 0 || len(record) < 6 {
			continue
		}
		amount, _ := strconv.ParseFloat(record[5], 64)
		transactions = append(transactions, Transaction{
			TranDate:    record[0],
			TranTime:    record[1],
			From:        record[2],
			To:          record[3],
			Description: record[4],
			Amount:      amount,
		})
	}
	return transactions, nil
}

func writeTransactionsToFile(filePath string, transactions []Transaction) error {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"TranDate", "TranTime", "From", "To", "Description", "Amount"})

	for _, t := range transactions {
		writer.Write([]string{
			t.TranDate,
			t.TranTime,
			t.From,
			t.To,
			t.Description,
			fmt.Sprintf("%.2f", t.Amount),
		})
	}
	writer.Flush()
	return writer.Error()
}

func recalculateAllData() error {
	transactions, err := readAllTransactions()
	if err != nil {
		return err
	}

	accounts, err := readAccounts()
	if err != nil {
		return err
	}

	// Initialize account balances to zero (we'll build them up from transactions)
	accountBalances := make(map[string]float64)
	for _, acc := range accounts {
		// Start with current balance for non-transactional accounts (MutualFunds, Stocks, Loans)
		accountBalances[acc.Name] = 0
	}

	// Group transactions by date
	type DailyData struct {
		Transactions []Transaction
		Expenses     float64
	}
	dailyData := make(map[string]*DailyData)

	for _, tran := range transactions {
		date := tran.TranDate
		if dailyData[date] == nil {
			dailyData[date] = &DailyData{
				Transactions: []Transaction{},
				Expenses:     0,
			}
		}
		dailyData[date].Transactions = append(dailyData[date].Transactions, tran)

		toAcc := findAccount(accounts, tran.To)
		if toAcc.Type == "EXPENSE" {
			dailyData[date].Expenses += tran.Amount
		}
	}

	// Get sorted dates (chronologically - oldest first)
	var dates []string
	for date := range dailyData {
		dates = append(dates, date)
	}
	// Sort dates chronologically
	sort.Slice(dates, func(i, j int) bool {
		return compareDates(dates[j], dates[i]) // Reverse to get oldest first
	})

	// Calculate progressive net worth day by day
	var records []Record
	for _, date := range dates {
		// Apply transactions for this date
		for _, tran := range dailyData[date].Transactions {
			accountBalances[tran.From] -= tran.Amount
			accountBalances[tran.To] += tran.Amount
		}

		// Calculate net worth after this day's transactions
		netWorth := 0.0
		assets := 0.0
		liabilities := 0.0

		for _, acc := range accounts {
			currentBalance := accountBalances[acc.Name]
			
			if acc.IINW == "Yes" {
				if acc.Type == "ASSET" {
					assets += currentBalance
					netWorth += currentBalance
				} else if acc.Type == "LIABILITIES" {
					liabilities += currentBalance
					netWorth += currentBalance  // currentBalance is already negative
				}
			}
		}

		records = append(records, Record{
			Date:        date,
			NetWorth:    netWorth,
			Assets:      assets,
			Liabilities: liabilities,
			Expenses:    dailyData[date].Expenses,
		})
	}

	// Update account balances to final values
	for _, acc := range accounts {
		if err := updateAccountBalance(acc.Name, accountBalances[acc.Name]); err != nil {
			return err
		}
	}

	return writeRecords(records)
}

func findAccount(accounts []Account, name string) Account {
	for _, acc := range accounts {
		if acc.Name == name {
			return acc
		}
	}
	return Account{}
}

func updateAccountBalance(name string, balance float64) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}

	// Update balance
	for i := range accounts {
		if accounts[i].Name == name {
			accounts[i].Amount = balance
			break
		}
	}
	
	// Read all transactions to calculate usage and sort
	transactions, err := readAllTransactions()
	if err == nil {
		accounts = sortAccountsByUsage(accounts, transactions)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filepath.Join(DATA_DIR, "account.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Account", "Type", "Amount", "IINW", "Budget", "DueDate"})

	for _, acc := range accounts {
		writer.Write([]string{
			acc.Name,
			acc.Type,
			fmt.Sprintf("%.2f", acc.Amount),
			acc.IINW,
			fmt.Sprintf("%.2f", acc.Budget),
			acc.DueDate,
		})
	}
	writer.Flush()
	return writer.Error()
}

func readRecords() ([]Record, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Open(filepath.Join(DATA_DIR, "record.csv"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var result []Record
	for i, record := range records {
		if i == 0 || len(record) < 5 {
			continue
		}
		netWorth, _ := strconv.ParseFloat(record[1], 64)
		assets, _ := strconv.ParseFloat(record[2], 64)
		liabilities, _ := strconv.ParseFloat(record[3], 64)
		expenses, _ := strconv.ParseFloat(record[4], 64)

		result = append(result, Record{
			Date:        record[0],
			NetWorth:    netWorth,
			Assets:      assets,
			Liabilities: liabilities,
			Expenses:    expenses,
		})
	}
	return result, nil
}

func writeRecords(records []Record) error {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	file, err := os.Create(filepath.Join(DATA_DIR, "record.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Date", "NetWorth", "Assets", "Liabilities", "Expenses"})

	// Write records in reverse order (newest first) for display
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		writer.Write([]string{
			r.Date,
			fmt.Sprintf("%.2f", r.NetWorth),
			fmt.Sprintf("%.2f", r.Assets),
			fmt.Sprintf("%.2f", r.Liabilities),
			fmt.Sprintf("%.2f", r.Expenses),
		})
	}
	writer.Flush()
	return writer.Error()
}

func calculateBudget(transactions []Transaction, accounts []Account, month string) map[string]interface{} {
	totalBudget := 0.0
	totalSpent := 0.0
	breakdown := make(map[string]map[string]float64)

	for _, acc := range accounts {
		if acc.Type == "EXPENSE" && acc.Budget > 0 {
			totalBudget += acc.Budget
			breakdown[acc.Name] = map[string]float64{
				"budget": acc.Budget,
				"spent":  0,
			}
		}
	}

	for _, tran := range transactions {
		if len(tran.TranDate) >= 10 {
			tranMonth := tran.TranDate[3:10]
			if tranMonth == month {
				toAcc := findAccount(accounts, tran.To)
				if toAcc.Type == "EXPENSE" {
					totalSpent += tran.Amount
					if breakdown[tran.To] != nil {
						breakdown[tran.To]["spent"] += tran.Amount
					}
				}
			}
		}
	}

	percentage := 0.0
	if totalBudget > 0 {
		percentage = (totalSpent / totalBudget) * 100
	}

	return map[string]interface{}{
		"totalBudget": totalBudget,
		"totalSpent":  totalSpent,
		"percentage":  percentage,
		"breakdown":   breakdown,
	}
}

func getUpcomingBills(accounts []Account) []map[string]interface{} {
	var bills []map[string]interface{}
	now := time.Now()

	for _, acc := range accounts {
		if acc.Type == "LIABILITIES" && acc.DueDate != "" {
			dueDate, err := time.Parse("02-01-2006", acc.DueDate)
			if err != nil {
				continue
			}

			daysUntil := int(dueDate.Sub(now).Hours() / 24)
			if daysUntil >= 0 && daysUntil <= 30 {
				urgency := "normal"
				if daysUntil < 3 {
					urgency = "high"
				} else if daysUntil < 7 {
					urgency = "medium"
				}

				bills = append(bills, map[string]interface{}{
					"name":     acc.Name,
					"dueDate":  acc.DueDate,
					"amount":   acc.Amount,
					"urgency":  urgency,
					"daysLeft": daysUntil,
				})
			}
		}
	}

	sort.Slice(bills, func(i, j int) bool {
		return bills[i]["daysLeft"].(int) < bills[j]["daysLeft"].(int)
	})

	return bills
}

func startDailyBatch() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		logFile, err := os.OpenFile(
			filepath.Join(LOG_DIR, "batch_"+time.Now().Format("2006-01-02")+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0600,
		)
		if err != nil {
			log.Printf("Failed to open batch log: %v", err)
			continue
		}

		logger := log.New(logFile, "", log.LstdFlags)
		logger.Println("Starting daily batch process")

		if err := recalculateAllData(); err != nil {
			logger.Printf("Error in batch process: %v", err)
		} else {
			logger.Println("Daily batch completed successfully")
		}

		logFile.Close()
	}
}

// Utility functions
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex]
	}
	return ip
}

// compareDates compares two dates in DD-MM-YYYY format
// Returns true if date1 is after date2
func compareDates(date1, date2 string) bool {
	// Parse DD-MM-YYYY format
	if len(date1) < 10 || len(date2) < 10 {
		return date1 > date2
	}
	
	// Extract day, month, year
	day1, month1, year1 := date1[0:2], date1[3:5], date1[6:10]
	day2, month2, year2 := date2[0:2], date2[3:5], date2[6:10]
	
	// Compare year, then month, then day
	if year1 != year2 {
		return year1 > year2
	}
	if month1 != month2 {
		return month1 > month2
	}
	return day1 > day2
}

func respondError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func logSecurityEvent(event, ip, details string) {
	logFile, err := os.OpenFile(
		filepath.Join(LOG_DIR, "security_"+time.Now().Format("2006-01-02")+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0600,
	)
	if err != nil {
		log.Printf("Failed to open security log: %v", err)
		return
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags)
	logger.Printf("[%s] IP: %s - %s", event, ip, details)
}

func cleanupSessions() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sessionMutex.Lock()
		for token, session := range sessions {
			if time.Since(session.LastAccess) > SESSION_TIMEOUT {
				delete(sessions, token)
			}
		}
		sessionMutex.Unlock()
	}
}

func cleanupLoginAttempts() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		loginAttemptsMux.Lock()
		for ip, attempt := range loginAttempts {
			if time.Since(attempt.LastAttempt) > LOGIN_LOCKOUT_MINUTES*time.Minute*2 {
				delete(loginAttempts, ip)
			}
		}
		loginAttemptsMux.Unlock()
	}
}