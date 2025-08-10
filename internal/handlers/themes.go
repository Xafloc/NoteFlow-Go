package handlers

import (
	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/themes"
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