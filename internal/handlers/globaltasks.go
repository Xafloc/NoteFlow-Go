package handlers

import (
	"strconv"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// GlobalTasksHandler handles global task management across folders
type GlobalTasksHandler struct {
	taskRegistry *services.TaskRegistryService
}

// NewGlobalTasksHandler creates a new global tasks handler
func NewGlobalTasksHandler(taskRegistry *services.TaskRegistryService) *GlobalTasksHandler {
	return &GlobalTasksHandler{
		taskRegistry: taskRegistry,
	}
}

// GetGlobalTasks returns all tasks across all registered folders
// GET /api/global-tasks
func (gth *GlobalTasksHandler) GetGlobalTasks(c *fiber.Ctx) error {
	globalTasks, err := gth.taskRegistry.GetGlobalTasks()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: "Failed to get global tasks: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Status: "success",
		Data:   globalTasks,
	})
}

// UpdateGlobalTask updates the completion status of a global task
// POST /api/global-tasks/:id/toggle
func (gth *GlobalTasksHandler) UpdateGlobalTask(c *fiber.Ctx) error {
	taskIDStr := c.Params("id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "Invalid task ID",
		})
	}

	var req struct {
		Completed bool `json:"completed"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
	}

	err = gth.taskRegistry.UpdateGlobalTaskCompletion(taskID, req.Completed)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: "Failed to update task: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Status:  "success",
		Message: "Task updated successfully",
	})
}

// GetActiveFolders returns all active registered folders
// GET /api/global-folders
func (gth *GlobalTasksHandler) GetActiveFolders(c *fiber.Ctx) error {
	folders, err := gth.taskRegistry.GetActiveFolders()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: "Failed to get folders: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Status: "success",
		Data:   folders,
	})
}

// ForceSync forces a sync of all registered folders
// POST /api/global-sync
func (gth *GlobalTasksHandler) ForceSync(c *fiber.Ctx) error {
	err := gth.taskRegistry.ForceSync()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: "Failed to sync folders: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Status:  "success",
		Message: "All folders synced successfully",
	})
}

// AddFolder explicitly registers a folder the user typed in (rather than
// relying on the implicit auto-register that happens when noteflow-go is
// launched in a directory). Useful for power users with notes.md files
// they've moved/copied/created outside the normal flow.
// POST /api/global-folders/add  {"path": "/abs/or/relative"}
func (gth *GlobalTasksHandler) AddFolder(c *fiber.Ctx) error {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
	}
	if req.Path == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "path is required",
		})
	}
	folder, err := gth.taskRegistry.AddFolderByPath(req.Path)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{
		Status: "success",
		Data:   folder,
	})
}

// ForgetFolder soft-removes a folder from active tracking. The row stays
// in the DB with active=0 (audit trail); re-adding the same path later
// resurrects the same id with its history intact.
// POST /api/global-folders/:id/forget
func (gth *GlobalTasksHandler) ForgetFolder(c *fiber.Ctx) error {
	folderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "Invalid folder ID",
		})
	}
	if err := gth.taskRegistry.ForgetFolder(folderID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{
		Status:  "success",
		Message: "Folder forgotten",
	})
}

// SyncFolder re-syncs a single folder's notes.md. Useful when the user has
// edited the file externally and wants the global view to catch up without
// waiting for the 30s background tick.
// POST /api/global-folders/:id/sync
func (gth *GlobalTasksHandler) SyncFolder(c *fiber.Ctx) error {
	folderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "Invalid folder ID",
		})
	}
	if err := gth.taskRegistry.SyncFolderByID(folderID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{
		Status:  "success",
		Message: "Folder synced",
	})
}