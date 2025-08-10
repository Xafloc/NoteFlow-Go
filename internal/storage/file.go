package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/darren/noteflow-go/internal/models"
)

// FileStorage handles file-based operations
type FileStorage struct {
	BasePath string
	mu       sync.RWMutex // Protects concurrent file access
}

// NewFileStorage creates a new file storage instance
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{
		BasePath: basePath,
	}
}

// EnsureDirectories creates necessary directories
func (fs *FileStorage) EnsureDirectories() error {
	directories := []string{
		"assets",
		"assets/images", 
		"assets/files",
		"assets/sites",
	}

	for _, dir := range directories {
		fullPath := filepath.Join(fs.BasePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}
	return nil
}

// GetNotesFilePath returns the path to the notes.md file
func (fs *FileStorage) GetNotesFilePath() string {
	return filepath.Join(fs.BasePath, "notes.md")
}

// LoadNotes loads all notes from the notes.md file
func (fs *FileStorage) LoadNotes() ([]*models.Note, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	notesPath := fs.GetNotesFilePath()
	
	// Create notes.md if it doesn't exist
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		if err := os.WriteFile(notesPath, []byte(""), 0644); err != nil {
			return nil, fmt.Errorf("failed to create notes.md: %w", err)
		}
		return []*models.Note{}, nil
	}

	data, err := os.ReadFile(notesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read notes.md: %w", err)
	}

	// Handle different encodings
	content := string(data)
	if content == "" {
		return []*models.Note{}, nil
	}

	return fs.parseNotes(content)
}

// parseNotes parses the raw content into Note objects
func (fs *FileStorage) parseNotes(content string) ([]*models.Note, error) {
	var notes []*models.Note
	
	// Split by note separator
	rawNotes := strings.Split(content, models.NoteSeparator)
	
	for _, rawNote := range rawNotes {
		rawNote = strings.TrimSpace(rawNote)
		if rawNote == "" {
			continue
		}
		
		// Only process notes that start with markdown header
		if strings.HasPrefix(rawNote, "## ") {
			note, err := models.NewNoteFromText(rawNote)
			if err != nil {
				// Log error but continue processing other notes
				continue
			}
			notes = append(notes, note)
		}
	}
	
	return notes, nil
}

// SaveNotes saves all notes to the notes.md file
func (fs *FileStorage) SaveNotes(notes []*models.Note) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var rendered []string
	for _, note := range notes {
		rendered = append(rendered, note.Render())
	}
	
	content := strings.Join(rendered, models.NoteSeparator)
	notesPath := fs.GetNotesFilePath()
	
	return os.WriteFile(notesPath, []byte(content), 0644)
}

// SaveFile saves an uploaded file to the appropriate directory
func (fs *FileStorage) SaveFile(filename string, data []byte, isImage bool) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var subDir string
	if isImage {
		subDir = "images"
	} else {
		subDir = "files"
	}

	assetsDir := filepath.Join(fs.BasePath, "assets", subDir)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create assets directory: %w", err)
	}

	filePath := filepath.Join(assetsDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return relative path for web serving
	return fmt.Sprintf("/assets/%s/%s", subDir, filename), nil
}

// DeleteFile deletes a file from the assets directory
func (fs *FileStorage) DeleteFile(relativePath string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Convert relative path to absolute path
	// relativePath format: "/assets/files/filename.ext"
	if !strings.HasPrefix(relativePath, "/assets/") {
		return fmt.Errorf("invalid file path: %s", relativePath)
	}

	// Remove leading slash and join with base path
	fullPath := filepath.Join(fs.BasePath, strings.TrimPrefix(relativePath, "/"))
	
	// Ensure the file is within our assets directory for security
	absBasePath, err := filepath.Abs(fs.BasePath)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}
	
	absFilePath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	
	if !strings.HasPrefix(absFilePath, filepath.Join(absBasePath, "assets")) {
		return fmt.Errorf("file path outside assets directory: %s", relativePath)
	}

	return os.Remove(absFilePath)
}

// ListArchivedSites returns a list of archived website files
func (fs *FileStorage) ListArchivedSites() (map[string]interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	sitesPath := filepath.Join(fs.BasePath, "assets", "sites")
	entries, err := os.ReadDir(sitesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("failed to read sites directory: %w", err)
	}

	linkGroups := make(map[string]interface{})
	
	// Filter for HTML files and group by domain
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
			// Parse filename: YYYY_MM_DD_HHMMSS_title-domain.html
			parts := strings.Split(strings.TrimSuffix(entry.Name(), ".html"), "_")
			if len(parts) >= 4 {
				// Extract domain from the last part after the dash
				lastPart := parts[len(parts)-1]
				if dashIndex := strings.LastIndex(lastPart, "-"); dashIndex != -1 {
					domain := lastPart[dashIndex+1:]
					
					if linkGroups[domain] == nil {
						linkGroups[domain] = map[string]interface{}{
							"domain":   domain,
							"archives": []map[string]string{},
						}
					}
					
					// Add archive info
					domainData := linkGroups[domain].(map[string]interface{})
					archives := domainData["archives"].([]map[string]string)
					archives = append(archives, map[string]string{
						"timestamp": strings.Join(parts[:3], "_"),
						"filename":  entry.Name(),
					})
					domainData["archives"] = archives
				}
			}
		}
	}

	return linkGroups, nil
}

// DeleteArchivedSite deletes an archived website file and its metadata
func (fs *FileStorage) DeleteArchivedSite(filename string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	sitesPath := filepath.Join(fs.BasePath, "assets", "sites")
	
	// Delete HTML file
	htmlPath := filepath.Join(sitesPath, filename)
	if err := os.Remove(htmlPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete HTML file: %w", err)
	}

	// Delete tags file if it exists
	tagsPath := strings.TrimSuffix(htmlPath, ".html") + ".tags"
	if err := os.Remove(tagsPath); err != nil && !os.IsNotExist(err) {
		// Non-critical error, log but don't fail
	}

	return nil
}