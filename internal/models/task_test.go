package models

import (
	"reflect"
	"testing"
	"time"
)

// Pins the inline-metadata contract documented in
// docs/20260512_notes_md_schema.md §4 ("Lightweight inline task metadata").

func TestParseTaskMetadata_Priorities(t *testing.T) {
	tests := []struct {
		in       string
		wantPrio int
	}{
		{"- [ ] !p1 ship release notes", 1},
		{"- [ ] !p2 review the PR", 2},
		{"- [ ] !p3 nice-to-have", 3},
		{"- [ ] !p0 highest urgency", 1}, // !p0 normalized to 1 — see ParseTaskMetadata comment
		{"- [ ] no priority here", 0},
		{"- [ ] surrounding word!p1 should not match (no preceding space)", 0},
		{"- [ ] !p9 out of range, ignored", 0},
		{"- [ ] !p1", 1}, // end-of-text boundary OK
	}
	for _, tt := range tests {
		got, _, _ := ParseTaskMetadata(tt.in)
		if got != tt.wantPrio {
			t.Errorf("ParseTaskMetadata(%q) priority = %d, want %d", tt.in, got, tt.wantPrio)
		}
	}
}

func TestParseTaskMetadata_DueDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"- [ ] @2026-05-20 ship release notes", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)},
		{"- [ ] no date here", time.Time{}},
		{"- [ ] @2026-13-01 invalid month", time.Time{}},
		{"- [ ] email me @alice next week", time.Time{}}, // mention-style, not a date
		{"- [ ] @2026-05-20", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		_, got, _ := ParseTaskMetadata(tt.in)
		if !got.Equal(tt.want) {
			t.Errorf("ParseTaskMetadata(%q) due = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseTaskMetadata_Tags(t *testing.T) {
	tests := []struct {
		in       string
		wantTags []string
	}{
		{"- [ ] #release ship the changelog", []string{"release"}},
		{"- [ ] #release #docs both at once", []string{"release", "docs"}},
		{"- [ ] no tags here", nil},
		{"- [ ] see issue #123 (pure numeric, not a tag)", nil},
		{"- [ ] inline#tag not preceded by space", nil},
		{"- [ ] #tag-with-dash and #snake_case", []string{"tag-with-dash", "snake_case"}},
	}
	for _, tt := range tests {
		_, _, got := ParseTaskMetadata(tt.in)
		if !reflect.DeepEqual(got, tt.wantTags) {
			t.Errorf("ParseTaskMetadata(%q) tags = %v, want %v", tt.in, got, tt.wantTags)
		}
	}
}

func TestParseTaskMetadata_Combined(t *testing.T) {
	line := "- [ ] !p1 @2026-05-20 #release #docs ship the changelog"
	prio, due, tags := ParseTaskMetadata(line)
	if prio != 1 {
		t.Errorf("priority = %d, want 1", prio)
	}
	if !due.Equal(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("due = %v, want 2026-05-20", due)
	}
	if !reflect.DeepEqual(tags, []string{"release", "docs"}) {
		t.Errorf("tags = %v, want [release docs]", tags)
	}
}

func TestCleanTaskText(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"- [ ] !p1 @2026-05-20 #release ship the changelog", "- [ ] ship the changelog"},
		{"- [ ] !p2 #urgent #docs review", "- [ ] review"},
		{"- [ ] plain task", "- [ ] plain task"},
		{"- [ ] tag at end #done", "- [ ] tag at end"},
	}
	for _, tt := range tests {
		got := CleanTaskText(tt.in)
		if got != tt.want {
			t.Errorf("CleanTaskText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNoteParseTasks_AttachesMetadata(t *testing.T) {
	// End-to-end: parsing a note's tasks should populate the new metadata
	// fields without changing the stored Text.
	input := "## 2026-05-12 09:30:45 - Sprint\n\n" +
		"- [ ] !p1 @2026-05-20 #release ship release notes\n" +
		"- [x] #done already finished\n" +
		"- [ ] plain task"
	note, err := NewNoteFromText(input)
	if err != nil {
		t.Fatalf("NewNoteFromText: %v", err)
	}
	if len(note.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(note.Tasks))
	}

	t0 := note.Tasks[0]
	if t0.Priority != 1 || !t0.DueDate.Equal(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) || !reflect.DeepEqual(t0.Tags, []string{"release"}) {
		t.Errorf("task 0 metadata wrong: prio=%d due=%v tags=%v", t0.Priority, t0.DueDate, t0.Tags)
	}
	// Text must retain the original tokens — the file stays the source of truth.
	if !contains(t0.Text, "!p1") || !contains(t0.Text, "@2026-05-20") || !contains(t0.Text, "#release") {
		t.Errorf("task 0 Text lost original tokens: %q", t0.Text)
	}

	t1 := note.Tasks[1]
	if t1.Priority != 0 || !t1.DueDate.IsZero() || !reflect.DeepEqual(t1.Tags, []string{"done"}) {
		t.Errorf("task 1 metadata wrong: prio=%d due=%v tags=%v", t1.Priority, t1.DueDate, t1.Tags)
	}

	t2 := note.Tasks[2]
	if t2.Priority != 0 || !t2.DueDate.IsZero() || t2.Tags != nil {
		t.Errorf("task 2 should have no metadata: prio=%d due=%v tags=%v", t2.Priority, t2.DueDate, t2.Tags)
	}
}

// small helper so the test file doesn't depend on the strings package
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
