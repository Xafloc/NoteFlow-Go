package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
	_ "github.com/mattn/go-sqlite3"
)

// Populates a temp task DB with a few tasks across two folders and exercises
// the filtering surface of `noteflow tasks`.

func setupTaskDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseServiceAt: %v", err)
	}
	defer svc.Close()

	// Two folders.
	projA, err := svc.RegisterFolder("/tmp/project-a")
	if err != nil {
		t.Fatalf("RegisterFolder A: %v", err)
	}
	projB, err := svc.RegisterFolder("/tmp/project-b")
	if err != nil {
		t.Fatalf("RegisterFolder B: %v", err)
	}

	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	if err := svc.SyncFolderTasks(projA.ID, []models.Task{
		{Text: "- [ ] !p1 @" + tomorrow + " #release ship the changelog", Checked: false},
		{Text: "- [ ] !p3 #docs update README", Checked: false},
		{Text: "- [x] done thing", Checked: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncFolderTasks(projB.ID, []models.Task{
		{Text: "- [ ] !p2 @" + yesterday + " overdue bug fix", Checked: false},
		{Text: "- [ ] @" + today + " #release demo prep", Checked: false},
	}); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

func TestRunTasks_AllOpen(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, nil, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "ship the changelog") {
		t.Errorf("expected p1 task in output:\n%s", s)
	}
	if strings.Contains(s, "done thing") {
		t.Errorf("completed task leaked into default output:\n%s", s)
	}
	// 4 open tasks total.
	if got := strings.Count(s, "\n"); got != 4 {
		t.Errorf("expected 4 lines, got %d:\n%s", got, s)
	}
}

func TestRunTasks_IncludeDone(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--done"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	if !strings.Contains(out.String(), "done thing") {
		t.Errorf("--done did not include completed task:\n%s", out.String())
	}
}

func TestRunTasks_FilterPriority(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--priority", "1"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	if !strings.Contains(out.String(), "ship the changelog") {
		t.Errorf("priority=1 missed p1 task:\n%s", out.String())
	}
	if strings.Contains(out.String(), "update README") {
		t.Errorf("priority=1 leaked p3 task:\n%s", out.String())
	}
}

func TestRunTasks_FilterDueOverdue(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--due", "overdue"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	if !strings.Contains(out.String(), "overdue bug fix") {
		t.Errorf("--due overdue missed the overdue task:\n%s", out.String())
	}
	if strings.Contains(out.String(), "ship the changelog") {
		t.Errorf("--due overdue leaked a future-dated task:\n%s", out.String())
	}
}

func TestRunTasks_FilterTag(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--tag", "release"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "ship the changelog") || !strings.Contains(s, "demo prep") {
		t.Errorf("--tag release missed expected tasks:\n%s", s)
	}
	if strings.Contains(s, "update README") {
		t.Errorf("--tag release leaked #docs task:\n%s", s)
	}
}

func TestRunTasks_FilterProject(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--project", "project-b"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	if !strings.Contains(out.String(), "demo prep") {
		t.Errorf("--project missed expected task:\n%s", out.String())
	}
	if strings.Contains(out.String(), "ship the changelog") {
		t.Errorf("--project leaked task from project-a:\n%s", out.String())
	}
}

func TestRunTasks_JSON(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--json"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks: %v", err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON parse: %v\noutput:\n%s", err, out.String())
	}
	if len(parsed) != 4 {
		t.Errorf("expected 4 open tasks in JSON, got %d", len(parsed))
	}
}

func TestRunTasks_MissingDB(t *testing.T) {
	// Pointing at a path with no DB file should not error — "no tasks" is
	// not an error condition.
	dbPath := filepath.Join(t.TempDir(), "never-created.db")
	out := &bytes.Buffer{}
	err := RunTasks(dbPath, nil, out, &bytes.Buffer{})
	if err != nil {
		t.Errorf("expected no error for missing DB, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected empty output for missing DB, got: %s", out.String())
	}
}

func TestRunTasks_InvalidDueFlag(t *testing.T) {
	dbPath := setupTaskDB(t)
	err := RunTasks(dbPath, []string{"--due", "tomorrowish"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Errorf("expected error for invalid --due value")
	}
}

// setupToggleWorld creates a temp folder containing a real notes.md with
// known tasks, registers it in a fresh task DB, syncs once, and returns
// (dbPath, folderPath, hash-of-first-task). This is the world the
// --toggle tests operate in.
func setupToggleWorld(t *testing.T) (dbPath, folderPath, firstHash string) {
	t.Helper()
	folderPath = t.TempDir()
	notesContent := strings.Join([]string{
		"## 2026-05-12 09:00:00 - sprint",
		"",
		"- [ ] task alpha",
		"- [ ] task beta",
		"- [x] task gamma already done",
	}, "\n")
	if err := os.WriteFile(filepath.Join(folderPath, "notes.md"), []byte(notesContent), 0644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	dbPath = filepath.Join(t.TempDir(), "tasks.db")
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseServiceAt: %v", err)
	}
	defer svc.Close()
	folder, err := svc.RegisterFolder(folderPath)
	if err != nil {
		t.Fatalf("RegisterFolder: %v", err)
	}

	// Use a NoteManager to parse the file and pass tasks to the sync — this
	// is how the production path works, so the test exercises real codepaths.
	mgr, err := services.NewNoteManager(folderPath)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	tasks := mgr.GetAllTasks()
	if err := svc.SyncFolderTasks(folder.ID, tasks); err != nil {
		t.Fatalf("SyncFolderTasks: %v", err)
	}
	hashes := services.ComputeTaskHashes(tasks)
	return dbPath, folderPath, hashes[0]
}

func TestRunTasks_ToggleFlipsBothFileAndDB(t *testing.T) {
	dbPath, folderPath, firstHash := setupToggleWorld(t)
	out := &bytes.Buffer{}

	// First toggle: open → done.
	if err := RunTasks(dbPath, []string{"--toggle", firstHash}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("first toggle: %v", err)
	}
	if !strings.Contains(out.String(), "[x]") {
		t.Errorf("first toggle output did not show new state: %q", out.String())
	}

	// File should now have [x] on the alpha line.
	data, _ := os.ReadFile(filepath.Join(folderPath, "notes.md"))
	if !strings.Contains(string(data), "- [x] task alpha") {
		t.Errorf("notes.md not updated; got:\n%s", string(data))
	}

	// DB row for that hash should also be completed=1.
	db, _ := sql.Open("sqlite3", dbPath)
	defer db.Close()
	var done bool
	if err := db.QueryRow(`SELECT completed FROM tasks WHERE task_hash = ?`, firstHash).Scan(&done); err != nil {
		t.Fatalf("query db: %v", err)
	}
	if !done {
		t.Errorf("DB row not marked complete")
	}

	// Second toggle: done → open. Round-trip back to original state.
	out.Reset()
	if err := RunTasks(dbPath, []string{"--toggle", firstHash}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("second toggle: %v", err)
	}
	if !strings.Contains(out.String(), "[ ]") || strings.Contains(out.String(), "[x]") {
		t.Errorf("second toggle did not flip back to open: %q", out.String())
	}
	data, _ = os.ReadFile(filepath.Join(folderPath, "notes.md"))
	if !strings.Contains(string(data), "- [ ] task alpha") {
		t.Errorf("notes.md not flipped back; got:\n%s", string(data))
	}
}

func TestRunTasks_ToggleUnknownHash(t *testing.T) {
	dbPath, _, _ := setupToggleWorld(t)
	err := RunTasks(dbPath, []string{"--toggle", "deadbeef0000"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Errorf("expected error for unknown hash")
	}
}

func TestRunTasks_StatusLine_Default(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--status"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks --status: %v", err)
	}
	s := strings.TrimSpace(out.String())
	// setupTaskDB creates: p1 due tomorrow, p3 no due, done (excluded),
	// p2 due yesterday (overdue), no-prio due today.
	// Expected: today=1 overdue=1 open=4.
	wantSubstrs := []string{"today=1", "overdue=1", "open=4"}
	for _, w := range wantSubstrs {
		if !strings.Contains(s, w) {
			t.Errorf("status line %q missing %q", s, w)
		}
	}
}

func TestRunTasks_StatusLine_JSON(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--status", "--json"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks --status --json: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse JSON: %v\nstdout=%s", err, out.String())
	}
	if got["today"] != 1 || got["overdue"] != 1 || got["open"] != 4 {
		t.Errorf("status JSON = %#v, want today=1 overdue=1 open=4", got)
	}
}

func TestRunTasks_StatusLine_ProjectFilter(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--status", "--project", "project-b"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunTasks --status --project: %v", err)
	}
	s := strings.TrimSpace(out.String())
	// project-b has: overdue (p2 + yesterday), today (no-prio + today).
	if !strings.Contains(s, "today=1") || !strings.Contains(s, "overdue=1") || !strings.Contains(s, "open=2") {
		t.Errorf("project-filtered status line wrong: %q", s)
	}
}

func TestRunTasks_StatusLine_MissingDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "never.db")
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--status"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("--status against missing DB: %v", err)
	}
	if !strings.Contains(out.String(), "today=0") {
		t.Errorf("missing DB should yield zeros, got: %q", out.String())
	}
}

func TestRunTasks_SaveAndApplyView(t *testing.T) {
	dbPath := setupTaskDB(t)
	out := &bytes.Buffer{}

	// Save a view that filters to overdue p2.
	if err := RunTasks(dbPath, []string{"--save-view", "blockers", "--due", "overdue", "--priority", "2"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("save-view: %v", err)
	}
	if !strings.Contains(out.String(), "saved view: blockers") {
		t.Errorf("save-view output unexpected: %q", out.String())
	}

	// Apply the view — should match exactly the one task it describes.
	out.Reset()
	if err := RunTasks(dbPath, []string{"--view", "blockers"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("apply view: %v", err)
	}
	if !strings.Contains(out.String(), "overdue bug fix") {
		t.Errorf("view did not surface expected task:\n%s", out.String())
	}
	// Should be exactly one matching task in setupTaskDB.
	if strings.Count(out.String(), "\n") != 1 {
		t.Errorf("expected 1 task line, got:\n%s", out.String())
	}
}

func TestRunTasks_CLIOverridesView(t *testing.T) {
	// --tag flag on command line must beat the saved view's --tag value.
	dbPath := setupTaskDB(t)
	if err := RunTasks(dbPath, []string{"--save-view", "release-things", "--tag", "release"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("save-view: %v", err)
	}
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--view", "release-things", "--tag", "docs"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("apply view + override: %v", err)
	}
	// --tag=docs should win; we should see the #docs task and NOT the #release ones.
	if !strings.Contains(out.String(), "update README") {
		t.Errorf("CLI override missed expected #docs task:\n%s", out.String())
	}
	if strings.Contains(out.String(), "ship the changelog") {
		t.Errorf("CLI override didn't suppress #release task:\n%s", out.String())
	}
}

func TestRunTasks_ListViews(t *testing.T) {
	dbPath := setupTaskDB(t)
	for _, name := range []string{"zeta-view", "alpha-view", "middle"} {
		if err := RunTasks(dbPath, []string{"--save-view", name}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--list-views"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("list-views: %v", err)
	}
	// Alphabetical.
	got := strings.TrimSpace(out.String())
	want := "alpha-view\nmiddle\nzeta-view"
	if got != want {
		t.Errorf("list-views output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRunTasks_DeleteView(t *testing.T) {
	dbPath := setupTaskDB(t)
	if err := RunTasks(dbPath, []string{"--save-view", "tmp"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := RunTasks(dbPath, []string{"--delete-view", "tmp"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Listing should not include "tmp".
	out := &bytes.Buffer{}
	if err := RunTasks(dbPath, []string{"--list-views"}, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(out.String(), "tmp") {
		t.Errorf("deleted view still listed: %q", out.String())
	}
}

func TestRunTasks_DeleteViewIdempotent(t *testing.T) {
	dbPath := setupTaskDB(t)
	// Deleting a view that doesn't exist must not error.
	if err := RunTasks(dbPath, []string{"--delete-view", "never-existed"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Errorf("delete-view of missing name errored: %v", err)
	}
}

func TestRunTasks_ApplyMissingView(t *testing.T) {
	dbPath := setupTaskDB(t)
	err := RunTasks(dbPath, []string{"--view", "does-not-exist"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Errorf("expected error when applying missing view")
	}
	if !strings.Contains(err.Error(), "no saved view") {
		t.Errorf("error should mention missing view; got: %v", err)
	}
}

func TestRunTasks_ToggleStaleFile(t *testing.T) {
	// Simulates: user edited the source notes.md after the last sync, so
	// the task in the DB no longer exists in the current file. Toggle must
	// refuse rather than silently writing a wrong line.
	dbPath, folderPath, firstHash := setupToggleWorld(t)

	// Replace notes.md with a file that has *different* tasks. The DB still
	// has the old hashes from setupToggleWorld's sync.
	newContent := "## 2026-05-12 10:00:00\n\n- [ ] completely new task\n"
	if err := os.WriteFile(filepath.Join(folderPath, "notes.md"), []byte(newContent), 0644); err != nil {
		t.Fatalf("rewrite notes.md: %v", err)
	}

	err := RunTasks(dbPath, []string{"--toggle", firstHash}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Errorf("expected error when DB hash is not in current file")
	}
	if !strings.Contains(err.Error(), "no longer in") {
		t.Errorf("error message should mention stale file; got: %v", err)
	}
}
