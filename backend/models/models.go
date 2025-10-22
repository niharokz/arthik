package models

import "time"

// Account represents a financial account
type Account struct {
	Name              string  `json:"name"`
	Category          string  `json:"category"`
	IncludeInNetWorth bool    `json:"includeInNetWorth"`
	CurrentBalance    float64 `json:"currentBalance"`
	DueDate           string  `json:"dueDate,omitempty"`
	LastPaymentDate   string  `json:"lastPaymentDate,omitempty"`
	Budget            float64 `json:"budget,omitempty"`
}

// Transaction represents a financial transaction
type Transaction struct {
	ID          string    `json:"id"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	Date        time.Time `json:"date"`
}

// Budget represents a budget allocation
type Budget struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Month    string  `json:"month"`
}

// Settings represents application settings
type Settings struct {
	Theme         string `json:"theme"`
	DateFormat    string `json:"dateFormat"`
	PasswordHash  string `json:"passwordHash"`
	EncryptionKey string `json:"encryptionKey"`
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalAssets      float64         `json:"totalAssets"`
	TotalLiabilities float64         `json:"totalLiabilities"`
	NetWorth         float64         `json:"netWorth"`
	MonthIncome      float64         `json:"monthIncome"`
	MonthExpenses    float64         `json:"monthExpenses"`
	MonthSavings     float64         `json:"monthSavings"`
	BudgetVsExpenses []BudgetExpense `json:"budgetVsExpenses"`
	HistoricalData   []MonthlyReport `json:"historicalData"`
}

// BudgetExpense represents budget vs actual expense comparison
type BudgetExpense struct {
	Category string  `json:"category"`
	Budget   float64 `json:"budget"`
	Actual   float64 `json:"actual"`
}

// MonthlyReport represents a monthly financial summary
type MonthlyReport struct {
	Date        string  `json:"date"`
	NetWorth    float64 `json:"netWorth"`
	Liabilities float64 `json:"liabilities"`
	Savings     float64 `json:"savings"`
}

// Recurrence represents a recurring transaction
type Recurrence struct {
	ID          string    `json:"id"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	NextDate    time.Time `json:"nextDate"`
	DayOfMonth  int       `json:"dayOfMonth"`
}

// Note represents a user note
type Note struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Content string `json:"content"`
	Created string `json:"created"`
}

// LoginRequest represents login request payload
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse represents login response payload
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// SettingsUpdateRequest represents settings update request
type SettingsUpdateRequest struct {
	Theme       string `json:"theme"`
	DateFormat  string `json:"dateFormat"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// GenericResponse represents a generic success/error response
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// TransactionRequest represents transaction create/update request
type TransactionRequest struct {
	ID          string  `json:"id"`
	From        string  `json:"from"`
	To          string  `json:"to"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Time        string  `json:"time"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	Timestamp time.Time
	IP        string
	User      string
	Action    string
	Resource  string
	Success   bool
}