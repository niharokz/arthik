package utils

import (
	"fmt"
	"log"
	"os"
	"time"

	"arthik/config"
	"arthik/models"
)

// LogAudit logs an audit entry to the audit log file
func LogAudit(ip, user, action, resource string, success bool) {
	entry := models.AuditLog{
		Timestamp: time.Now(),
		IP:        ip,
		User:      user,
		Action:    action,
		Resource:  resource,
		Success:   success,
	}

	logLine := fmt.Sprintf("[%s] IP:%s User:%s Action:%s Resource:%s Success:%v\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.IP,
		entry.User,
		entry.Action,
		entry.Resource,
		entry.Success,
	)

	f, err := os.OpenFile(config.AuditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("Failed to write audit log: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logLine); err != nil {
		log.Printf("Failed to write audit log: %v", err)
	}
}

// LogInfo logs an informational message
func LogInfo(message string) {
	log.Printf("[INFO] %s", message)
}

// LogError logs an error message
func LogError(message string, err error) {
	log.Printf("[ERROR] %s: %v", message, err)
}

// LogWarning logs a warning message
func LogWarning(message string) {
	log.Printf("[WARNING] %s", message)
}