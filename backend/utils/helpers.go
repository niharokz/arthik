package utils

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// GenerateEncryptionKey generates a random encryption key
func GenerateEncryptionKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (if behind proxy)
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
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// ToSnakeCase converts a string to snake_case
func ToSnakeCase(str string) string {
	str = strings.ToLower(str)
	str = strings.ReplaceAll(str, " ", "_")
	str = strings.ReplaceAll(str, "-", "_")
	
	var result strings.Builder
	for _, char := range str {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

// GenerateTransactionID generates a unique transaction ID
func GenerateTransactionID() string {
	return "TRAN" + time.Now().Format("020106150405")
}

// ParseTransactionID parses a transaction ID to extract the date
func ParseTransactionID(id string) time.Time {
	if len(id) < 16 || !strings.HasPrefix(id, "TRAN") {
		return time.Now()
	}
	timeStr := id[4:16] // Get exactly 12 characters for date/time
	
	// Parse with 2-digit year format
	t, err := time.Parse("020106150405", timeStr)
	if err != nil {
		return time.Now()
	}
	
	// Fix 2-digit year: if year < 100, assume it's 20XX
	if t.Year() < 100 {
		t = t.AddDate(2000, 0, 0)
	}
	
	return t
}

// GenerateRecurrenceID generates a unique recurrence ID
func GenerateRecurrenceID() string {
	return "REC" + time.Now().Format("020106150405")
}

// GenerateNoteID generates a unique note ID
func GenerateNoteID() string {
	return "NOTE" + time.Now().Format("020106150405")
}

// FormatDateTime formats a date and time string into a time.Time
func FormatDateTime(date, timeStr string) (time.Time, error) {
	dateTimeStr := date + " " + timeStr + ":00"
	return time.Parse("2006-01-02 15:04:05", dateTimeStr)
}

// GetCurrentMonth returns current month in YYYYMM format
func GetCurrentMonth() string {
	return time.Now().Format("200601")
}

// GetMonthFromDate extracts month in YYYYMM format from a date
func GetMonthFromDate(date time.Time) string {
	return date.Format("200601")
}