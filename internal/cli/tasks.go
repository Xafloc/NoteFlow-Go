package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/services"
	_ "modernc.org/sqlite"
)

const tasksHelp = `USAGE:
    noteflow-go tasks [FLAGS]                List tasks, filtered as requested
    noteflow-go tasks --status [--project P] One-line summary (for status bars)
    noteflow-go tasks --toggle HASH          Mark a task done/undone
    noteflow-go tasks --save-view NAME …     Save current filters as a view
    noteflow-go tasks --view NAME            Apply a saved view's filters
    noteflow-go tasks --list-views           List all saved view names
    noteflow-go tasks --delete-view NAME     Delete a saved view

Reads from ~/.config/noteflow/tasks.db — populated automatically whenever
you run 'noteflow-go' in a folder. No DB means no tasks; not an error.

FILTERING (combine freely):
    --done             Include completed tasks (default: open only)
    --due VALUE        today | week | overdue | YYYY-MM-DD
    --priority N       1..3 — match tasks tagged !p1..!p3 in markdown
    --tag NAME         Match tasks tagged #NAME (no leading #)
    --project SUBSTR   Match folders whose path contains SUBSTR
                       (case-insensitive)

OUTPUT:
    --json             Emit JSON instead of the human-readable table
    --status           Print "today=N overdue=N open=N" and exit
                       (combines with --project and --json)

ACTIONS:
    --toggle HASH      Flip completion state of the task with stable hash
                       HASH. Updates both notes.md and the task DB.
                       Get hashes via --json (each task has an .id and the
                       hash is derivable from .content).

SAVED VIEWS:
    --save-view NAME   Persist the current filter set as NAME. Composes
                       with --view, so '--view A --save-view B' saves a
                       tweaked copy of A as B.
    --view NAME        Load filters from NAME. Explicit flags still win
                       (CLI overrides view).
    --list-views       Alphabetical list of saved view names
    --delete-view NAME Idempotent — deleting a missing view is not an error

GLOBAL:
    --help, -h         Show this help and exit

EXAMPLES:
    # See everything open right now
    noteflow-go tasks

    # Today's planning surface
    noteflow-go tasks --due today
    noteflow-go tasks --due overdue --priority 1   # urgent past-due only

    # Per-repo status line for your shell prompt
    noteflow-go tasks --status --project current-repo

    # Save a query, recall it
    noteflow-go tasks --save-view blockers --due overdue --priority 1
    noteflow-go tasks --view blockers

    # Mark a task done from the terminal (the file gets updated too)
    noteflow-go tasks --toggle a3eb73cb5f2e
`


// RunTasks lists tasks from the cross-project task DB, applying optional
// filters. Read-only: never writes to the DB. Filtering on priority/due/tag
// is done in-process against the parsed metadata tokens in each task's text
// (see models.ParseTaskMetadata) because the DB does not yet have dedicated
// columns for these — see docs/20260512_task_db_schema.md §7.
//
// Usage:
//
//	noteflow tasks [--done] [--due today|week|overdue|YYYY-MM-DD]
//	              [--priority N] [--tag T] [--project SUBSTR] [--json]
//
// Output is one line per task in the form:
//
//	[STATE] PRIORITY DUE  TEXT                  (PROJECT)
//
// where STATE is "[ ]" or "[x]", PRIORITY is "p1"/"p2"/"p3"/"-",
// DUE is YYYY-MM-DD or "-", TEXT is the task text with metadata stripped,
// and PROJECT is the basename of the project folder.
func RunTasks(dbPath string, args []string, stdout, stderr io.Writer) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprint(stdout, tasksHelp)
			return nil
		}
	}

	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	includeDone := fs.Bool("done", false, "include completed tasks (default: open only)")
	dueFilter := fs.String("due", "", "filter by due date: today, week, overdue, or YYYY-MM-DD")
	priorityFilter := fs.Int("priority", 0, "filter by priority 1..3 (0 = no filter)")
	tagFilter := fs.String("tag", "", "filter by tag (without leading #)")
	projectFilter := fs.String("project", "", "filter by project path substring (case-insensitive)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human format")
	toggleHash := fs.String("toggle", "", "toggle the completion state of the task with the given hash; updates both notes.md and the DB")
	statusLine := fs.Bool("status", false, "print a single-line summary suitable for shell prompts / status bars and exit")
	viewName := fs.String("view", "", "apply a saved view's filters (command-line flags override the view's stored values)")
	saveView := fs.String("save-view", "", "save the current filter combination under NAME and exit")
	listViews := fs.Bool("list-views", false, "list all saved view names and exit")
	deleteView := fs.String("delete-view", "", "delete the named saved view and exit")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *listViews {
		return runListViews(dbPath, stdout)
	}
	if *deleteView != "" {
		return runDeleteView(dbPath, *deleteView, stdout)
	}

	// If --view is provided, load and merge its filters in *before* the
	// explicit flags get to override anything still at their zero values.
	// Command-line flags always win when both are set.
	if *viewName != "" {
		if err := mergeSavedView(dbPath, *viewName, includeDone, dueFilter, priorityFilter, tagFilter, projectFilter); err != nil {
			return err
		}
	}

	// --save-view captures the *effective* filter set (after a possible
	// --view merge) and stores it. This means `--view A --save-view B`
	// creates B as a tweaked copy of A.
	if *saveView != "" {
		return runSaveView(dbPath, *saveView, *includeDone, *dueFilter, *priorityFilter, *tagFilter, *projectFilter, stdout)
	}

	if *toggleHash != "" {
		return toggleTask(dbPath, *toggleHash, stdout)
	}

	if *statusLine {
		return printStatusLine(dbPath, *jsonOut, *projectFilter, stdout)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT t.id, t.content, t.completed, f.path
		FROM tasks t
		JOIN folders f ON t.folder_id = f.id
		WHERE f.active = 1
		ORDER BY f.path, t.completed, t.id`)
	if err != nil {
		// Likely cause: DB doesn't exist yet because NoteFlow hasn't been run.
		// That's "no tasks" rather than an error from the user's perspective.
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	type viewTask struct {
		ID       int        `json:"id"`
		Content  string     `json:"content"`            // raw, includes tokens
		Clean    string     `json:"clean"`              // tokens stripped, for display
		Done     bool       `json:"done"`
		Priority int        `json:"priority,omitempty"`
		Due      *time.Time `json:"due,omitempty"`
		Tags     []string   `json:"tags,omitempty"`
		Project  string     `json:"project"`            // folder path
	}

	var out []viewTask
	for rows.Next() {
		var t viewTask
		if err := rows.Scan(&t.ID, &t.Content, &t.Done, &t.Project); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		prio, due, tags := models.ParseTaskMetadata(t.Content)
		t.Priority = prio
		if !due.IsZero() {
			d := due
			t.Due = &d
		}
		t.Tags = tags
		t.Clean = models.CleanTaskText(t.Content)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	filtered := out[:0]
	for _, t := range out {
		if !*includeDone && t.Done {
			continue
		}
		if *priorityFilter != 0 && t.Priority != *priorityFilter {
			continue
		}
		if *tagFilter != "" && !containsTag(t.Tags, *tagFilter) {
			continue
		}
		if *projectFilter != "" && !strings.Contains(strings.ToLower(t.Project), strings.ToLower(*projectFilter)) {
			continue
		}
		if *dueFilter != "" {
			keep, err := dueMatches(*dueFilter, t.Due, t.Done, today)
			if err != nil {
				return err
			}
			if !keep {
				continue
			}
		}
		filtered = append(filtered, t)
	}

	// Sort: priority asc (0 last), due asc (none last), project, content.
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if (a.Priority == 0) != (b.Priority == 0) {
			return a.Priority != 0
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if (a.Due == nil) != (b.Due == nil) {
			return a.Due != nil
		}
		if a.Due != nil && b.Due != nil && !a.Due.Equal(*b.Due) {
			return a.Due.Before(*b.Due)
		}
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		return a.Content < b.Content
	})

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}

	for _, t := range filtered {
		state := "[ ]"
		if t.Done {
			state = "[x]"
		}
		prio := "-"
		if t.Priority > 0 {
			prio = fmt.Sprintf("p%d", t.Priority)
		}
		due := "-"
		if t.Due != nil {
			due = t.Due.Format("2006-01-02")
		}
		fmt.Fprintf(stdout, "%s %s %s  %s  (%s)\n", state, prio, due, stripCheckbox(t.Clean), filepath.Base(t.Project))
	}
	return nil
}

// savedViewFilters is the on-disk shape of a saved view: a JSON blob of the
// filter values. New fields can be added safely — JSON unmarshal ignores
// keys it doesn't recognize, and zero values are treated as "unset" by the
// merge logic below. See docs/20260512_task_db_schema.md §2 (task_views).
type savedViewFilters struct {
	Done     bool   `json:"done,omitempty"`
	Due      string `json:"due,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Project  string `json:"project,omitempty"`
}

// mergeSavedView loads a view by name and overlays its stored filter values
// onto the current flag pointers — but only for fields the caller didn't
// already set on the command line. Command-line wins, view fills the gaps.
func mergeSavedView(dbPath, name string, includeDone *bool, due *string, priority *int, tag, project *string) error {
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer svc.Close()
	raw, err := svc.GetView(name)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("no saved view named %q (try --list-views)", name)
	}
	if err != nil {
		return fmt.Errorf("load view: %w", err)
	}
	var v savedViewFilters
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("view %q has corrupt filters JSON: %w", name, err)
	}
	// Only fill flags still at their zero values — CLI overrides.
	if !*includeDone && v.Done {
		*includeDone = true
	}
	if *due == "" {
		*due = v.Due
	}
	if *priority == 0 {
		*priority = v.Priority
	}
	if *tag == "" {
		*tag = v.Tag
	}
	if *project == "" {
		*project = v.Project
	}
	return nil
}

func runSaveView(dbPath, name string, done bool, due string, priority int, tag, project string, stdout io.Writer) error {
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer svc.Close()
	blob, err := json.Marshal(savedViewFilters{
		Done: done, Due: due, Priority: priority, Tag: tag, Project: project,
	})
	if err != nil {
		return fmt.Errorf("encode view: %w", err)
	}
	if err := svc.SaveView(name, string(blob)); err != nil {
		return fmt.Errorf("save view: %w", err)
	}
	fmt.Fprintf(stdout, "saved view: %s\n", name)
	return nil
}

func runListViews(dbPath string, stdout io.Writer) error {
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer svc.Close()
	names, err := svc.ListViews()
	if err != nil {
		return fmt.Errorf("list views: %w", err)
	}
	for _, n := range names {
		fmt.Fprintln(stdout, n)
	}
	return nil
}

func runDeleteView(dbPath, name string, stdout io.Writer) error {
	svc, err := services.NewDatabaseServiceAt(dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer svc.Close()
	if err := svc.DeleteView(name); err != nil {
		return fmt.Errorf("delete view: %w", err)
	}
	fmt.Fprintf(stdout, "deleted view: %s\n", name)
	return nil
}

// printStatusLine prints a compact one-line summary of task counts, intended
// for embedding in shell prompts, tmux status bars, or editor status lines.
//
// Default format (Goal 2's "status-line surface"):
//
//	today=N overdue=N open=N
//
// With --json: {"today":N,"overdue":N,"open":N}
//
// With --project SUBSTR: counts are filtered to folders matching the
// substring, so a per-repo status line is possible without scripting around
// the listing command.
//
// Designed to be cheap — runs a single query and counts in-process.
func printStatusLine(dbPath string, asJSON bool, projectSubstr string, stdout io.Writer) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT t.content, t.completed, f.path
		FROM tasks t JOIN folders f ON t.folder_id = f.id
		WHERE f.active = 1`)
	if err != nil {
		// Missing DB == zero tasks. Don't error.
		if strings.Contains(err.Error(), "no such table") {
			return writeStatusCounts(stdout, asJSON, 0, 0, 0)
		}
		return fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	want := strings.ToLower(projectSubstr)

	var todayCount, overdueCount, openCount int
	for rows.Next() {
		var content, project string
		var done bool
		if err := rows.Scan(&content, &done, &project); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		if done {
			continue
		}
		if want != "" && !strings.Contains(strings.ToLower(project), want) {
			continue
		}
		openCount++
		_, due, _ := models.ParseTaskMetadata(content)
		if due.IsZero() {
			continue
		}
		d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, today.Location())
		if d.Equal(today) {
			todayCount++
		} else if d.Before(today) {
			overdueCount++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}
	return writeStatusCounts(stdout, asJSON, todayCount, overdueCount, openCount)
}

func writeStatusCounts(w io.Writer, asJSON bool, today, overdue, open int) error {
	if asJSON {
		enc := json.NewEncoder(w)
		return enc.Encode(map[string]int{"today": today, "overdue": overdue, "open": open})
	}
	_, err := fmt.Fprintf(w, "today=%d overdue=%d open=%d\n", today, overdue, open)
	return err
}

// toggleTask flips the completion state of a single task identified by its
// stable content hash. It is the bidirectional half of Goal 2: the source
// notes.md file and the task DB row are both updated, in that order, so that
// the file remains the source of truth even if the DB write fails.
//
// Concurrency caveat: if the NoteFlow web server is running against the same
// folder, it holds an in-memory NoteManager that won't see the file change
// until its next sync. The next sync reconciles correctly (file wins), but
// there is a brief window where the server's view is stale. For now this is
// documented; a file lock would tighten it — see TODO.md technical debt.
func toggleTask(dbPath, hash string, stdout io.Writer) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open task db: %w", err)
	}
	defer db.Close()

	var content, folderPath string
	var currentlyDone bool
	err = db.QueryRow(`
		SELECT t.content, t.completed, f.path
		FROM tasks t JOIN folders f ON t.folder_id = f.id
		WHERE t.task_hash = ?`, hash).Scan(&content, &currentlyDone, &folderPath)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("no task found with hash %q (run `noteflow tasks` to see available hashes)", hash)
	}
	if err != nil {
		return fmt.Errorf("look up task: %w", err)
	}

	// Open the folder's NoteManager — this is the only authoritative way to
	// edit notes.md, because it knows the parsing rules.
	mgr, err := services.NewNoteManager(folderPath)
	if err != nil {
		return fmt.Errorf("open notes.md at %s: %w", folderPath, err)
	}

	// Find the task whose hash matches. We walk every task in the file and
	// compute hashes the same way SyncFolderTasks does, then match.
	allTasks := mgr.GetAllTasks()
	hashes := services.ComputeTaskHashes(allTasks)
	matchIdx := -1
	for i, h := range hashes {
		if h == hash {
			matchIdx = i
			break
		}
	}
	if matchIdx == -1 {
		return fmt.Errorf("task hash %q is in the DB but no longer in %s — the file has changed since the last sync. Open the project in NoteFlow to re-sync, then try again",
			hash, filepath.Join(folderPath, "notes.md"))
	}

	newDone := !currentlyDone
	if err := mgr.UpdateTask(allTasks[matchIdx].Index, newDone); err != nil {
		return fmt.Errorf("update notes.md: %w", err)
	}

	// File is updated. Now reflect the change in the DB so subsequent queries
	// (and the next sync) see consistent state without waiting. We also update
	// the `content` column so the DB's view of the row's checkbox marker
	// matches the file — otherwise the next full sync would see a content
	// "change" and pointlessly bump last_updated.
	newContent := content
	if newDone {
		newContent = strings.Replace(content, "[ ]", "[x]", 1)
	} else {
		newContent = strings.Replace(content, "[x]", "[ ]", 1)
		newContent = strings.Replace(newContent, "[X]", "[ ]", 1)
	}
	if _, err := db.Exec(`UPDATE tasks SET content = ?, completed = ?, last_updated = ? WHERE task_hash = ?`,
		newContent, newDone, time.Now(), hash); err != nil {
		return fmt.Errorf("update task db: %w", err)
	}

	state := "[ ]"
	if newDone {
		state = "[x]"
	}
	fmt.Fprintf(stdout, "toggled: %s %s  (%s)\n", state, stripCheckbox(models.CleanTaskText(content)), filepath.Base(folderPath))
	return nil
}

// stripCheckbox removes a leading "- [ ] " / "- [x] " prefix from a cleaned
// task line, since the display already shows the checked state in its own
// column. Returns the input unchanged when no checkbox prefix is found.
func stripCheckbox(line string) string {
	t := strings.TrimLeft(line, " ")
	t = strings.TrimPrefix(t, "- ")
	if len(t) >= 3 && t[0] == '[' && (t[1] == ' ' || t[1] == 'x' || t[1] == 'X') && t[2] == ']' {
		t = strings.TrimLeft(t[3:], " ")
	}
	return t
}

func containsTag(have []string, want string) bool {
	want = strings.TrimPrefix(want, "#")
	for _, t := range have {
		if t == want {
			return true
		}
	}
	return false
}

// dueMatches returns whether a task should be included given its due date,
// completion state, and the --due filter token. Filter tokens:
//
//	today     — due is today
//	week      — due within the next 7 days (today inclusive)
//	overdue   — due strictly before today AND not done
//	YYYY-MM-DD — due is exactly that date
//
// Tasks with no due date are excluded by any --due filter.
func dueMatches(filter string, due *time.Time, done bool, today time.Time) (bool, error) {
	if due == nil {
		return false, nil
	}
	d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, today.Location())
	switch filter {
	case "today":
		return d.Equal(today), nil
	case "week":
		end := today.AddDate(0, 0, 7)
		return !d.Before(today) && d.Before(end), nil
	case "overdue":
		return d.Before(today) && !done, nil
	default:
		exact, err := time.Parse("2006-01-02", filter)
		if err != nil {
			return false, fmt.Errorf("invalid --due value %q (want today|week|overdue|YYYY-MM-DD)", filter)
		}
		exactDay := time.Date(exact.Year(), exact.Month(), exact.Day(), 0, 0, 0, 0, today.Location())
		return d.Equal(exactDay), nil
	}
}
