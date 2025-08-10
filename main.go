package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/darren/noteflow-go/internal/app"
)

const Version = "1.0.0"

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("NoteFlow-Go v%s\n", Version)
		os.Exit(0)
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