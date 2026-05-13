package handlers

import (
	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/themes"
	"github.com/gofiber/fiber/v2"
)

// ThemesHandler handles theme-related requests
type ThemesHandler struct {
	config     *models.Config
	configPath string
}

// NewThemesHandler creates a new themes handler
func NewThemesHandler(config *models.Config, configPath string) *ThemesHandler {
	return &ThemesHandler{
		config:     config,
		configPath: configPath,
	}
}


// GetThemes returns the list of available themes
func (h *ThemesHandler) GetThemes(c *fiber.Ctx) error {
	var themeNames []string
	for name := range themes.AvailableThemes {
		themeNames = append(themeNames, name)
	}
	return c.JSON(themeNames)
}

// GetCurrentTheme returns the currently active theme
func (h *ThemesHandler) GetCurrentTheme(c *fiber.Ctx) error {
	return c.JSON(map[string]string{
		"theme": h.config.Theme,
	})
}

// SetTheme sets the current theme (runtime only)
func (h *ThemesHandler) SetTheme(c *fiber.Ctx) error {
	var req struct {
		Theme string `form:"theme" json:"theme"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request format")
	}

	theme, exists := themes.AvailableThemes[req.Theme]
	if !exists {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid theme")
	}

	return c.JSON(models.APIResponse{
		Status: "success",
		Data:   theme.Colors,
	})
}

// GetFontScales returns the persisted font-size multipliers for every
// section the UI can scale. Always returns the full canonical key set,
// substituting defaults for any sections never explicitly saved — so the
// JS doesn't need to special-case "first run."
func (h *ThemesHandler) GetFontScales(c *fiber.Ctx) error {
	out := make(map[string]float64, len(models.FontScaleSections))
	for _, s := range models.FontScaleSections {
		out[s] = h.config.GetFontScale(s)
	}
	return c.JSON(models.APIResponse{
		Status: "success",
		Data: map[string]any{
			"scales": out,
			"min":    models.FontScaleMin,
			"max":    models.FontScaleMax,
		},
	})
}

// SaveFontScale persists a single section's font-size multiplier. The
// section name must be one of models.FontScaleSections; the value is
// clamped to [FontScaleMin, FontScaleMax] before being stored.
func (h *ThemesHandler) SaveFontScale(c *fiber.Ctx) error {
	var req struct {
		Section string  `json:"section"`
		Scale   float64 `json:"scale"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request format")
	}
	known := false
	for _, s := range models.FontScaleSections {
		if s == req.Section {
			known = true
			break
		}
	}
	if !known {
		return fiber.NewError(fiber.StatusBadRequest, "Unknown section: "+req.Section)
	}
	h.config.SetFontScale(req.Section, req.Scale)
	if err := models.SaveConfig(h.config, h.configPath); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save font scale")
	}
	return c.JSON(models.APIResponse{
		Status: "success",
		Data: map[string]float64{
			req.Section: h.config.GetFontScale(req.Section),
		},
	})
}

// SaveTheme saves the user's theme preference to config file
func (h *ThemesHandler) SaveTheme(c *fiber.Ctx) error {
	var req struct {
		Theme string `form:"theme" json:"theme"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request format")
	}

	if _, exists := themes.AvailableThemes[req.Theme]; !exists {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid theme")
	}

	// Update config
	h.config.Theme = req.Theme

	// Save to file
	if err := models.SaveConfig(h.config, h.configPath); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save theme preference")
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}