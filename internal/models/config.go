package models

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	Theme string `json:"theme"`
	// FontScales holds per-section font-size multipliers persisted across
	// sessions. Keys: "notes", "tasks", "links". Values are floats in the
	// inclusive range FontScaleMin..FontScaleMax (clamped on read). A value
	// of 1.0 means "use the default font size."
	FontScales map[string]float64 `json:"font_scales,omitempty"`
}

// Font-scale clamps used by the API handler and the client UI.
const (
	FontScaleMin     = 0.8
	FontScaleMax     = 1.6
	FontScaleDefault = 1.0
)

// FontScaleSections is the canonical list of section keys the UI can scale.
// Adding a new section here + a matching CSS variable wires it through with
// no further plumbing changes.
var FontScaleSections = []string{"notes", "tasks", "links"}

// GetFontScale returns the persisted scale for the named section, or
// FontScaleDefault if unset. Always returns a value within [FontScaleMin,
// FontScaleMax] — out-of-range values stored on disk are clamped.
func (c *Config) GetFontScale(section string) float64 {
	if c.FontScales == nil {
		return FontScaleDefault
	}
	v, ok := c.FontScales[section]
	if !ok {
		return FontScaleDefault
	}
	if v < FontScaleMin {
		return FontScaleMin
	}
	if v > FontScaleMax {
		return FontScaleMax
	}
	return v
}

// SetFontScale records a section's scale, clamping to the allowed range.
// Pass FontScaleDefault to reset a section.
func (c *Config) SetFontScale(section string, value float64) {
	if c.FontScales == nil {
		c.FontScales = make(map[string]float64, len(FontScaleSections))
	}
	if value < FontScaleMin {
		value = FontScaleMin
	}
	if value > FontScaleMax {
		value = FontScaleMax
	}
	c.FontScales[section] = value
}

// Theme represents a color theme
type Theme struct {
	Name   string            `json:"name"`
	Colors map[string]string `json:"colors"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	scales := make(map[string]float64, len(FontScaleSections))
	for _, s := range FontScaleSections {
		scales[s] = FontScaleDefault
	}
	return &Config{
		Theme:      "dark-orange",
		FontScales: scales,
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