package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Xafloc/NoteFlow-Go/internal/app"
	"github.com/Xafloc/NoteFlow-Go/internal/cli"
	"github.com/Xafloc/NoteFlow-Go/internal/services"
)

const Version = "1.5.0"

const topHelp = `NoteFlow-Go v%s — local-first notebook that lives next to your code

USAGE:
    noteflow-go                       Start the web server in the current folder
    noteflow-go <subcommand> [args]   Run a subcommand

FLAGS (when starting the server):
    --no-browser     Don't auto-open the default browser on startup
    --version, -v    Print version and exit
    --help, -h       Show this help and exit

SUBCOMMANDS:
    append           Append a note to notes.md (for AI agents / scripts / shell)
    tasks            Query and manage tasks across every NoteFlow project

Run 'noteflow-go <subcommand> --help' for subcommand-specific options.
Docs: https://github.com/Xafloc/NoteFlow-Go
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("NoteFlow-Go v%s\n", Version)
			return
		case "--help", "-h":
			fmt.Printf(topHelp, Version)
			return
		case "append":
			workingDir, err := os.Getwd()
			if err != nil {
				log.Fatal("Failed to get working directory:", err)
			}
			if err := cli.RunAppend(workingDir, os.Args[2:], os.Stdin, os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "noteflow append:", err)
				os.Exit(1)
			}
			return
		case "tasks":
			dbPath, err := services.DefaultDatabasePath()
			if err != nil {
				log.Fatal("Failed to resolve task DB path:", err)
			}
			if err := cli.RunTasks(dbPath, os.Args[2:], os.Stdout, os.Stderr); err != nil {
				fmt.Fprintln(os.Stderr, "noteflow tasks:", err)
				os.Exit(1)
			}
			return
		}
	}

	// Get working directory for notes storage
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}

	// Create assets directory if it doesn't exist
	assetsDir := filepath.Join(workingDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		log.Fatal("Failed to create assets directory:", err)
	}

	// Initialize and start the application
	application, err := app.NewApp(workingDir, &WebAssets)
	if err != nil {
		log.Fatal("Failed to initialize application:", err)
	}

	// --no-browser disables auto-opening the user's default browser to the
	// server URL once it's listening. Useful for headless / SSH sessions.
	for _, arg := range os.Args[1:] {
		if arg == "--no-browser" {
			application.SetNoBrowser(true)
			break
		}
	}

	log.Fatal(application.Start())
}