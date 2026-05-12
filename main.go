package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/darren/noteflow-go/internal/app"
	"github.com/darren/noteflow-go/internal/cli"
	"github.com/darren/noteflow-go/internal/services"
)

const Version = "1.3.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("NoteFlow-Go v%s\n", Version)
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

	log.Fatal(application.Start())
}