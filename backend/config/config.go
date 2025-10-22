package config

import (
	"path/filepath"
	"time"
)

// Application constants
const (
	// Input validation limits
	MaxInputLength    = 500
	MaxDescriptionLen = 1000
	MaxAmount         = 999999999.99

	// Security settings
	TokenExpiry        = 24 * time.Hour
	LoginAttemptWindow = 15 * time.Minute
	MaxLoginAttempts   = 5
	RequestsPerSecond  = 10
	BurstSize          = 20

	// Server settings
	ServerPort    = ":8081"
	MaxHeaderSize = 1 << 20 // 1 MB
)

// File paths
var (
	DataDir        = "../data" 
	LedgerDir      = filepath.Join(DataDir, "ledger")
	NotesDir       = filepath.Join(DataDir, "notes")
	AccountsFile   = filepath.Join(DataDir, "accounts.csv")
	SettingsFile   = filepath.Join(DataDir, "settings.json")
	BudgetsFile    = filepath.Join(DataDir, "budgets.json")
	RecurrenceFile = filepath.Join(DataDir, "recurrence.csv")
	ReportsFile    = filepath.Join(DataDir, "reports.csv")
	AuditFile      = filepath.Join(DataDir, "audit.log")
)

// Valid account categories
var ValidCategories = map[string]bool{
	"Assets":      true,
	"Liabilities": true,
	"Equity":      true,
	"Revenue":     true,
	"Expenses":    true,
}

// Default accounts to create on first run
var DefaultAccounts = []struct {
	Name              string
	Category          string
	IncludeInNetWorth bool
}{
	{"Cash", "Assets", true},
	{"Bank Account", "Assets", true},
	{"Credit Card", "Liabilities", true},
	{"Salary", "Revenue", false},
	{"Food & Dining", "Expenses", false},
	{"Transportation", "Expenses", false},
}

// Security headers configuration
var SecurityHeaders = map[string]string{
	"X-Content-Type-Options":  "nosniff",
	"X-Frame-Options":         "DENY",
	"X-XSS-Protection":        "1; mode=block",
	"Referrer-Policy":         "strict-origin-when-cross-origin",
	"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	"Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:",
}

// GetTransactionFilePath returns the file path for a given month's transactions
func GetTransactionFilePath(month string) string {
	return filepath.Join(LedgerDir, "transactions_"+month+".csv")
}

// GetNoteFilePath returns the file path for a note
func GetNoteFilePath(heading string) string {
	return filepath.Join(NotesDir, toSnakeCase(heading)+".csv")
}

// toSnakeCase converts a string to snake_case
func toSnakeCase(str string) string {
	// This is a simplified version - move full implementation from main
	return str
}
