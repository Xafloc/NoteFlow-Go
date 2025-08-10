package services

import (
	"bytes"
	"embed"
	"html/template"
	"os"
	"strings"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/themes"
)

// TemplateService handles HTML template rendering
type TemplateService struct {
	templates map[string]*template.Template
	assets    *embed.FS
}

// NewTemplateService creates a new template service
func NewTemplateService(assets *embed.FS) (*TemplateService, error) {
	service := &TemplateService{
		templates: make(map[string]*template.Template),
		assets:    assets,
	}

	// Load main template
	if err := service.loadTemplates(); err != nil {
		return nil, err
	}

	return service, nil
}

// loadTemplates loads all templates from embedded filesystem
func (ts *TemplateService) loadTemplates() error {
	var indexHTML []byte
	var err error
	
	// Try to read from embedded assets first, fallback to filesystem
	if ts.assets != nil {
		indexHTML, err = ts.assets.ReadFile("web/templates/index.html")
	} else {
		indexHTML, err = os.ReadFile("web/templates/index.html")
	}
	
	if err != nil {
		return err
	}

	// Create template with helper functions
	tmpl := template.New("index.html").Funcs(template.FuncMap{
		"join": strings.Join,
	})

	// Parse the template
	tmpl, err = tmpl.Parse(string(indexHTML))
	if err != nil {
		return err
	}

	ts.templates["index"] = tmpl
	return nil
}

// RenderIndex renders the main index page with theme and context
func (ts *TemplateService) RenderIndex(config *models.Config, basePath string) (string, error) {
	// Get current theme
	theme := themes.AvailableThemes[config.Theme]
	if theme == nil {
		theme = themes.AvailableThemes["dark-orange"]
	}

	// Read font CSS
	fontCSS, err := ts.getFontCSS()
	if err != nil {
		return "", err
	}

	// Generate themed CSS
	themedCSS, err := ts.getThemedCSS(theme.Colors)
	if err != nil {
		return "", err
	}

	// Template data
	data := struct {
		FontFaces    template.CSS
		ThemedStyles template.CSS
		CurrentTheme string
		FolderPath   string
	}{
		FontFaces:    template.CSS(fontCSS),
		ThemedStyles: template.CSS(themedCSS),
		CurrentTheme: config.Theme,
		FolderPath:   basePath,
	}

	// Execute template
	var buf bytes.Buffer
	if err := ts.templates["index"].Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getFontCSS returns the font CSS content
func (ts *TemplateService) getFontCSS() (string, error) {
	var fontCSS []byte
	var err error
	
	if ts.assets != nil {
		fontCSS, err = ts.assets.ReadFile("web/static/css/fonts.css")
	} else {
		fontCSS, err = os.ReadFile("web/static/css/fonts.css")
	}
	
	if err != nil {
		return "", err
	}
	return string(fontCSS), nil
}

// getThemedCSS returns the CSS with theme colors applied
func (ts *TemplateService) getThemedCSS(colors map[string]string) (string, error) {
	var cssTemplate []byte
	var err error
	
	if ts.assets != nil {
		cssTemplate, err = ts.assets.ReadFile("web/static/css/styles.css")
	} else {
		cssTemplate, err = os.ReadFile("web/static/css/styles.css")
	}
	
	if err != nil {
		return "", err
	}

	cssContent := string(cssTemplate)

	// Replace color placeholders with actual theme colors
	for key, value := range colors {
		placeholder := "{{." + key + "}}"
		cssContent = strings.ReplaceAll(cssContent, placeholder, value)
	}

	return cssContent, nil
}

// RenderGlobalTasks renders the global tasks page with theme styling
func (ts *TemplateService) RenderGlobalTasks(config *models.Config, basePath string) (string, error) {
	// Get current theme
	theme := themes.AvailableThemes[config.Theme]
	if theme == nil {
		theme = themes.AvailableThemes["dark-orange"]
	}

	// Read global tasks template
	var templateHTML []byte
	var err error
	
	if ts.assets != nil {
		templateHTML, err = ts.assets.ReadFile("web/templates/globaltasks.html")
	} else {
		templateHTML, err = os.ReadFile("web/templates/globaltasks.html")
	}
	
	if err != nil {
		return "", err
	}

	// Generate themed CSS
	themedCSS, err := ts.getThemedCSS(theme.Colors)
	if err != nil {
		return "", err
	}

	// Template data combining theme colors and CSS
	data := map[string]interface{}{
		"CSS":        template.CSS(themedCSS),
		"WorkingDir": basePath,
	}

	// Add theme colors to template data
	for key, value := range theme.Colors {
		data[key] = value
	}

	// Parse and execute template
	tmpl, err := template.New("globaltasks").Parse(string(templateHTML))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}