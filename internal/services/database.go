package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/darren/noteflow-go/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// DatabaseService handles SQLite operations for task registry
type DatabaseService struct {
	db   *sql.DB
	path string
}

// NewDatabaseService creates a new database service
func NewDatabaseService() (*DatabaseService, error) {
	// Create config directory if it doesn't exist
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "noteflow")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	dbPath := filepath.Join(configDir, "tasks.db")

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	service := &DatabaseService{
		db:   db,
		path: dbPath,
	}

	// Initialize database schema
	if err := service.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return service, nil
}

// migrate creates the database schema
func (ds *DatabaseService) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		last_scan DATETIME DEFAULT CURRENT_TIMESTAMP,
		active BOOLEAN DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		folder_id INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		line_number INTEGER NOT NULL,
		content TEXT NOT NULL,
		completed BOOLEAN DEFAULT 0,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_folder ON tasks(folder_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_completed ON tasks(completed);
	CREATE INDEX IF NOT EXISTS idx_tasks_folder_file ON tasks(folder_id, file_path);
	`

	_, err := ds.db.Exec(schema)
	return err
}

// RegisterFolder registers a folder in the database
func (ds *DatabaseService) RegisterFolder(folderPath string) (*models.FolderRegistry, error) {
	// Check if folder already exists
	var folder models.FolderRegistry
	err := ds.db.QueryRow(`
		SELECT id, path, last_scan, active 
		FROM folders 
		WHERE path = ?`, folderPath).Scan(
		&folder.ID, &folder.Path, &folder.LastScan, &folder.Active)

	if err == nil {
		// Update as active if it was inactive
		if !folder.Active {
			_, err = ds.db.Exec(`UPDATE folders SET active = 1 WHERE id = ?`, folder.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to reactivate folder: %w", err)
			}
			folder.Active = true
		}
		return &folder, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing folder: %w", err)
	}

	// Insert new folder
	result, err := ds.db.Exec(`
		INSERT INTO folders (path, last_scan, active) 
		VALUES (?, ?, 1)`, folderPath, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to register folder: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get folder ID: %w", err)
	}

	folder = models.FolderRegistry{
		ID:       int(id),
		Path:     folderPath,
		LastScan: time.Now(),
		Active:   true,
	}

	return &folder, nil
}

// SyncFolderTasks scans a folder and updates its tasks in the database
func (ds *DatabaseService) SyncFolderTasks(folderID int, tasks []models.Task) error {
	tx, err := ds.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing tasks for this folder
	_, err = tx.Exec(`DELETE FROM tasks WHERE folder_id = ?`, folderID)
	if err != nil {
		return fmt.Errorf("failed to clear existing tasks: %w", err)
	}

	// Insert new tasks
	stmt, err := tx.Prepare(`
		INSERT INTO tasks (folder_id, file_path, line_number, content, completed, last_updated)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare task insert: %w", err)
	}
	defer stmt.Close()

	for i, task := range tasks {
		_, err = stmt.Exec(
			folderID,
			"notes.md", // All tasks are in notes.md for now
			i,          // Use task index as line number for uniqueness
			task.Text,
			task.Checked,
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert task: %w", err)
		}
	}

	// Update folder last_scan time
	_, err = tx.Exec(`UPDATE folders SET last_scan = ? WHERE id = ?`, time.Now(), folderID)
	if err != nil {
		return fmt.Errorf("failed to update folder scan time: %w", err)
	}

	return tx.Commit()
}

// GetGlobalTasks retrieves all tasks across all active folders
func (ds *DatabaseService) GetGlobalTasks() (*models.GlobalTasksResponse, error) {
	// Get tasks with folder information
	rows, err := ds.db.Query(`
		SELECT t.id, t.folder_id, t.file_path, t.line_number, t.content, 
			   t.completed, t.last_updated, f.path
		FROM tasks t
		JOIN folders f ON t.folder_id = f.id
		WHERE f.active = 1
		ORDER BY f.path, t.completed, t.last_updated DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.GlobalTask
	for rows.Next() {
		var task models.GlobalTask
		var lastUpdated string
		err := rows.Scan(
			&task.ID, &task.FolderID, &task.FilePath, &task.LineNumber,
			&task.Content, &task.Completed, &lastUpdated, &task.FolderPath)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		// Parse timestamp
		if t, err := time.Parse("2006-01-02 15:04:05.000000-07:00", lastUpdated); err == nil {
			task.LastUpdated = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", lastUpdated); err == nil {
			task.LastUpdated = t
		}
		tasks = append(tasks, task)
	}

	// Get summaries per folder
	summaries, err := ds.getTaskSummaries()
	if err != nil {
		return nil, fmt.Errorf("failed to get task summaries: %w", err)
	}

	return &models.GlobalTasksResponse{
		Tasks:     tasks,
		Summaries: summaries,
		Total:     len(tasks),
	}, nil
}

// getTaskSummaries generates task summaries grouped by folder
func (ds *DatabaseService) getTaskSummaries() ([]models.TaskSummary, error) {
	rows, err := ds.db.Query(`
		SELECT f.path,
			   COUNT(t.id) as total_tasks,
			   SUM(CASE WHEN t.completed = 1 THEN 1 ELSE 0 END) as completed_tasks,
			   SUM(CASE WHEN t.completed = 0 THEN 1 ELSE 0 END) as pending_tasks,
			   MAX(t.last_updated) as last_updated
		FROM folders f
		LEFT JOIN tasks t ON f.id = t.folder_id
		WHERE f.active = 1
		GROUP BY f.id, f.path
		ORDER BY f.path`)
	if err != nil {
		return nil, fmt.Errorf("failed to query task summaries: %w", err)
	}
	defer rows.Close()

	var summaries []models.TaskSummary
	for rows.Next() {
		var summary models.TaskSummary
		var lastUpdated sql.NullString
		err := rows.Scan(
			&summary.FolderPath, &summary.TotalTasks,
			&summary.CompletedTasks, &summary.PendingTasks, &lastUpdated)
		if err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}
		if lastUpdated.Valid {
			// Try to parse the timestamp string
			if t, err := time.Parse("2006-01-02 15:04:05.000000-07:00", lastUpdated.String); err == nil {
				summary.LastUpdated = t
			} else if t, err := time.Parse("2006-01-02 15:04:05", lastUpdated.String); err == nil {
				summary.LastUpdated = t
			}
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// UpdateTaskCompletion updates the completion status of a specific task
func (ds *DatabaseService) UpdateTaskCompletion(taskID int, completed bool) error {
	_, err := ds.db.Exec(`
		UPDATE tasks 
		SET completed = ?, last_updated = ? 
		WHERE id = ?`, completed, time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("failed to update task completion: %w", err)
	}
	return nil
}

// GetActiveFolders returns all active registered folders
func (ds *DatabaseService) GetActiveFolders() ([]models.FolderRegistry, error) {
	rows, err := ds.db.Query(`
		SELECT id, path, last_scan, active 
		FROM folders 
		WHERE active = 1 
		ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("failed to query folders: %w", err)
	}
	defer rows.Close()

	var folders []models.FolderRegistry
	for rows.Next() {
		var folder models.FolderRegistry
		err := rows.Scan(&folder.ID, &folder.Path, &folder.LastScan, &folder.Active)
		if err != nil {
			return nil, fmt.Errorf("failed to scan folder: %w", err)
		}
		folders = append(folders, folder)
	}

	return folders, nil
}

// RemoveFolder removes a folder and all its associated tasks from the database
func (ds *DatabaseService) RemoveFolder(folderID int) error {
	tx, err := ds.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete all tasks for this folder
	_, err = tx.Exec("DELETE FROM tasks WHERE folder_id = ?", folderID)
	if err != nil {
		return fmt.Errorf("failed to delete tasks for folder %d: %w", folderID, err)
	}

	// Delete the folder record
	_, err = tx.Exec("DELETE FROM folders WHERE id = ?", folderID)
	if err != nil {
		return fmt.Errorf("failed to delete folder %d: %w", folderID, err)
	}

	return tx.Commit()
}

// Close closes the database connection
func (ds *DatabaseService) Close() error {
	return ds.db.Close()
}