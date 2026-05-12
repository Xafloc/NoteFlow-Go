package services

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	_ "modernc.org/sqlite"
)

// These tests pin the §7-resolved invariants in docs/20260512_task_db_schema.md:
//   - task.id stays stable across syncs for unchanged tasks
//   - editing a task swaps its row (new id) without touching its siblings
//   - removing a task deletes only its row
//   - last_updated advances only when content/completed changes

func newTestDB(t *testing.T) (*DatabaseService, *models.FolderRegistry) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	svc, err := NewDatabaseServiceAt(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseServiceAt: %v", err)
	}
	t.Cleanup(func() { svc.Close() })
	folder, err := svc.RegisterFolder("/tmp/test-project")
	if err != nil {
		t.Fatalf("RegisterFolder: %v", err)
	}
	return svc, folder
}

// idMap returns a map from task hash to task DB id for the given folder.
func idMap(t *testing.T, svc *DatabaseService, folderID int) map[string]int {
	t.Helper()
	rows, err := svc.db.Query(`SELECT id, task_hash FROM tasks WHERE folder_id = ?`, folderID)
	if err != nil {
		t.Fatalf("query ids: %v", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id int
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[hash] = id
	}
	return out
}

func updatedAt(t *testing.T, svc *DatabaseService, folderID int, hash string) time.Time {
	t.Helper()
	var ts time.Time
	err := svc.db.QueryRow(`SELECT last_updated FROM tasks WHERE folder_id = ? AND task_hash = ?`, folderID, hash).Scan(&ts)
	if err != nil {
		t.Fatalf("query last_updated: %v", err)
	}
	return ts
}

func TestSyncFolderTasks_IDsStableAcrossSyncs(t *testing.T) {
	svc, folder := newTestDB(t)
	tasks := []models.Task{
		{Text: "- [ ] task one"},
		{Text: "- [ ] task two"},
		{Text: "- [ ] task three"},
	}

	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := idMap(t, svc, folder.ID)
	if len(before) != 3 {
		t.Fatalf("expected 3 rows after first sync, got %d", len(before))
	}

	// Sync the exact same tasks again. IDs must be unchanged.
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	after := idMap(t, svc, folder.ID)
	if len(after) != 3 {
		t.Fatalf("expected 3 rows after second sync, got %d", len(after))
	}
	for hash, beforeID := range before {
		if after[hash] != beforeID {
			t.Errorf("task %s id changed: %d -> %d", hash, beforeID, after[hash])
		}
	}
}

func TestSyncFolderTasks_EditingOneDoesNotDisturbOthers(t *testing.T) {
	svc, folder := newTestDB(t)
	tasks := []models.Task{
		{Text: "- [ ] task one"},
		{Text: "- [ ] task two"},
		{Text: "- [ ] task three"},
	}
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := idMap(t, svc, folder.ID)

	// Edit task two — text changes, so hash changes, so it becomes a new row.
	// The other two must keep their ids.
	editedTask2 := "- [ ] task two (edited)"
	tasks[1] = models.Task{Text: editedTask2}
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("edit sync: %v", err)
	}
	after := idMap(t, svc, folder.ID)
	if len(after) != 3 {
		t.Fatalf("expected 3 rows after edit, got %d", len(after))
	}

	t1Hash := TaskHashFromText("- [ ] task one")
	t3Hash := TaskHashFromText("- [ ] task three")
	if before[t1Hash] == 0 || after[t1Hash] != before[t1Hash] {
		t.Errorf("task one id changed: %d -> %d", before[t1Hash], after[t1Hash])
	}
	if before[t3Hash] == 0 || after[t3Hash] != before[t3Hash] {
		t.Errorf("task three id changed: %d -> %d", before[t3Hash], after[t3Hash])
	}
	editedHash := TaskHashFromText(editedTask2)
	if after[editedHash] == 0 {
		t.Errorf("edited task missing from after-set: %#v", after)
	}
}

func TestSyncFolderTasks_RemovingOnlyDeletesThatRow(t *testing.T) {
	svc, folder := newTestDB(t)
	tasks := []models.Task{
		{Text: "- [ ] keep me"},
		{Text: "- [ ] delete me"},
		{Text: "- [ ] also keep me"},
	}
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	before := idMap(t, svc, folder.ID)

	// Remove the middle task.
	if err := svc.SyncFolderTasks(folder.ID, []models.Task{tasks[0], tasks[2]}); err != nil {
		t.Fatalf("removal sync: %v", err)
	}
	after := idMap(t, svc, folder.ID)
	if len(after) != 2 {
		t.Fatalf("expected 2 rows after removal, got %d", len(after))
	}
	keepHash := TaskHashFromText("- [ ] keep me")
	alsoHash := TaskHashFromText("- [ ] also keep me")
	delHash := TaskHashFromText("- [ ] delete me")
	if after[keepHash] != before[keepHash] {
		t.Errorf("keep-me id changed: %d -> %d", before[keepHash], after[keepHash])
	}
	if after[alsoHash] != before[alsoHash] {
		t.Errorf("also-keep-me id changed: %d -> %d", before[alsoHash], after[alsoHash])
	}
	if _, present := after[delHash]; present {
		t.Errorf("delete-me row still present after removal sync")
	}
}

func TestSyncFolderTasks_LastUpdatedOnlyOnChange(t *testing.T) {
	svc, folder := newTestDB(t)
	tasks := []models.Task{{Text: "- [ ] stable task"}, {Text: "- [ ] will toggle"}}
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	stableHash := TaskHashFromText("- [ ] stable task")
	toggleHash := TaskHashFromText("- [ ] will toggle")

	stableBefore := updatedAt(t, svc, folder.ID, stableHash)
	toggleBefore := updatedAt(t, svc, folder.ID, toggleHash)

	// SQLite's CURRENT_TIMESTAMP and our time.Now() have second resolution,
	// so wait long enough to detect a change.
	time.Sleep(1100 * time.Millisecond)

	// Sync again, toggling only the second task.
	tasks[1].Checked = true
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	stableAfter := updatedAt(t, svc, folder.ID, stableHash)
	toggleAfter := updatedAt(t, svc, folder.ID, toggleHash)

	if !stableAfter.Equal(stableBefore) {
		t.Errorf("unchanged task got new last_updated: %v -> %v", stableBefore, stableAfter)
	}
	if !toggleAfter.After(toggleBefore) {
		t.Errorf("toggled task did not advance last_updated: %v -> %v", toggleBefore, toggleAfter)
	}
}

func TestComputeTaskHashes_DuplicatesGetSuffixed(t *testing.T) {
	tasks := []models.Task{
		{Text: "- [ ] same"},
		{Text: "- [ ] different"},
		{Text: "- [ ] same"},
		{Text: "- [ ] same"},
	}
	hashes := ComputeTaskHashes(tasks)
	if hashes[0] == hashes[2] || hashes[2] == hashes[3] || hashes[0] == hashes[3] {
		t.Errorf("expected duplicate-text tasks to get unique hashes, got %v", hashes)
	}
	if hashes[0] != TaskHashFromText("- [ ] same") {
		t.Errorf("first occurrence should equal base hash; got %s vs %s", hashes[0], TaskHashFromText("- [ ] same"))
	}
}

func TestAddColumnIfMissing_Idempotent(t *testing.T) {
	// Running migrate twice (simulated by calling addColumnIfMissing again)
	// must not error.
	svc, _ := newTestDB(t)
	if err := svc.addColumnIfMissing("tasks", "task_hash", "TEXT"); err != nil {
		t.Errorf("re-adding task_hash errored: %v", err)
	}
}

func TestMigrate_FromPreHashSchema(t *testing.T) {
	// Regression for a real bug caught against the live ~/.config DB:
	// the original migrate() created the task_hash index before ALTER TABLE
	// had added the column, so opening a pre-2026-05-12 DB failed with
	// "no such column: task_hash". This test simulates that DB by writing
	// the old schema directly, then calls NewDatabaseServiceAt to drive the
	// migration and asserts everything ends up correct.
	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	rawDB, err := openRaw(dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	// Write the legacy schema (no task_hash column, no idx_tasks_hash index).
	if _, err := rawDB.Exec(`
		CREATE TABLE folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE NOT NULL,
			last_scan DATETIME DEFAULT CURRENT_TIMESTAMP,
			active BOOLEAN DEFAULT 1
		);
		CREATE TABLE tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_id INTEGER NOT NULL,
			file_path TEXT NOT NULL,
			line_number INTEGER NOT NULL,
			content TEXT NOT NULL,
			completed BOOLEAN DEFAULT 0,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE CASCADE
		);
		INSERT INTO folders (path) VALUES ('/legacy/folder');
		INSERT INTO tasks (folder_id, file_path, line_number, content, completed)
		VALUES (1, 'notes.md', 0, '- [ ] legacy task', 0);
	`); err != nil {
		t.Fatalf("write legacy schema: %v", err)
	}
	rawDB.Close()

	// Now open via the production constructor; the migration must succeed.
	svc, err := NewDatabaseServiceAt(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseServiceAt against legacy schema: %v", err)
	}
	t.Cleanup(func() { svc.Close() })

	// task_hash column must now exist and be queryable.
	rows, err := svc.db.Query(`SELECT id, content, task_hash FROM tasks`)
	if err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	defer rows.Close()
	var seen int
	for rows.Next() {
		var id int
		var content string
		var hash sql.NullString
		if err := rows.Scan(&id, &content, &hash); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// Legacy row's task_hash should be NULL (will be cleaned up on next sync).
		if hash.Valid {
			t.Errorf("legacy row unexpectedly has hash %q", hash.String)
		}
		seen++
	}
	if seen != 1 {
		t.Errorf("expected 1 legacy task row, got %d", seen)
	}

	// And a fresh sync should clean up the NULL row and insert with a hash.
	folder, err := svc.RegisterFolder("/legacy/folder")
	if err != nil {
		t.Fatalf("RegisterFolder: %v", err)
	}
	if err := svc.SyncFolderTasks(folder.ID, []models.Task{{Text: "- [ ] new task"}}); err != nil {
		t.Fatalf("SyncFolderTasks after migration: %v", err)
	}
	var n int
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE folder_id = ? AND task_hash IS NOT NULL`, folder.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("after sync, expected 1 row with task_hash, got %d", n)
	}
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE folder_id = ? AND task_hash IS NULL`, folder.ID).Scan(&n); err != nil {
		t.Fatalf("count NULL: %v", err)
	}
	if n != 0 {
		t.Errorf("after sync, expected 0 NULL-hash rows, got %d", n)
	}
}

// openRaw bypasses NewDatabaseServiceAt's migration so a test can plant a
// legacy schema before the real migration runs.
func openRaw(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
