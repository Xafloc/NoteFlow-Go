package handlers

import (
	"strconv"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// NotesHandler handles note-related HTTP requests
type NotesHandler struct {
	noteManager *services.NoteManager
}

// NewNotesHandler creates a new notes handler
func NewNotesHandler(noteManager *services.NoteManager) *NotesHandler {
	return &NotesHandler{
		noteManager: noteManager,
	}
}

// GetNotes returns all notes as HTML
func (h *NotesHandler) GetNotes(c *fiber.Ctx) error {
	html, err := h.noteManager.RenderNotesHTML()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to render notes: "+err.Error())
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// AddNote creates a new note
func (h *NotesHandler) AddNote(c *fiber.Ctx) error {
	var title, content string
	
	// Check content type to handle both JSON and FormData
	contentType := c.Get("Content-Type")
	if contentType == "application/json" {
		// Handle JSON request (API calls)
		var req models.NoteRequest
		if err := c.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid JSON request format")
		}
		title = req.Title
		content = req.Content
	} else {
		// Handle FormData request (web form)
		title = c.FormValue("title")
		content = c.FormValue("content")
	}

	if content == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Content cannot be empty")
	}

	if err := h.noteManager.AddNote(title, content); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to add note: "+err.Error())
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}

// GetNote returns a specific note for editing
func (h *NotesHandler) GetNote(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid note index")
	}

	note, err := h.noteManager.GetNote(index)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Note not found")
	}

	response := map[string]interface{}{
		"timestamp": note.Timestamp.Format("2006-01-02 15:04:05"),
		"content":   note.Content,
		"title":     note.Title,
	}

	return c.JSON(response)
}

// UpdateNote updates an existing note
func (h *NotesHandler) UpdateNote(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid note index")
	}

	var title, content string
	
	// Check content type to handle both JSON and FormData
	contentType := c.Get("Content-Type")
	if contentType == "application/json" {
		// Handle JSON request (API calls)
		var req models.NoteRequest
		if err := c.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid JSON request format")
		}
		title = req.Title
		content = req.Content
	} else {
		// Handle FormData request (web form)
		title = c.FormValue("title")
		content = c.FormValue("content")
	}

	if err := h.noteManager.UpdateNote(index, title, content); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to update note: "+err.Error())
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}

// DeleteNote deletes a specific note
func (h *NotesHandler) DeleteNote(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid note index")
	}

	if err := h.noteManager.DeleteNote(index); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Note not found")
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}