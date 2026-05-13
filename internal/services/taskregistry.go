package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
)

// TaskRegistryService manages cross-folder task synchronization
type TaskRegistryService struct {
	db         *DatabaseService
	noteManagers map[string]*NoteManager // folderPath -> NoteManager
	mu           sync.RWMutex
	syncTicker   *time.Ticker
	stopCh       chan struct{}
}

// NewTaskRegistryService creates a new task registry service
func NewTaskRegistryService() (*TaskRegistryService, error) {
	db, err := NewDatabaseService()
	if err != nil {
		return nil, fmt.Errorf("failed to create database service: %w", err)
	}

	service := &TaskRegistryService{
		db:           db,
		noteManagers: make(map[string]*NoteManager),
		stopCh:       make(chan struct{}),
	}

	// Start background sync every 30 seconds
	service.startBackgroundSync()

	return service, nil
}

// RegisterFolder registers a folder for cross-folder task management
func (trs *TaskRegistryService) RegisterFolder(folderPath string, noteManager *NoteManager) error {
	trs.mu.Lock()
	defer trs.mu.Unlock()

	// Register in database
	folder, err := trs.db.RegisterFolder(folderPath)
	if err != nil {
		return fmt.Errorf("failed to register folder in database: %w", err)
	}

	// Store note manager for this folder
	trs.noteManagers[folderPath] = noteManager

	// Initial sync of tasks for this folder
	if err := trs.syncFolderTasks(folder.ID, folderPath, noteManager); err != nil {
		log.Printf("Warning: failed initial sync for folder %s: %v", folderPath, err)
	}

	log.Printf("Registered folder for global task management: %s", folderPath)
	return nil
}

// syncFolderTasks synchronizes tasks for a specific folder
func (trs *TaskRegistryService) syncFolderTasks(folderID int, folderPath string, noteManager *NoteManager) error {
	// Get all tasks from the note manager
	tasks := noteManager.GetAllTasks()
	
	// Sync with database
	return trs.db.SyncFolderTasks(folderID, tasks)
}

// GetGlobalTasks returns all tasks across all registered folders
func (trs *TaskRegistryService) GetGlobalTasks() (*models.GlobalTasksResponse, error) {
	return trs.db.GetGlobalTasks()
}

// UpdateGlobalTaskCompletion updates task completion and syncs back to the note file
func (trs *TaskRegistryService) UpdateGlobalTaskCompletion(taskID int, completed bool) error {
	// First, get the task details to know which folder it belongs to
	globalTasks, err := trs.db.GetGlobalTasks()
	if err != nil {
		return fmt.Errorf("failed to get global tasks: %w", err)
	}

	var targetTask *models.GlobalTask
	for _, task := range globalTasks.Tasks {
		if task.ID == taskID {
			targetTask = &task
			break
		}
	}

	if targetTask == nil {
		return fmt.Errorf("task with ID %d not found", taskID)
	}

	// Update in database
	if err := trs.db.UpdateTaskCompletion(taskID, completed); err != nil {
		return fmt.Errorf("failed to update task in database: %w", err)
	}

	// Update in the corresponding note file
	trs.mu.RLock()
	noteManager, exists := trs.noteManagers[targetTask.FolderPath]
	trs.mu.RUnlock()

	if exists {
		// Find and update the task in the note manager
		tasks := noteManager.GetAllTasks()
		for _, task := range tasks {
			if task.Text == targetTask.Content {
				if err := noteManager.UpdateTask(task.Index, completed); err != nil {
					log.Printf("Warning: failed to update task in note file: %v", err)
				}
				break
			}
		}
	}

	return nil
}

// startBackgroundSync starts a background goroutine to periodically sync all folders
func (trs *TaskRegistryService) startBackgroundSync() {
	trs.syncTicker = time.NewTicker(30 * time.Second)
	
	go func() {
		for {
			select {
			case <-trs.syncTicker.C:
				trs.performBackgroundSync()
			case <-trs.stopCh:
				return
			}
		}
	}()
}

// performBackgroundSync syncs all registered folders and cleans up stale entries
func (trs *TaskRegistryService) performBackgroundSync() {
	trs.mu.RLock()
	defer trs.mu.RUnlock()

	folders, err := trs.db.GetActiveFolders()
	if err != nil {
		log.Printf("Warning: failed to get active folders for sync: %v", err)
		return
	}

	var foldersToRemove []int

	for _, folder := range folders {
		// Check if folder still exists and has notes.md
		if !trs.validateFolder(folder.Path) {
			foldersToRemove = append(foldersToRemove, folder.ID)
			log.Printf("Marking stale folder for removal: %s", folder.Path)
			continue
		}

		noteManager, exists := trs.noteManagers[folder.Path]
		if !exists {
			continue
		}

		// Check if the notes file has been modified since last sync
		if noteManager.HasChanges() || time.Since(folder.LastScan) > 5*time.Minute {
			if err := trs.syncFolderTasks(folder.ID, folder.Path, noteManager); err != nil {
				log.Printf("Warning: failed to sync folder %s: %v", folder.Path, err)
			}
		}
	}

	// Clean up stale folders
	for _, folderID := range foldersToRemove {
		if err := trs.db.RemoveFolder(folderID); err != nil {
			log.Printf("Warning: failed to remove stale folder %d: %v", folderID, err)
		}
	}
}

// ForceSync forces a sync of all registered folders and cleans up stale entries
func (trs *TaskRegistryService) ForceSync() error {
	trs.mu.RLock()
	defer trs.mu.RUnlock()

	folders, err := trs.db.GetActiveFolders()
	if err != nil {
		return fmt.Errorf("failed to get active folders: %w", err)
	}

	var foldersToRemove []int

	for _, folder := range folders {
		// Check if folder still exists and has notes.md
		if !trs.validateFolder(folder.Path) {
			foldersToRemove = append(foldersToRemove, folder.ID)
			log.Printf("Marking stale folder for removal during force sync: %s", folder.Path)
			continue
		}

		noteManager, exists := trs.noteManagers[folder.Path]
		if !exists {
			continue
		}

		if err := trs.syncFolderTasks(folder.ID, folder.Path, noteManager); err != nil {
			log.Printf("Warning: failed to sync folder %s: %v", folder.Path, err)
		}
	}

	// Clean up stale folders
	for _, folderID := range foldersToRemove {
		if err := trs.db.RemoveFolder(folderID); err != nil {
			log.Printf("Warning: failed to remove stale folder %d: %v", folderID, err)
		} else {
			log.Printf("Removed stale folder ID %d from global task registry", folderID)
		}
	}

	return nil
}

// GetActiveFolders returns all active registered folders
func (trs *TaskRegistryService) GetActiveFolders() ([]models.FolderRegistry, error) {
	return trs.db.GetActiveFolders()
}

// AddFolderByPath registers a user-supplied path with the global task graph.
// Accepts any absolute or absolute-able path the user can type — no admin
// or sandbox restrictions, matching the existing implicit auto-register
// behavior. Validates that the path exists and is a directory; creates a
// fresh NoteManager for it (which also creates an empty notes.md if one
// doesn't exist) and runs an initial sync.
//
// If the folder is already registered (active or not), this resurrects /
// updates the existing row rather than creating a duplicate — the
// underlying db.RegisterFolder already upserts on the unique path column.
func (trs *TaskRegistryService) AddFolderByPath(folderPath string) (*models.FolderRegistry, error) {
	abs, err := filepath.Abs(folderPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", abs)
		}
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	noteManager, err := NewNoteManager(abs)
	if err != nil {
		return nil, fmt.Errorf("open notes.md at %s: %w", abs, err)
	}

	trs.mu.Lock()
	defer trs.mu.Unlock()

	folder, err := trs.db.RegisterFolder(abs)
	if err != nil {
		return nil, fmt.Errorf("register folder in db: %w", err)
	}
	trs.noteManagers[abs] = noteManager

	if err := trs.syncFolderTasks(folder.ID, abs, noteManager); err != nil {
		log.Printf("Warning: initial sync for added folder %s: %v", abs, err)
	}

	log.Printf("User added folder to global task registry: %s", abs)
	return folder, nil
}

// SyncFolderByID re-syncs a single folder's tasks. Used by the per-folder
// "Sync" button in the global tasks UI — useful when the user has edited
// notes.md externally and wants the central view to catch up immediately
// instead of waiting for the 30s background tick.
func (trs *TaskRegistryService) SyncFolderByID(folderID int) error {
	folder, err := trs.db.GetFolderByID(folderID)
	if err != nil {
		return fmt.Errorf("folder %d not found: %w", folderID, err)
	}
	if !folder.Active {
		return fmt.Errorf("folder %d is forgotten — re-add it first", folderID)
	}

	trs.mu.Lock()
	noteManager, exists := trs.noteManagers[folder.Path]
	if !exists {
		// Folder was registered but we don't have a live NoteManager — that
		// happens for folders the current process didn't open itself (e.g.
		// folders registered by an earlier session, or auto-discovered after
		// a path move). Lazily create one now.
		nm, err := NewNoteManager(folder.Path)
		if err != nil {
			trs.mu.Unlock()
			return fmt.Errorf("open notes.md at %s: %w", folder.Path, err)
		}
		trs.noteManagers[folder.Path] = nm
		noteManager = nm
	}
	trs.mu.Unlock()

	return trs.syncFolderTasks(folder.ID, folder.Path, noteManager)
}

// ForgetFolder removes a folder from active tracking (soft-delete via
// db.SoftRemoveFolder) and evicts its NoteManager from this process's
// in-memory cache. The folder row remains in the DB with active=0 so
// re-adding the same path later resurfaces the same id.
func (trs *TaskRegistryService) ForgetFolder(folderID int) error {
	folder, err := trs.db.GetFolderByID(folderID)
	if err != nil {
		return fmt.Errorf("folder %d not found: %w", folderID, err)
	}
	if err := trs.db.SoftRemoveFolder(folderID); err != nil {
		return fmt.Errorf("soft-remove folder: %w", err)
	}
	trs.mu.Lock()
	delete(trs.noteManagers, folder.Path)
	trs.mu.Unlock()
	log.Printf("User forgot folder %s (id=%d) — kept as inactive audit row", folder.Path, folderID)
	return nil
}

// validateFolder checks if a folder still exists and has notes.md
func (trs *TaskRegistryService) validateFolder(folderPath string) bool {
	// Check if folder exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		return false
	}

	// Check if notes.md exists in the folder
	notesPath := filepath.Join(folderPath, "notes.md")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// Close stops the background sync and closes the database connection
func (trs *TaskRegistryService) Close() error {
	if trs.syncTicker != nil {
		trs.syncTicker.Stop()
	}
	
	close(trs.stopCh)
	
	if trs.db != nil {
		return trs.db.Close()
	}
	
	return nil
}