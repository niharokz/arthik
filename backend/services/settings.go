package services

import (
	"encoding/json"
	"os"

	"arthik/config"
	"arthik/middleware"
	"arthik/models"
)

var settings models.Settings

// LoadSettings loads settings from file
func LoadSettings() error {
	data, err := os.ReadFile(config.SettingsFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &settings)
}

// SaveSettings saves settings to file
func SaveSettings(s models.Settings) error {
	settings = s
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.SettingsFile, data, 0644)
}

// GetSettings returns current settings
func GetSettings() models.Settings {
	return settings
}

// InitializeSettings creates default settings if they don't exist
func InitializeSettings() error {
	if _, err := os.Stat(config.SettingsFile); os.IsNotExist(err) {
		hash, _ := middleware.HashPassword("admin123")
		settings = models.Settings{
			Theme:         "light",
			DateFormat:    "DD/MM/YYYY",
			PasswordHash:  hash,
			EncryptionKey: generateEncryptionKey(),
		}
		return SaveSettings(settings)
	}
	return LoadSettings()
}

func generateEncryptionKey() string {
	// This will be imported from utils
	return ""
}