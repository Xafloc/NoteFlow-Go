package services

import (
	"fmt"
	"strings"
	"sync"

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

// processArchiveLinks processes +http links in content (placeholder for future archiving feature)
func (nm *NoteManager) processArchiveLinks(content string) (string, error) {
	// TODO: Implement website archiving functionality
	// For now, just return the original content
	return content, nil
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