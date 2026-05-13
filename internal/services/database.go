package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	_ "modernc.org/sqlite"
)

// taskHashCheckboxRE matches the checkbox marker inside a task line. We
// normalize it out before hashing so a task's identity does not depend on
// its completion state — toggling `[ ]` ↔ `[x]` must not change the hash.
var taskHashCheckboxRE = regexp.MustCompile(`\[[ xX]\]`)

// normalizeForHash returns the canonical form of task text used for hashing:
// the checkbox marker is replaced with a placeholder so completion state
// doesn't influence task identity.
func normalizeForHash(text string) string {
	return taskHashCheckboxRE.ReplaceAllString(text, "[]")
}

// DatabaseService handles SQLite operations for task registry
type DatabaseService struct {
	db   *sql.DB
	path string
}

// DefaultDatabasePath returns the conventional location of the cross-project
// task DB: ~/.config/noteflow/tasks.db. It is exported so subcommands and
// tests can target the same file the server uses (or, for tests, choose not
// to).
func DefaultDatabasePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "noteflow", "tasks.db"), nil
}

// NewDatabaseService creates a new database service at the default path.
func NewDatabaseService() (*DatabaseService, error) {
	dbPath, err := DefaultDatabasePath()
	if err != nil {
		return nil, err
	}
	return NewDatabaseServiceAt(dbPath)
}

// NewDatabaseServiceAt opens (or creates) a task DB at the given path. The
// containing directory is created if missing. Used by tests and by any
// subcommand that needs to inject a different path.
func NewDatabaseServiceAt(dbPath string) (*DatabaseService, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign-key enforcement so the ON DELETE CASCADE in the tasks
	// table actually fires. SQLite leaves FKs off by default per connection.
	// See docs/20260512_task_db_schema.md §7.
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
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

// migrate creates the database schema and adds any missing columns.
//
// `task_hash` was added 2026-05-12 to give tasks IDs stable across syncs
// — see docs/20260512_task_db_schema.md §7. New DBs get the column from
// the CREATE TABLE; pre-existing DBs get it via the addColumnIfMissing
// pass below.
func (ds *DatabaseService) migrate() error {
	// Step 1: create core tables (no-op if they already exist) and the
	// indexes that don't depend on later-added columns. The task_hash index
	// is created in step 3 below, AFTER the column is guaranteed to exist.
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
		task_hash TEXT,
		FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_folder ON tasks(folder_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_completed ON tasks(completed);
	CREATE INDEX IF NOT EXISTS idx_tasks_folder_file ON tasks(folder_id, file_path);
	`
	if _, err := ds.db.Exec(schema); err != nil {
		return err
	}

	// Step 2: ensure the task_hash column exists. New DBs got it from the
	// CREATE TABLE above (no-op here); older DBs predate the column and need
	// an ALTER. This must run before any operation references task_hash —
	// including the index in step 3.
	if err := ds.addColumnIfMissing("tasks", "task_hash", "TEXT"); err != nil {
		return err
	}

	// Step 3: create the task_hash index now that the column is guaranteed
	// to exist on both fresh and migrated databases.
	if _, err := ds.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_hash ON tasks(folder_id, task_hash)`); err != nil {
		return err
	}

	// Step 4: saved views for the `noteflow tasks` CLI (Goal 2 — "Save common
	// queries as views"). Independent of the tasks table; no FK because views
	// reference filter shapes, not specific tasks.
	if _, err := ds.db.Exec(`
		CREATE TABLE IF NOT EXISTS task_views (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			filters TEXT NOT NULL,
			created DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return err
	}
	return nil
}

// SaveView upserts a named view storing a JSON-encoded filter blob.
func (ds *DatabaseService) SaveView(name, filters string) error {
	_, err := ds.db.Exec(`
		INSERT INTO task_views (name, filters) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET filters = excluded.filters
	`, name, filters)
	return err
}

// GetView returns the stored filter blob for a view name, or sql.ErrNoRows
// if no view with that name exists.
func (ds *DatabaseService) GetView(name string) (string, error) {
	var filters string
	err := ds.db.QueryRow(`SELECT filters FROM task_views WHERE name = ?`, name).Scan(&filters)
	return filters, err
}

// ListViews returns all saved view names in alphabetical order.
func (ds *DatabaseService) ListViews() ([]string, error) {
	rows, err := ds.db.Query(`SELECT name FROM task_views ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// DeleteView removes a saved view. Idempotent — deleting a non-existent view
// returns no error.
func (ds *DatabaseService) DeleteView(name string) error {
	_, err := ds.db.Exec(`DELETE FROM task_views WHERE name = ?`, name)
	return err
}

// addColumnIfMissing is a tiny one-shot migration helper. SQLite's
// `ALTER TABLE ADD COLUMN` errors if the column already exists, so we
// check first via PRAGMA table_info. This is enough for the small set
// of forward-compatible additions we expect in the near term; if the
// schema grows more, replace this with a real schema_version table.
func (ds *DatabaseService) addColumnIfMissing(table, column, sqlType string) error {
	rows, err := ds.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("check columns on %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notnull, pk  int
			dflt         sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan column info: %w", err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = ds.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, sqlType))
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// ComputeTaskHashes returns one hash per task, using a 12-char hex prefix of
// sha256(text). Duplicate-text tasks within the same folder are disambiguated
// by an occurrence suffix (#1, #2, ...) so each task gets a unique key.
// Folder isolation is provided by the (folder_id, task_hash) compound key,
// so the folder path does not need to enter the hash.
//
// Exported so the CLI (and any future RPC) can look up tasks by the same
// stable identity the sync writes.
func ComputeTaskHashes(tasks []models.Task) []string {
	out := make([]string, len(tasks))
	seen := make(map[string]int, len(tasks))
	for i, t := range tasks {
		h := sha256.Sum256([]byte(normalizeForHash(t.Text)))
		base := hex.EncodeToString(h[:])[:12]
		seen[base]++
		if seen[base] == 1 {
			out[i] = base
		} else {
			out[i] = fmt.Sprintf("%s#%d", base, seen[base]-1)
		}
	}
	return out
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

// SyncFolderTasks reconciles the DB with the current task list for a folder.
//
// Unlike the previous delete-and-rewrite implementation, this keeps `tasks.id`
// stable across syncs for any task whose text hasn't changed: each task gets
// a content hash (see ComputeTaskHashes), and the sync is an upsert keyed on
// (folder_id, task_hash). Tasks present in the previous sync but absent from
// the current list are deleted. `last_updated` only advances when content or
// completion actually changes — so it now means "task last modified," not
// "row last touched by any sync."
//
// This is the foundation for Goal 2's bidirectional integrity and CLI/UI
// references to tasks by ID — see docs/20260512_task_db_schema.md §7.
func (ds *DatabaseService) SyncFolderTasks(folderID int, tasks []models.Task) error {
	hashes := ComputeTaskHashes(tasks)

	tx, err := ds.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop any legacy rows for this folder that pre-date the task_hash column.
	// Once those are gone the upsert path is the only way rows get created.
	if _, err := tx.Exec(`DELETE FROM tasks WHERE folder_id = ? AND task_hash IS NULL`, folderID); err != nil {
		return fmt.Errorf("clear legacy rows: %w", err)
	}

	// Pull existing hashes so we know what to delete.
	rows, err := tx.Query(`SELECT task_hash FROM tasks WHERE folder_id = ?`, folderID)
	if err != nil {
		return fmt.Errorf("list existing hashes: %w", err)
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var h sql.NullString
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			return fmt.Errorf("scan hash: %w", err)
		}
		if h.Valid {
			existing[h.String] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	now := time.Now()
	currentSet := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		currentSet[h] = true
	}

	// Delete rows no longer present in the current task list.
	for h := range existing {
		if !currentSet[h] {
			if _, err := tx.Exec(`DELETE FROM tasks WHERE folder_id = ? AND task_hash = ?`, folderID, h); err != nil {
				return fmt.Errorf("delete stale task: %w", err)
			}
		}
	}

	// Upsert: UPDATE if a row with this (folder_id, task_hash) exists, else INSERT.
	// last_updated advances only when content or completed actually changes — a
	// CASE expression in the UPDATE handles that without an extra round trip.
	updateStmt, err := tx.Prepare(`
		UPDATE tasks
		SET content = ?2,
		    completed = ?3,
		    line_number = ?4,
		    last_updated = CASE WHEN content != ?2 OR completed != ?3 THEN ?5 ELSE last_updated END
		WHERE folder_id = ?1 AND task_hash = ?6`)
	if err != nil {
		return fmt.Errorf("prepare update: %w", err)
	}
	defer updateStmt.Close()

	insertStmt, err := tx.Prepare(`
		INSERT INTO tasks (folder_id, file_path, line_number, content, completed, last_updated, task_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	for i, task := range tasks {
		h := hashes[i]
		if existing[h] {
			if _, err := updateStmt.Exec(folderID, task.Text, task.Checked, i, now, h); err != nil {
				return fmt.Errorf("update task %s: %w", h, err)
			}
		} else {
			if _, err := insertStmt.Exec(folderID, "notes.md", i, task.Text, task.Checked, now, h); err != nil {
				return fmt.Errorf("insert task %s: %w", h, err)
			}
		}
	}

	if _, err := tx.Exec(`UPDATE folders SET last_scan = ? WHERE id = ?`, now, folderID); err != nil {
		return fmt.Errorf("update folder scan time: %w", err)
	}

	return tx.Commit()
}

// TaskHashFromText is exported so callers (CLI, future UI surfaces) can look
// up a task by stable hash. Same algorithm as ComputeTaskHashes' base hash,
// without the duplicate-suffix logic — callers needing disambiguation must
// pass the full hash including any `#N` suffix.
//
// The checkbox marker is normalized out before hashing, so a task's hash
// does not change when it is toggled between `[ ]` and `[x]`.
func TaskHashFromText(text string) string {
	h := sha256.Sum256([]byte(normalizeForHash(text)))
	return hex.EncodeToString(h[:])[:12]
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

// SoftRemoveFolder marks a folder inactive without deleting any data, and
// also clears that folder's task rows. Used for user-initiated "Forget" —
// preserves the audit row (active=0) so re-adding the same folder later
// resurrects the same id (per the existing RegisterFolder upsert).
//
// Distinct from RemoveFolder: stale-folder cleanup (when the path no
// longer exists on disk) still hard-deletes via RemoveFolder because
// there's nothing to preserve. User-initiated forgets soft-delete so
// intent + history are recoverable.
func (ds *DatabaseService) SoftRemoveFolder(folderID int) error {
	tx, err := ds.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tasks WHERE folder_id = ?", folderID); err != nil {
		return fmt.Errorf("clear tasks for folder %d: %w", folderID, err)
	}
	if _, err := tx.Exec("UPDATE folders SET active = 0 WHERE id = ?", folderID); err != nil {
		return fmt.Errorf("deactivate folder %d: %w", folderID, err)
	}
	return tx.Commit()
}

// GetFolderByID returns a single folder regardless of active state, or
// sql.ErrNoRows if none exists. Used by the per-folder sync/forget paths
// so they can validate the ID before doing work.
func (ds *DatabaseService) GetFolderByID(folderID int) (*models.FolderRegistry, error) {
	var f models.FolderRegistry
	err := ds.db.QueryRow(
		`SELECT id, path, last_scan, active FROM folders WHERE id = ?`,
		folderID,
	).Scan(&f.ID, &f.Path, &f.LastScan, &f.Active)
	if err != nil {
		return nil, err
	}
	return &f, nil
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