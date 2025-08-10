package handlers

import (
	"strconv"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
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