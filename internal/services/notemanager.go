package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	
	// Process HTML to inline resources (simplified version)
	processedHTML := nm.inlineBasicResources(string(htmlContent), websiteURL)
	
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

// inlineBasicResources performs basic resource inlining (simplified version)
func (nm *NoteManager) inlineBasicResources(htmlContent, baseURL string) string {
	// Add a simple header indicating this is an archived page
	archiveHeader := fmt.Sprintf(`
<!-- ARCHIVED PAGE - Original URL: %s - Archived: %s -->
<div style="background: #fff3cd; border: 1px solid #ffeaa7; padding: 10px; margin: 10px 0; border-radius: 4px; font-family: Arial, sans-serif;">
	📄 <strong>Archived Page</strong> - Original: <a href="%s" target="_blank">%s</a> - Archived: %s
</div>
`, baseURL, time.Now().Format("2006-01-02 15:04:05"), baseURL, baseURL, time.Now().Format("2006-01-02 15:04:05"))
	
	// Insert header after <body> tag
	bodyRe := regexp.MustCompile(`(<body[^>]*>)`)
	htmlContent = bodyRe.ReplaceAllString(htmlContent, `$1`+archiveHeader)
	
	// For now, return HTML with archive header
	// TODO: Add full resource inlining (CSS, images, etc.)
	return htmlContent
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