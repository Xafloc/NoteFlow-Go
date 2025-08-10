package handlers

import (
	"strconv"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// TasksHandler handles task-related HTTP requests
type TasksHandler struct {
	noteManager *services.NoteManager
}

// NewTasksHandler creates a new tasks handler
func NewTasksHandler(noteManager *services.NoteManager) *TasksHandler {
	return &TasksHandler{
		noteManager: noteManager,
	}
}

// GetTasks returns all active tasks as JSON
func (h *TasksHandler) GetTasks(c *fiber.Ctx) error {
	tasks := h.noteManager.GetActiveTasks()
	return c.JSON(tasks)
}

// UpdateTask updates a task's completion status
func (h *TasksHandler) UpdateTask(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid task index")
	}

	var req models.TaskUpdate
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request format")
	}

	if err := h.noteManager.UpdateTask(index, req.Checked); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Task not found: "+err.Error())
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}