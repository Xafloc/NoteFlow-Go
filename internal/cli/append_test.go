package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readNotes(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "notes.md"))
	if err != nil {
		t.Fatalf("read notes.md: %v", err)
	}
	return string(data)
}

func TestAppend_FromArgs(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	err := RunAppend(dir, []string{"--title", "discovery", "found", "the", "bug"}, nil, out)
	if err != nil {
		t.Fatalf("RunAppend: %v", err)
	}
	got := readNotes(t, dir)
	if !strings.Contains(got, "- discovery\n\nfound the bug") {
		t.Errorf("notes.md missing expected content; got:\n%s", got)
	}
	if !strings.HasPrefix(out.String(), "appended: ") || !strings.Contains(out.String(), "- discovery") {
		t.Errorf("stdout = %q, want prefix 'appended: ' with title", out.String())
	}
}

func TestAppend_FromStdin(t *testing.T) {
	dir := t.TempDir()
	stdin := strings.NewReader("piped body\nsecond line")
	out := &bytes.Buffer{}
	err := RunAppend(dir, nil, stdin, out)
	if err != nil {
		t.Fatalf("RunAppend: %v", err)
	}
	got := readNotes(t, dir)
	if !strings.Contains(got, "piped body\nsecond line") {
		t.Errorf("notes.md missing stdin content; got:\n%s", got)
	}
	// No title means no " - " suffix in the output.
	if strings.Contains(out.String(), " - ") {
		t.Errorf("stdout had title separator when none was provided: %q", out.String())
	}
}

func TestAppend_ArgsBeatStdin(t *testing.T) {
	dir := t.TempDir()
	stdin := strings.NewReader("from stdin")
	out := &bytes.Buffer{}
	err := RunAppend(dir, []string{"from args"}, stdin, out)
	if err != nil {
		t.Fatalf("RunAppend: %v", err)
	}
	got := readNotes(t, dir)
	if !strings.Contains(got, "from args") {
		t.Errorf("notes.md missing args content; got:\n%s", got)
	}
	if strings.Contains(got, "from stdin") {
		t.Errorf("notes.md unexpectedly contains stdin content when args were given")
	}
}

func TestAppend_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	err := RunAppend(dir, nil, strings.NewReader("   \n  \n"), out)
	if err == nil {
		t.Errorf("expected error on empty body, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notes.md")); statErr == nil {
		data := readNotes(t, dir)
		if strings.TrimSpace(data) != "" {
			t.Errorf("notes.md should be empty after refused append; got:\n%s", data)
		}
	}
}

func TestAppend_PrependsToExisting(t *testing.T) {
	// New notes go at the top (§2 ordering invariant in the schema doc).
	dir := t.TempDir()

	// First note via the same code path.
	if err := RunAppend(dir, []string{"--title", "first", "older"}, nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("first append: %v", err)
	}
	// Second note. Different content so we can find the order.
	if err := RunAppend(dir, []string{"--title", "second", "newer"}, nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("second append: %v", err)
	}

	got := readNotes(t, dir)
	firstIdx := strings.Index(got, "newer")
	secondIdx := strings.Index(got, "older")
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("missing expected content; got:\n%s", got)
	}
	if firstIdx > secondIdx {
		t.Errorf("ordering invariant violated: newer note should appear first; got file:\n%s", got)
	}
	if !strings.Contains(got, "<!-- note -->") {
		t.Errorf("expected separator between notes; got:\n%s", got)
	}
}
