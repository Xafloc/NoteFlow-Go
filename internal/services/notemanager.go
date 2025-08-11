package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/storage"
)

// NoteManager manages notes and tasks for a specific project
type NoteManager struct {
	notes         []*models.Note
	checkboxIndex int
	storage       *storage.FileStorage
	renderer      *MarkdownRenderer
	mu            sync.RWMutex
	needsSave     bool
}

// NewNoteManager creates a new note manager for the given base path
func NewNoteManager(basePath string) (*NoteManager, error) {
	storage := storage.NewFileStorage(basePath)
	renderer := NewMarkdownRenderer()

	// Ensure necessary directories exist
	if err := storage.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	manager := &NoteManager{
		notes:         make([]*models.Note, 0),
		checkboxIndex: 0,
		storage:       storage,
		renderer:      renderer,
	}

	// Load existing notes
	if err := manager.loadNotes(); err != nil {
		return nil, fmt.Errorf("failed to load notes: %w", err)
	}

	return manager, nil
}

// loadNotes loads all notes from storage
func (nm *NoteManager) loadNotes() error {
	notes, err := nm.storage.LoadNotes()
	if err != nil {
		return err
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.notes = notes
	nm.assignTaskIndices()

	return nil
}

// assignTaskIndices assigns unique indices to all tasks
func (nm *NoteManager) assignTaskIndices() {
	index := 0
	for _, note := range nm.notes {
		for _, task := range note.Tasks {
			task.Index = index
			index++
		}
	}
	nm.checkboxIndex = index
}

// AddNote adds a new note to the collection
func (nm *NoteManager) AddNote(title, content string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Process any +http links in content
	processedContent, err := nm.processArchiveLinks(content)
	if err != nil {
		// Log error but continue with original content
		processedContent = content
	}

	note := models.NewNote(title, processedContent)
	
	// Assign task indices
	for _, task := range note.Tasks {
		task.Index = nm.checkboxIndex
		nm.checkboxIndex++
	}

	// Insert at the beginning (newest first)
	nm.notes = append([]*models.Note{note}, nm.notes...)
	nm.needsSave = true

	return nm.save()
}

// UpdateNote updates an existing note
func (nm *NoteManager) UpdateNote(index int, title, content string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if index < 0 || index >= len(nm.notes) {
		return fmt.Errorf("note index %d out of range", index)
	}

	// Process any +http links in content
	processedContent, err := nm.processArchiveLinks(content)
	if err != nil {
		// Log error but continue with original content
		processedContent = content
	}

	note := nm.notes[index]
	oldTaskCount := len(note.Tasks)

	note.Update(title, processedContent)

	// Update task indices if task count changed
	if len(note.Tasks) != oldTaskCount {
		nm.reassignTaskIndicesFromNote(index)
	}

	nm.needsSave = true
	return nm.save()
}

// DeleteNote removes a note from the collection
func (nm *NoteManager) DeleteNote(index int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if index < 0 || index >= len(nm.notes) {
		return fmt.Errorf("note index %d out of range", index)
	}

	// Remove note from slice
	nm.notes = append(nm.notes[:index], nm.notes[index+1:]...)
	
	// Reassign all task indices since we removed a note
	nm.assignTaskIndices()
	
	nm.needsSave = true
	return nm.save()
}

// GetNote returns a note by index
func (nm *NoteManager) GetNote(index int) (*models.Note, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if index < 0 || index >= len(nm.notes) {
		return nil, fmt.Errorf("note index %d out of range", index)
	}

	return nm.notes[index], nil
}

// GetAllNotes returns all notes
func (nm *NoteManager) GetAllNotes() []*models.Note {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// Return a copy to prevent external modification
	notes := make([]*models.Note, len(nm.notes))
	copy(notes, nm.notes)
	return notes
}

// GetActiveTasksreturns all unchecked tasks across all notes
func (nm *NoteManager) GetActiveTasks() []*models.TaskInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var tasks []*models.TaskInfo
	for _, note := range nm.notes {
		tasks = append(tasks, note.GetUncheckedTasks()...)
	}
	return tasks
}

// UpdateTask updates a task's completion status
func (nm *NoteManager) UpdateTask(taskIndex int, checked bool) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Find the task across all notes
	for _, note := range nm.notes {
		if note.UpdateTask(taskIndex, checked) {
			nm.needsSave = true
			return nm.save()
		}
	}

	return fmt.Errorf("task with index %d not found", taskIndex)
}

// RenderNotesHTML returns HTML representation of all notes
func (nm *NoteManager) RenderNotesHTML() (string, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var htmlParts []string

	for i, note := range nm.notes {
		timestamp := note.Timestamp.Format("2006-01-02 15:04:05")
		titleDisplay := timestamp
		if note.Title != "" {
			titleDisplay += " - " + note.Title
		}

		noteHTML, err := nm.renderer.RenderNoteHTML(note.Content, titleDisplay, note.Title, i)
		if err != nil {
			return "", fmt.Errorf("failed to render note %d: %w", i, err)
		}

		htmlParts = append(htmlParts, noteHTML)
	}

	return strings.Join(htmlParts, ""), nil
}

// save persists notes to storage if needed
func (nm *NoteManager) save() error {
	if !nm.needsSave {
		return nil
	}

	if err := nm.storage.SaveNotes(nm.notes); err != nil {
		return fmt.Errorf("failed to save notes: %w", err)
	}

	nm.needsSave = false
	return nil
}

// reassignTaskIndicesFromNote reassigns task indices starting from a specific note
func (nm *NoteManager) reassignTaskIndicesFromNote(startNoteIndex int) {
	index := nm.checkboxIndex

	// Count existing tasks before the start note
	for i := 0; i < startNoteIndex && i < len(nm.notes); i++ {
		for range nm.notes[i].Tasks {
			index--
		}
	}

	// Reassign from start note onwards
	for i := startNoteIndex; i < len(nm.notes); i++ {
		for _, task := range nm.notes[i].Tasks {
			task.Index = index
			index++
		}
	}

	// Update the global counter
	nm.checkboxIndex = index
}

// processArchiveLinks processes +http links in content and archives the websites
func (nm *NoteManager) processArchiveLinks(content string) (string, error) {
	// Regular expression to match +http(s)://... links
	re := regexp.MustCompile(`\+https?://[^\s\)]+`)
	
	// Find all matches
	matches := re.FindAllString(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	processedContent := content
	
	for _, match := range matches {
		// Remove the + prefix to get the actual URL
		url := strings.TrimPrefix(match, "+")
		
		// Archive the website
		archiveInfo, err := nm.archiveWebsite(url)
		if err != nil {
			log.Printf("Warning: failed to archive %s: %v", url, err)
			continue
		}
		
		// Replace +URL with archived link reference
		archiveLink := fmt.Sprintf("[%s](%s) (archived %s)", 
			archiveInfo.Title, 
			archiveInfo.FilePath, 
			archiveInfo.Timestamp.Format("2006-01-02 15:04"))
		
		processedContent = strings.Replace(processedContent, match, archiveLink, 1)
	}
	
	return processedContent, nil
}

// ArchiveInfo contains information about an archived website
type ArchiveInfo struct {
	Title     string
	FilePath  string
	Timestamp time.Time
}

// archiveWebsite downloads and archives a website with inlined resources
func (nm *NoteManager) archiveWebsite(websiteURL string) (*ArchiveInfo, error) {
	// Parse the URL
	parsedURL, err := url.Parse(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	
	// Download the webpage
	resp, err := http.Get(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download webpage: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	// Read the HTML content
	htmlContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Extract title from HTML
	title := nm.extractTitle(string(htmlContent), parsedURL.Host)
	
	// Create filename in format expected by storage: YYYY_MM_DD_HHMMSS_title-domain.html
	timestamp := time.Now()
	filename := fmt.Sprintf("%s_%s-%s.html", 
		timestamp.Format("2006_01_02_150405"),
		nm.sanitizeFilename(title),
		nm.sanitizeFilename(parsedURL.Host))
	
	// Ensure sites directory exists
	sitesDir := filepath.Join(nm.storage.BasePath, "assets", "sites")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sites directory: %w", err)
	}
	
	// Process HTML to inline all external resources
	processedHTML := nm.inlineAllResources(string(htmlContent), websiteURL)
	
	// Save the archived file
	filePath := filepath.Join(sitesDir, filename)
	if err := os.WriteFile(filePath, []byte(processedHTML), 0644); err != nil {
		return nil, fmt.Errorf("failed to save archived file: %w", err)
	}
	
	// Create relative path for linking
	relativePath := filepath.Join("assets", "sites", filename)
	
	return &ArchiveInfo{
		Title:     title,
		FilePath:  relativePath,
		Timestamp: timestamp,
	}, nil
}

// extractTitle extracts the title from HTML content
func (nm *NoteManager) extractTitle(htmlContent, host string) string {
	// Simple regex to extract title
	titleRe := regexp.MustCompile(`<title[^>]*>([^<]*)</title>`)
	matches := titleRe.FindStringSubmatch(htmlContent)
	
	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
		return strings.TrimSpace(matches[1])
	}
	
	return host
}

// sanitizeFilename removes invalid characters from filenames
func (nm *NoteManager) sanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := re.ReplaceAllString(filename, "_")
	
	// Limit length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	
	return strings.Trim(sanitized, "_")
}

// inlineAllResources performs comprehensive resource inlining
func (nm *NoteManager) inlineAllResources(htmlContent, baseURL string) string {
	// Add archive header
	archiveHeader := fmt.Sprintf(`
<!-- ARCHIVED PAGE - Original URL: %s - Archived: %s -->
<div style="background: #fff3cd; border: 1px solid #ffeaa7; padding: 10px; margin: 10px 0; border-radius: 4px; font-family: Arial, sans-serif;">
	📄 <strong>Archived Page</strong> - Original: <a href="%s" target="_blank">%s</a> - Archived: %s
</div>
`, baseURL, time.Now().Format("2006-01-02 15:04:05"), baseURL, baseURL, time.Now().Format("2006-01-02 15:04:05"))
	
	// Parse base URL for resolving relative URLs
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("Warning: failed to parse base URL %s: %v", baseURL, err)
		return htmlContent
	}
	
	// Inline CSS stylesheets
	htmlContent = nm.inlineCSS(htmlContent, baseURLParsed)
	
	// Inline JavaScript files
	htmlContent = nm.inlineJavaScript(htmlContent, baseURLParsed)
	
	// Inline images as base64 data URIs
	htmlContent = nm.inlineImages(htmlContent, baseURLParsed)
	
	// Inline web fonts
	htmlContent = nm.inlineWebFonts(htmlContent, baseURLParsed)
	
	// Process inline CSS styles that may contain background images
	htmlContent = nm.inlineStyleAttributes(htmlContent, baseURLParsed)
	
	// Insert header after <body> tag
	bodyRe := regexp.MustCompile(`(<body[^>]*>)`)
	htmlContent = bodyRe.ReplaceAllString(htmlContent, `$1`+archiveHeader)
	
	return htmlContent
}

// inlineCSS inlines external CSS stylesheets
func (nm *NoteManager) inlineCSS(htmlContent string, baseURL *url.URL) string {
	// Match <link> tags for stylesheets
	linkRe := regexp.MustCompile(`<link[^>]*href=["']([^"']+)["'][^>]*rel=["']stylesheet["'][^>]*>|<link[^>]*rel=["']stylesheet["'][^>]*href=["']([^"']+)["'][^>]*>`)
	
	return linkRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract href value
		hrefRe := regexp.MustCompile(`href=["']([^"']+)["']`)
		hrefMatch := hrefRe.FindStringSubmatch(match)
		if len(hrefMatch) < 2 {
			return match // Keep original if we can't extract href
		}
		
		cssURL := hrefMatch[1]
		
		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, cssURL)
		if resolvedURL == "" {
			return match
		}
		
		// Download CSS content
		cssContent := nm.downloadResource(resolvedURL)
		if cssContent == "" {
			return match
		}
		
		// Process CSS to inline any @import and url() references
		processedCSS := nm.processCSS(cssContent, resolvedURL)
		
		return fmt.Sprintf(`<style type="text/css">
/* Inlined from: %s */
%s
</style>`, resolvedURL, processedCSS)
	})
}

// inlineJavaScript inlines external JavaScript files
func (nm *NoteManager) inlineJavaScript(htmlContent string, baseURL *url.URL) string {
	// Match <script> tags with src attributes
	scriptRe := regexp.MustCompile(`<script[^>]*src=["']([^"']+)["'][^>]*></script>`)
	
	return scriptRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract src value
		srcRe := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRe.FindStringSubmatch(match)
		if len(srcMatch) < 2 {
			return match
		}
		
		jsURL := srcMatch[1]
		
		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, jsURL)
		if resolvedURL == "" {
			return match
		}
		
		// Download JavaScript content
		jsContent := nm.downloadResource(resolvedURL)
		if jsContent == "" {
			return match
		}
		
		return fmt.Sprintf(`<script type="text/javascript">
/* Inlined from: %s */
%s
</script>`, resolvedURL, jsContent)
	})
}

// inlineImages inlines images as base64 data URIs
func (nm *NoteManager) inlineImages(htmlContent string, baseURL *url.URL) string {
	// Match <img> tags
	imgRe := regexp.MustCompile(`<img[^>]*src=["']([^"']+)["'][^>]*>`)
	
	htmlContent = imgRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract src value
		srcRe := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRe.FindStringSubmatch(match)
		if len(srcMatch) < 2 {
			return match
		}
		
		imgURL := srcMatch[1]
		
		// Skip data URIs
		if strings.HasPrefix(imgURL, "data:") {
			return match
		}
		
		log.Printf("Processing image: %s", imgURL)
		
		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, imgURL)
		if resolvedURL == "" {
			log.Printf("Failed to resolve image URL: %s", imgURL)
			return match
		}
		
		log.Printf("Resolved image URL: %s", resolvedURL)
		
		// Download and encode image
		dataURI := nm.downloadAndEncodeImage(resolvedURL)
		if dataURI == "" {
			log.Printf("Failed to download/encode image: %s", resolvedURL)
			return match
		}
		
		log.Printf("Successfully inlined image: %s (data URI length: %d)", resolvedURL, len(dataURI))
		
		// Replace src with data URI
		return srcRe.ReplaceAllString(match, fmt.Sprintf(`src="%s"`, dataURI))
	})
	
	// Also process JavaScript string references to images
	jsImgRe := regexp.MustCompile(`['"]([^'"]*\.(?:png|jpg|jpeg|gif|svg|webp))['"]`)
	htmlContent = jsImgRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		quote := match[0:1]
		imgURL := match[1 : len(match)-1]
		
		// Skip data URIs
		if strings.HasPrefix(imgURL, "data:") {
			return match
		}
		
		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, imgURL)
		if resolvedURL == "" {
			return match
		}
		
		// Download and encode image
		dataURI := nm.downloadAndEncodeImage(resolvedURL)
		if dataURI == "" {
			return match
		}
		
		return fmt.Sprintf(`%s%s%s`, quote, dataURI, quote)
	})
	
	return htmlContent
}

// inlineWebFonts inlines web fonts from CSS @font-face rules
func (nm *NoteManager) inlineWebFonts(htmlContent string, baseURL *url.URL) string {
	// This will be handled within CSS processing
	// Web fonts in @font-face rules will be inlined when CSS is processed
	return htmlContent
}

// inlineStyleAttributes processes inline style attributes to inline background images
func (nm *NoteManager) inlineStyleAttributes(htmlContent string, baseURL *url.URL) string {
	// Match style attributes
	styleRe := regexp.MustCompile(`style=["']([^"']*url\([^)]+\)[^"']*)["']`)
	
	return styleRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract the style content
		styleMatch := styleRe.FindStringSubmatch(match)
		if len(styleMatch) < 2 {
			return match
		}
		
		styleContent := styleMatch[1]
		quote := match[6:7] // Extract the quote character
		
		// Process URL references in the style
		processedStyle := nm.processInlineCSS(styleContent, baseURL.String())
		
		return fmt.Sprintf(`style=%s%s%s`, quote, processedStyle, quote)
	})
}

// processInlineCSS processes CSS content for inline styles
func (nm *NoteManager) processInlineCSS(cssContent, baseURLStr string) string {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return cssContent
	}
	
	// Process url() references
	urlRe := regexp.MustCompile(`url\(["']?([^"')\s]+)["']?\)`)
	return urlRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		urlMatch := urlRe.FindStringSubmatch(match)
		if len(urlMatch) < 2 {
			return match
		}
		
		resourceURL := urlMatch[1]
		
		// Skip data URIs
		if strings.HasPrefix(resourceURL, "data:") {
			return match
		}
		
		resolvedURL := nm.resolveURL(baseURL, resourceURL)
		if resolvedURL == "" {
			return match
		}
		
		// Download and encode the resource
		dataURI := nm.downloadAndEncodeImage(resolvedURL)
		if dataURI != "" {
			return fmt.Sprintf(`url("%s")`, dataURI)
		}
		
		return match
	})
}

// resolveURL resolves a relative URL against a base URL
func (nm *NoteManager) resolveURL(baseURL *url.URL, targetURL string) string {
	// Skip data URIs, mailto, tel, etc.
	if strings.Contains(targetURL, ":") && !strings.HasPrefix(targetURL, "http") && !strings.HasPrefix(targetURL, "//") {
		return ""
	}
	
	resolvedURL, err := baseURL.Parse(targetURL)
	if err != nil {
		log.Printf("Warning: failed to resolve URL %s against %s: %v", targetURL, baseURL, err)
		return ""
	}
	
	return resolvedURL.String()
}

// downloadResource downloads a resource and returns its content as string
func (nm *NoteManager) downloadResource(resourceURL string) string {
	resp, err := http.Get(resourceURL)
	if err != nil {
		log.Printf("Warning: failed to download resource %s: %v", resourceURL, err)
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		log.Printf("Warning: HTTP error %d downloading %s", resp.StatusCode, resourceURL)
		return ""
	}
	
	// Limit resource size to prevent memory issues (5MB max)
	const maxSize = 5 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize)
	
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("Warning: failed to read resource %s: %v", resourceURL, err)
		return ""
	}
	
	return string(content)
}

// downloadAndEncodeImage downloads an image and returns it as a base64 data URI
func (nm *NoteManager) downloadAndEncodeImage(imageURL string) string {
	resp, err := http.Get(imageURL)
	if err != nil {
		log.Printf("Warning: failed to download image %s: %v", imageURL, err)
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		log.Printf("Warning: HTTP error %d downloading image %s", resp.StatusCode, imageURL)
		return ""
	}
	
	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to determine from URL extension
		ext := strings.ToLower(path.Ext(imageURL))
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	
	// Skip very large images (1MB max for images)
	const maxImageSize = 1 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxImageSize)
	
	imageData, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("Warning: failed to read image %s: %v", imageURL, err)
		return ""
	}
	
	// Encode as base64 data URI
	encoded := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

// processCSS processes CSS content to inline @import and url() references
func (nm *NoteManager) processCSS(cssContent, cssURL string) string {
	cssBaseURL, err := url.Parse(cssURL)
	if err != nil {
		return cssContent
	}
	
	// Process @import rules
	importRe := regexp.MustCompile(`@import\s+(?:url\()?["']([^"']+)["'](?:\))?[^;]*;`)
	cssContent = importRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		importMatch := importRe.FindStringSubmatch(match)
		if len(importMatch) < 2 {
			return match
		}
		
		importURL := nm.resolveURL(cssBaseURL, importMatch[1])
		if importURL == "" {
			return match
		}
		
		importedCSS := nm.downloadResource(importURL)
		if importedCSS == "" {
			return match
		}
		
		// Recursively process imported CSS
		return fmt.Sprintf("/* Imported from: %s */\n%s", importURL, nm.processCSS(importedCSS, importURL))
	})
	
	// Process url() references (fonts, background images, etc.)
	urlRe := regexp.MustCompile(`url\(["']?([^"')\s]+)["']?\)`)
	cssContent = urlRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		urlMatch := urlRe.FindStringSubmatch(match)
		if len(urlMatch) < 2 {
			return match
		}
		
		resourceURL := urlMatch[1]
		
		// Skip data URIs
		if strings.HasPrefix(resourceURL, "data:") {
			return match
		}
		
		resolvedURL := nm.resolveURL(cssBaseURL, resourceURL)
		if resolvedURL == "" {
			return match
		}
		
		// Determine if this is likely an image or font
		ext := strings.ToLower(path.Ext(resourceURL))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".svg" || ext == ".webp"
		isFont := ext == ".woff" || ext == ".woff2" || ext == ".ttf" || ext == ".otf" || ext == ".eot"
		
		if isImage || isFont {
			// Convert to data URI
			dataURI := nm.downloadAndEncodeImage(resolvedURL)
			if dataURI != "" {
				return fmt.Sprintf(`url("%s")`, dataURI)
			}
		}
		
		return match
	})
	
	return cssContent
}

// GetBasePath returns the base path for this note manager
func (nm *NoteManager) GetBasePath() string {
	return nm.storage.BasePath
}

// SaveFile saves an uploaded file and returns the path
func (nm *NoteManager) SaveFile(filename string, data []byte, contentType string) (string, bool, error) {
	isImage := strings.HasPrefix(contentType, "image/")
	path, err := nm.storage.SaveFile(filename, data, isImage)
	return path, isImage, err
}

// GetArchivedLinks returns information about archived websites
func (nm *NoteManager) GetArchivedLinks() (map[string]interface{}, error) {
	return nm.storage.ListArchivedSites()
}

// DeleteArchivedSite deletes an archived website file
func (nm *NoteManager) DeleteArchivedSite(filename string) error {
	if err := nm.storage.DeleteArchivedSite(filename); err != nil {
		return err
	}

	// Update notes.md to mark references as deleted
	nm.mu.Lock()
	defer nm.mu.Unlock()

	changesMade := false
	for _, note := range nm.notes {
		if strings.Contains(note.Content, filename) {
			lines := strings.Split(note.Content, "\n")
			for i, line := range lines {
				if strings.Contains(line, filename) {
					lines[i] = fmt.Sprintf("~~%s~~ _(archived link deleted)_", line)
					changesMade = true
				}
			}
			note.Content = strings.Join(lines, "\n")
		}
	}

	if changesMade {
		nm.needsSave = true
		return nm.save()
	}

	return nil
}

// HasChanges returns true if the notes have unsaved changes
func (nm *NoteManager) HasChanges() bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.needsSave
}

// GetAllTasks returns all tasks across all notes
func (nm *NoteManager) GetAllTasks() []models.Task {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	var allTasks []models.Task
	for _, note := range nm.notes {
		for _, task := range note.Tasks {
			allTasks = append(allTasks, *task)
		}
	}
	return allTasks
}