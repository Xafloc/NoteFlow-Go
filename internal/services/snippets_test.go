package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the +file: code-snippet sigil (Goal 1 "code snippet attachment
// by file path"). Three layers:
//   1. resolveSnippetPath — the security boundary.
//   2. extractSnippetLines — pure line-slicing logic.
//   3. processCodeSnippets — end-to-end through a NoteManager.

func TestResolveSnippetPath_StaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	// Create a real file so EvalSymlinks resolves cleanly.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok := resolveSnippetPath(root, "main.go")
	if !ok {
		t.Fatalf("expected ok=true for simple relative path")
	}
	if !strings.HasSuffix(got, "main.go") {
		t.Errorf("unexpected resolved path: %s", got)
	}
}

func TestResolveSnippetPath_NestedDirsOK(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "models"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "models", "note.go"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := resolveSnippetPath(root, "internal/models/note.go"); !ok {
		t.Errorf("nested relative path rejected unexpectedly")
	}
}

func TestResolveSnippetPath_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	if _, ok := resolveSnippetPath(root, "/etc/passwd"); ok {
		t.Errorf("absolute path was not rejected")
	}
}

func TestResolveSnippetPath_RejectsParentEscape(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../etc/passwd", "../../etc/passwd", "subdir/../../escape"} {
		if _, ok := resolveSnippetPath(root, p); ok {
			t.Errorf("path %q was not rejected", p)
		}
	}
}

func TestResolveSnippetPath_RejectsSymlinkOutside(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, ok := resolveSnippetPath(root, "link.txt"); ok {
		t.Errorf("symlink to outside was not rejected")
	}
}

func TestResolveSnippetPath_RejectsEmpty(t *testing.T) {
	if _, ok := resolveSnippetPath(t.TempDir(), ""); ok {
		t.Errorf("empty path was not rejected")
	}
}

func TestExtractSnippetLines(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	tests := []struct {
		name        string
		start, end  int
		wantContent string
		wantRange   string
		wantErr     bool
	}{
		{"whole file", 0, 0, "line1\nline2\nline3\nline4\nline5", "", false},
		{"single line", 2, 0, "line2", "2", false},
		{"range", 2, 4, "line2\nline3\nline4", "2-4", false},
		{"range to EOF clamps", 4, 999, "line4\nline5", "4-5", false},
		{"inverted range", 5, 2, "", "", true},
		{"start past EOF", 99, 99, "", "", true},
		{"start zero with end is invalid", 0, 3, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotRange, err := extractSnippetLines(content, tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error mismatch: got %v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotContent != tt.wantContent {
				t.Errorf("content = %q, want %q", gotContent, tt.wantContent)
			}
			if gotRange != tt.wantRange {
				t.Errorf("range = %q, want %q", gotRange, tt.wantRange)
			}
		})
	}
}

func TestProcessCodeSnippets_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewNoteManager(dir)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	// Create a fake source file inside the project root.
	src := "func main() {\n\tprintln(\"hello\")\n\tprintln(\"world\")\n}\n"
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Add a note with a snippet sigil that references lines 2-3.
	noteBody := "See the println calls: +file:cmd/main.go#2-3"
	if err := mgr.AddNote("", noteBody); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	// The stored note's content should now contain a fenced go block with
	// the right lines, and no sigil.
	notes := mgr.GetAllNotes()
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	got := notes[0].Content
	if strings.Contains(got, "+file:") {
		t.Errorf("sigil was not replaced; note content:\n%s", got)
	}
	wantSubs := []string{
		"```go",
		"// cmd/main.go#2-3",
		"println(\"hello\")",
		"println(\"world\")",
		"```",
	}
	for _, w := range wantSubs {
		if !strings.Contains(got, w) {
			t.Errorf("expected %q in note content; got:\n%s", w, got)
		}
	}
	// And the un-referenced first/last lines should NOT appear.
	if strings.Contains(got, "func main()") {
		t.Errorf("line 1 leaked into snippet despite range 2-3:\n%s", got)
	}
}

func TestProcessCodeSnippets_PathEscapeLeavesSigil(t *testing.T) {
	// A sigil with a path that escapes the project root is non-fatal: the
	// sigil is left in place and a warning is logged. The note still saves.
	dir := t.TempDir()
	mgr, err := NewNoteManager(dir)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	noteBody := "Try this: +file:../../etc/passwd"
	if err := mgr.AddNote("", noteBody); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	notes := mgr.GetAllNotes()
	if !strings.Contains(notes[0].Content, "+file:../../etc/passwd") {
		t.Errorf("escape-attempt sigil should remain in note; got:\n%s", notes[0].Content)
	}
	if strings.Contains(notes[0].Content, "```") {
		t.Errorf("escape-attempt should not produce a code block; got:\n%s", notes[0].Content)
	}
}

func TestProcessCodeSnippets_MissingFileLeavesSigil(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewNoteManager(dir)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	noteBody := "Reference: +file:nope.go"
	if err := mgr.AddNote("", noteBody); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	notes := mgr.GetAllNotes()
	if !strings.Contains(notes[0].Content, "+file:nope.go") {
		t.Errorf("missing-file sigil should remain; got:\n%s", notes[0].Content)
	}
}

func TestProcessCodeSnippets_LanguageDetection(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewNoteManager(dir)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	for _, ext := range []string{"py", "js", "yaml"} {
		name := "file." + ext
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x = 1\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	body := "+file:file.py and +file:file.js and +file:file.yaml"
	if err := mgr.AddNote("", body); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	got := mgr.GetAllNotes()[0].Content
	for _, lang := range []string{"```python", "```javascript", "```yaml"} {
		if !strings.Contains(got, lang) {
			t.Errorf("expected %q in expanded snippets:\n%s", lang, got)
		}
	}
}
