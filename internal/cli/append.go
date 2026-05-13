// Package cli implements NoteFlow's command-line subcommands beyond the server.
//
// The append subcommand exists so that AI coding agents (Claude Code, Cursor,
// Aider, etc.) and shell scripts can add a note to the current folder's
// notes.md without spinning up the web server or re-serializing the file
// themselves. It is the "thin write-API" called out in
// docs/TODO.md → Long-term Direction goal 1.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Xafloc/NoteFlow-Go/internal/services"
)

const appendHelp = `USAGE:
    noteflow-go append [--title TITLE] [BODY...]

Appends a single note to notes.md in the current directory, via the same
write path as the web UI — same schema, same task parsing, same sigil
expansion. Designed for AI coding agents (Claude Code, Cursor, Aider)
and shell scripts that want to drop a note without spinning up the
server.

BODY:
    Argument(s) after the flags = note body (joined with spaces).
    No arguments     = body is read from stdin.
    Empty input is rejected with a non-zero exit.

FLAGS:
    --title TITLE    Optional title (rendered as "## TIMESTAMP - TITLE")
    --help, -h       Show this help and exit

MARKDOWN FEATURES (parsed at write time, same as the web UI):
    - [ ] task                  Task; appears in 'noteflow-go tasks'
    !p1 @2026-05-20 #tag        Inline task metadata (priority / due / tag)
    +http://example.com         Archived locally on save; rewrites to a
                                 link to the archived copy
    +file:src/foo.go#10-25      Inlines those lines as a fenced code
                                 block with language detected from .go

OUTPUT:
    appended: YYYY-MM-DD HH:MM:SS[ - title]

EXAMPLES:
    # Quick shell capture
    noteflow-go append "revisit the cache eviction logic"

    # Pipe a command's output as a note body
    git log --oneline -5 | noteflow-go append --title "last week's commits"

    # An AI agent leaving a finding it discovered
    echo "memory leak repro: GET /api/notes 1000x while idle" \
        | noteflow-go append --title "perf"

    # Embed a code snippet from the repo (expanded at save time)
    echo "schema lives at +file:internal/services/database.go#15-30" \
        | noteflow-go append --title "schema notes"

    # Drop in a task with priority + due date
    noteflow-go append "- [ ] !p1 @2026-05-20 ship the release notes"
`

// RunAppend appends a single note to notes.md in basePath.
//
// Usage:
//
//	noteflow append [--title TITLE] [BODY...]
//
// If no BODY arguments are given, the note body is read from stdin. If both
// are provided, BODY args win (stdin is ignored). An empty resulting body is
// rejected — we don't write blank notes.
//
// Output on success is a single line written to stdout:
//
//	appended: <timestamp>[ - <title>]
//
// so callers can capture it without parsing.
func RunAppend(basePath string, args []string, stdin io.Reader, stdout io.Writer) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprint(stdout, appendHelp)
			return nil
		}
	}

	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we surface errors ourselves; suppress flag's auto-printing

	title := fs.String("title", "", "optional note title")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	body := strings.Join(fs.Args(), " ")
	if body == "" && stdin != nil {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		body = string(data)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("refusing to append empty note (provide body args or pipe content on stdin)")
	}

	manager, err := services.NewNoteManager(basePath)
	if err != nil {
		return fmt.Errorf("open notes.md: %w", err)
	}
	if err := manager.AddNote(*title, body); err != nil {
		return fmt.Errorf("append note: %w", err)
	}

	// NoteManager prepends, so the new note is at index 0.
	added, err := manager.GetNote(0)
	if err != nil {
		// Append succeeded but we can't read it back — odd, but not fatal for the caller.
		fmt.Fprintln(stdout, "appended")
		return nil
	}

	timestamp := added.Timestamp.Format("2006-01-02 15:04:05")
	if added.Title != "" {
		fmt.Fprintf(stdout, "appended: %s - %s\n", timestamp, added.Title)
	} else {
		fmt.Fprintf(stdout, "appended: %s\n", timestamp)
	}
	return nil
}
