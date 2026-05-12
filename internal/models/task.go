package models

import (
	"regexp"
	"strings"
	"time"
)

// Task represents a checkbox task within a note.
//
// Priority, DueDate, and Tags are derived from inline markdown tokens in
// Text — see docs/20260512_notes_md_schema.md §4 and §7. They are not
// persisted to the task DB yet; for now they exist only in-memory so the
// UI can render and filter on them. Once stable task IDs land (per
// docs/20260512_task_db_schema.md §7) these will move into real columns.
type Task struct {
	Index    int       `json:"index"`              // Unique global identifier
	Checked  bool      `json:"checked"`            // Completion state
	Text     string    `json:"text"`               // Full task text including checkbox + metadata tokens
	Priority int       `json:"priority,omitempty"` // 0 = none, 1..3 = !p1..!p3; lower = more urgent
	DueDate  time.Time `json:"due_date,omitempty"` // zero value = no due date
	Tags     []string  `json:"tags,omitempty"`     // values without the leading "#"
}

// TaskInfo represents task information for API responses
type TaskInfo struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	NoteTitle string `json:"note_title"`
	Timestamp string `json:"timestamp"`
}

// TaskUpdate represents a task update request
type TaskUpdate struct {
	Checked bool `json:"checked"`
}

// Inline metadata token shapes. These are intentionally narrow — they must
// not collide with ordinary prose ("!p1!" in a sentence, "@someone" as a
// mention, "#1" as an issue ref) so all three require a specific structure:
//
//	priority:  !p<digit>     where digit is 0..3
//	due date:  @YYYY-MM-DD   (exact 4-2-2 digit form)
//	tag:       #<word>       where word is letters/digits/_/- (not pure digits)
//
// Tokens must be preceded by whitespace or start-of-text. The trailing
// boundary uses \b (zero-width) rather than consuming whitespace so that
// adjacent tokens like "#a #b" both match — FindAll doesn't overlap, so a
// consumed trailing space would eat the next token's leading anchor.
var (
	priorityTokenRE = regexp.MustCompile(`(?:^|\s)!p([0-3])\b`)
	dueDateTokenRE  = regexp.MustCompile(`(?:^|\s)@(\d{4}-\d{2}-\d{2})\b`)
	tagTokenRE      = regexp.MustCompile(`(?:^|\s)#([A-Za-z_][A-Za-z0-9_-]*)`)
)

// ParseTaskMetadata extracts inline priority/due/tag tokens from a task
// line. It does not modify the input — the tokens stay in the text so the
// note remains diff-friendly and other tools (grep, AI agents) can see
// them. Unknown tokens are ignored.
//
// Returns zero values for any field whose token is absent or unparseable.
func ParseTaskMetadata(line string) (priority int, due time.Time, tags []string) {
	if m := priorityTokenRE.FindStringSubmatch(line); m != nil {
		// Priority 0 means "explicitly !p0" — treat as 0 (highest); the
		// zero-value of int already means "none" elsewhere, so we use 1..3
		// for set priorities and 0 for unset. To keep that invariant, map
		// !p0 to 1 (top priority).
		switch m[1] {
		case "0", "1":
			priority = 1
		case "2":
			priority = 2
		case "3":
			priority = 3
		}
	}
	if m := dueDateTokenRE.FindStringSubmatch(line); m != nil {
		if t, err := time.Parse("2006-01-02", m[1]); err == nil {
			due = t
		}
	}
	for _, m := range tagTokenRE.FindAllStringSubmatch(line, -1) {
		tags = append(tags, m[1])
	}
	return priority, due, tags
}

// CleanTaskText returns the task text with metadata tokens stripped, for
// display surfaces that want just the human-readable description. The
// stored Text field on Task always retains the original tokens.
func CleanTaskText(line string) string {
	out := priorityTokenRE.ReplaceAllString(line, " ")
	out = dueDateTokenRE.ReplaceAllString(out, " ")
	out = tagTokenRE.ReplaceAllString(out, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(out), " "))
}