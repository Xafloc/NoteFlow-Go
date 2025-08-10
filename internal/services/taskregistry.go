package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/darren/noteflow-go/internal/models"
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

// performBackgroundSync syncs all registered folders
func (trs *TaskRegistryService) performBackgroundSync() {
	trs.mu.RLock()
	defer trs.mu.RUnlock()

	folders, err := trs.db.GetActiveFolders()
	if err != nil {
		log.Printf("Warning: failed to get active folders for sync: %v", err)
		return
	}

	for _, folder := range folders {
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
}

// ForceSync forces a sync of all registered folders
func (trs *TaskRegistryService) ForceSync() error {
	trs.mu.RLock()
	defer trs.mu.RUnlock()

	folders, err := trs.db.GetActiveFolders()
	if err != nil {
		return fmt.Errorf("failed to get active folders: %w", err)
	}

	for _, folder := range folders {
		noteManager, exists := trs.noteManagers[folder.Path]
		if !exists {
			continue
		}

		if err := trs.syncFolderTasks(folder.ID, folder.Path, noteManager); err != nil {
			log.Printf("Warning: failed to sync folder %s: %v", folder.Path, err)
		}
	}

	return nil
}

// GetActiveFolders returns all active registered folders
func (trs *TaskRegistryService) GetActiveFolders() ([]models.FolderRegistry, error) {
	return trs.db.GetActiveFolders()
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