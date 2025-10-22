package utils

import (
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"arthik/config"
)

// ValidateAndSanitize validates and sanitizes input string
func ValidateAndSanitize(input string, maxLen int) (string, error) {
	input = strings.TrimSpace(input)

	if utf8.RuneCountInString(input) > maxLen {
		return "", fmt.Errorf("input exceeds maximum length of %d characters", maxLen)
	}

	input = html.EscapeString(input)
	return input, nil
}

// ValidateAmount validates a monetary amount
func ValidateAmount(amount float64) error {
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}
	if amount > config.MaxAmount {
		return fmt.Errorf("amount exceeds maximum allowed value")
	}
	return nil
}

// ValidateAccountName validates an account name
func ValidateAccountName(name string) error {
	if name == "" {
		return fmt.Errorf("account name is required")
	}
	if utf8.RuneCountInString(name) > config.MaxInputLength {
		return fmt.Errorf("account name too long")
	}
	return nil
}

// ValidateCategory validates an account category
func ValidateCategory(category string) error {
	if !config.ValidCategories[category] {
		return fmt.Errorf("invalid account category")
	}
	return nil
}

// ValidateDate validates a date string in YYYY-MM-DD format
func ValidateDate(dateStr string) error {
	_, err := time.Parse("2006-01-02", dateStr)
	return err
}

// ValidateTransaction validates a transaction request
func ValidateTransaction(from, to, description string, amount float64) error {
	if from == "" {
		return fmt.Errorf("from account is required")
	}
	if to == "" {
		return fmt.Errorf("to account is required")
	}
	if from == to {
		return fmt.Errorf("from and to accounts must be different")
	}

	if err := ValidateAmount(amount); err != nil {
		return err
	}

	if description == "" {
		return fmt.Errorf("description is required")
	}
	if utf8.RuneCountInString(description) > config.MaxDescriptionLen {
		return fmt.Errorf("description exceeds maximum length")
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}
	return nil
}

// ValidateDayOfMonth validates day of month for recurring transactions
func ValidateDayOfMonth(day int) error {
	if day < 1 || day > 31 {
		return fmt.Errorf("day of month must be between 1 and 31")
	}
	return nil
}

// SanitizeInput sanitizes input by escaping HTML
func SanitizeInput(input string) string {
	return html.EscapeString(strings.TrimSpace(input))
}