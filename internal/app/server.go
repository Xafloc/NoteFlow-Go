package app

import (
	"embed"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/darren/noteflow-go/internal/handlers"
	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// App represents the main application
type App struct {
	fiber           *fiber.App
	noteManager     *services.NoteManager
	templateService *services.TemplateService
	taskRegistry    *services.TaskRegistryService
	config          *models.Config
	configPath      string
	basePath        string
	port            int
}

// NewApp creates a new application instance
func NewApp(basePath string, webAssets *embed.FS) (*App, error) {
	// Initialize configuration
	configPath := getConfigPath()
	config, err := models.LoadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		config = models.DefaultConfig()
	}

	// Initialize note manager
	noteManager, err := services.NewNoteManager(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize note manager: %w", err)
	}

	// Initialize template service
	templateService, err := services.NewTemplateService(webAssets)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template service: %w", err)
	}

	// Initialize task registry service
	taskRegistry, err := services.NewTaskRegistryService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task registry: %w", err)
	}

	// Register this folder with the task registry
	if err := taskRegistry.RegisterFolder(basePath, noteManager); err != nil {
		log.Printf("Warning: failed to register folder for global tasks: %v", err)
	}

	// Find available port
	port, err := findFreePort(8000)
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}

	app := &App{
		noteManager:     noteManager,
		templateService: templateService,
		taskRegistry:    taskRegistry,
		config:          config,
		configPath:      configPath,
		basePath:        basePath,
		port:            port,
	}

	app.setupFiber()
	app.setupRoutes()

	return app, nil
}

// setupFiber initializes the Fiber app with middleware
func (a *App) setupFiber() {
	a.fiber = fiber.New(fiber.Config{
		AppName:      "NoteFlow",
		ServerHeader: "NoteFlow/1.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(models.APIResponse{
				Status:  "error",
				Message: err.Error(),
			})
		},
	})

	// Middleware
	a.fiber.Use(recover.New())
	a.fiber.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Serve static assets from basePath
	assetsPath := filepath.Join(a.basePath, "assets")
	a.fiber.Static("/assets", assetsPath)

	// Serve embedded static files (favicon, etc.)
	a.fiber.Static("/static", "./web/static")
}

// setupRoutes configures all application routes
func (a *App) setupRoutes() {
	// Initialize handlers
	notesHandler := handlers.NewNotesHandler(a.noteManager)
	tasksHandler := handlers.NewTasksHandler(a.noteManager)
	filesHandler := handlers.NewFilesHandler(a.noteManager)
	themesHandler := handlers.NewThemesHandler(a.config, a.configPath)
	globalTasksHandler := handlers.NewGlobalTasksHandler(a.taskRegistry)

	// Root route - serve main HTML page
	a.fiber.Get("/", a.serveIndex)
	a.fiber.Get("/global-tasks", a.serveGlobalTasks)
	a.fiber.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.Redirect("/static/favicon.ico")
	})

	// API routes
	api := a.fiber.Group("/api")

	// Note routes
	api.Get("/notes", notesHandler.GetNotes)
	api.Post("/notes", notesHandler.AddNote)
	api.Get("/notes/:index", notesHandler.GetNote)
	api.Put("/notes/:index", notesHandler.UpdateNote)
	api.Delete("/notes/:index", notesHandler.DeleteNote)

	// Task routes
	api.Get("/tasks", tasksHandler.GetTasks)
	api.Post("/tasks/:index", tasksHandler.UpdateTask)

	// File routes
	api.Post("/upload-file", filesHandler.UploadFile)
	api.Get("/links", filesHandler.GetLinks)
	api.Post("/archive-delete", filesHandler.DeleteArchive)

	// Theme routes
	api.Get("/themes", themesHandler.GetThemes)
	api.Get("/current-theme", themesHandler.GetCurrentTheme)
	api.Post("/theme", themesHandler.SetTheme)
	api.Post("/save-theme", themesHandler.SaveTheme)

	// Global task routes
	api.Get("/global-tasks", globalTasksHandler.GetGlobalTasks)
	api.Post("/global-tasks/:id/toggle", globalTasksHandler.UpdateGlobalTask)
	api.Get("/global-folders", globalTasksHandler.GetActiveFolders)
	api.Post("/global-sync", globalTasksHandler.ForceSync)

	// Shutdown route
	api.Post("/shutdown", func(c *fiber.Ctx) error {
		go func() {
			log.Println("Shutting down server...")
			if err := a.fiber.Shutdown(); err != nil {
				log.Printf("Error during shutdown: %v", err)
			}
		}()
		return c.JSON(models.APIResponse{
			Status:  "success",
			Message: "shutting down",
		})
	})
}

// serveIndex serves the main HTML page with theme styling
func (a *App) serveIndex(c *fiber.Ctx) error {
	html, err := a.templateService.RenderIndex(a.config, a.basePath)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to render page: "+err.Error())
	}
	
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// serveGlobalTasks serves the global tasks page with theme styling
func (a *App) serveGlobalTasks(c *fiber.Ctx) error {
	html, err := a.templateService.RenderGlobalTasks(a.config, a.basePath)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to render global tasks page: "+err.Error())
	}
	
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// Start starts the web server
func (a *App) Start() error {
	addr := fmt.Sprintf(":%d", a.port)
	log.Printf("NoteFlow server starting on http://localhost%s", addr)
	log.Printf("Using folder: %s", a.basePath)
	return a.fiber.Listen(addr)
}

// GetPort returns the port the server is running on
func (a *App) GetPort() int {
	return a.port
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "noteflow.json"
	}
	return filepath.Join(homeDir, ".config", "noteflow", "noteflow.json")
}

// findFreePort finds an available port starting from the given port
func findFreePort(startPort int) (int, error) {
	for port := startPort; port < 65535; port++ {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found")
}

