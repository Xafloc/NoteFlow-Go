package models

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	Theme string `json:"theme"`
}

// Theme represents a color theme
type Theme struct {
	Name   string            `json:"name"`
	Colors map[string]string `json:"colors"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Theme: "dark-orange",
	}
}

// LoadConfig loads configuration from the given file path
func LoadConfig(configPath string) (*Config, error) {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, err
	}

	// If config file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := SaveConfig(config, configPath); err != nil {
			return config, err // Return default config even if save fails
		}
		return config, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultConfig(), err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return DefaultConfig(), err
	}

	return &config, nil
}

// SaveConfig saves configuration to the given file path
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// NoteRequest represents a note creation/update request
type NoteRequest struct {
	Title   string `form:"title" json:"title"`
	Content string `form:"content" json:"content"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}