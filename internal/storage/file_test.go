package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
)

// Pins the file-layer half of docs/20260512_notes_md_schema.md.
// Section references (§2, §3, §6) point into that schema doc.

func newTempStorage(t *testing.T) *FileStorage {
	t.Helper()
	dir := t.TempDir()
	return NewFileStorage(dir)
}

func writeNotesFile(t *testing.T, fs *FileStorage, content string) {
	t.Helper()
	if err := os.WriteFile(fs.GetNotesFilePath(), []byte(content), 0644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
}

func TestLoadNotes_MissingFileCreatesEmpty(t *testing.T) {
	fs := newTempStorage(t)
	notes, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
	// §2: missing file is created on first read.
	if _, err := os.Stat(fs.GetNotesFilePath()); err != nil {
		t.Errorf("notes.md was not created: %v", err)
	}
}

func TestLoadNotes_EmptyFile(t *testing.T) {
	fs := newTempStorage(t)
	writeNotesFile(t, fs, "")
	notes, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes from empty file, got %d", len(notes))
	}
}

func TestLoadNotes_WhitespaceOnlyFile(t *testing.T) {
	// §2: a file with only whitespace parses to zero notes.
	fs := newTempStorage(t)
	writeNotesFile(t, fs, "   \n\n  \n")
	notes, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes from whitespace-only file, got %d", len(notes))
	}
}

func TestLoadNotes_SplitsOnSeparator(t *testing.T) {
	// §2: notes separated by "<!-- note -->" with optional surrounding newlines.
	fs := newTempStorage(t)
	content := strings.Join([]string{
		"## 2026-05-12 14:22:10 - Newer",
		"",
		"body two",
		"",
		"<!-- note -->",
		"",
		"## 2026-05-12 09:30:45 - Older",
		"",
		"body one",
	}, "\n")
	writeNotesFile(t, fs, content)

	notes, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Title != "Newer" || notes[1].Title != "Older" {
		t.Errorf("ordering not preserved: got titles [%q, %q]", notes[0].Title, notes[1].Title)
	}
}

func TestLoadNotes_DropsChunksWithoutHeader(t *testing.T) {
	// §3: any chunk between separators that does not start with "## " is dropped.
	fs := newTempStorage(t)
	content := strings.Join([]string{
		"## 2026-05-12 09:30:45 - Valid",
		"",
		"body",
		"<!-- note -->",
		"garbage not a header",
		"<!-- note -->",
		"## 2026-05-12 08:00:00 - Also valid",
		"",
		"body two",
	}, "\n")
	writeNotesFile(t, fs, content)

	notes, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 valid notes (1 garbage dropped), got %d", len(notes))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// §6 invariant 5: round-trip through disk preserves note identity.
	fs := newTempStorage(t)

	ts1 := time.Date(2026, 5, 12, 14, 22, 10, 0, time.UTC)
	ts2 := time.Date(2026, 5, 12, 9, 30, 45, 0, time.UTC)
	original := []*models.Note{
		{Title: "Newer", Content: "body two\nwith two lines", Timestamp: ts1},
		{Title: "Older", Content: "body one", Timestamp: ts2},
	}

	if err := fs.SaveNotes(original); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}
	loaded, err := fs.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(loaded) != len(original) {
		t.Fatalf("count mismatch: got %d, want %d", len(loaded), len(original))
	}
	for i := range original {
		if loaded[i].Title != original[i].Title {
			t.Errorf("note %d Title: got %q, want %q", i, loaded[i].Title, original[i].Title)
		}
		if !loaded[i].Timestamp.Equal(original[i].Timestamp) {
			t.Errorf("note %d Timestamp: got %v, want %v", i, loaded[i].Timestamp, original[i].Timestamp)
		}
		if loaded[i].Content != original[i].Content {
			t.Errorf("note %d Content: got %q, want %q", i, loaded[i].Content, original[i].Content)
		}
	}
}

func TestSaveNotes_UsesSeparator(t *testing.T) {
	// §2: exact on-disk separator is \n<!-- note -->\n (Join with separator).
	fs := newTempStorage(t)
	ts := time.Date(2026, 5, 12, 9, 30, 45, 0, time.UTC)
	notes := []*models.Note{
		{Title: "A", Content: "a", Timestamp: ts},
		{Title: "B", Content: "b", Timestamp: ts},
	}
	if err := fs.SaveNotes(notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}
	data, err := os.ReadFile(fs.GetNotesFilePath())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, models.NoteSeparator) {
		t.Errorf("output does not contain NoteSeparator %q\nactual:\n%s", models.NoteSeparator, got)
	}
	// Exactly one separator for two notes.
	if n := strings.Count(got, "<!-- note -->"); n != 1 {
		t.Errorf("separator count: got %d, want 1", n)
	}
}

func TestSaveNotes_DeterministicBytes(t *testing.T) {
	// §6 invariant 4: same inputs produce byte-identical files.
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	ts := time.Date(2026, 5, 12, 9, 30, 45, 0, time.UTC)
	notes := []*models.Note{
		{Title: "A", Content: "a\nmulti\nline", Timestamp: ts},
		{Title: "", Content: "no title", Timestamp: ts.Add(time.Hour)},
	}
	if err := fs.SaveNotes(notes); err != nil {
		t.Fatalf("first SaveNotes: %v", err)
	}
	first, err := os.ReadFile(fs.GetNotesFilePath())
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	// Rewrite to a sibling path and compare bytes.
	fs2 := NewFileStorage(filepath.Join(t.TempDir()))
	if err := fs2.SaveNotes(notes); err != nil {
		t.Fatalf("second SaveNotes: %v", err)
	}
	second, err := os.ReadFile(fs2.GetNotesFilePath())
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("non-deterministic output:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestEnsureDirectories(t *testing.T) {
	fs := newTempStorage(t)
	if err := fs.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories: %v", err)
	}
	for _, sub := range []string{"assets", "assets/images", "assets/files", "assets/sites"} {
		if _, err := os.Stat(filepath.Join(fs.BasePath, sub)); err != nil {
			t.Errorf("expected %s to exist: %v", sub, err)
		}
	}
}
