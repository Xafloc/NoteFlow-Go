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

	"github.com/darren/noteflow-go/internal/services"
)

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
