package handlers

import (
	"path/filepath"
	"strings"

	"github.com/darren/noteflow-go/internal/models"
	"github.com/darren/noteflow-go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// FilesHandler handles file upload and management
type FilesHandler struct {
	noteManager *services.NoteManager
}

// NewFilesHandler creates a new files handler
func NewFilesHandler(noteManager *services.NoteManager) *FilesHandler {
	return &FilesHandler{
		noteManager: noteManager,
	}
}

// UploadFile handles file uploads via drag-and-drop or form submission
func (h *FilesHandler) UploadFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "No file provided")
	}

	// Read file data
	fileHeader, err := file.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to open file")
	}
	defer fileHeader.Close()

	// Read file content
	fileData := make([]byte, file.Size)
	if _, err := fileHeader.Read(fileData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to read file")
	}

	// Validate file size (max 50MB)
	maxSize := int64(50 * 1024 * 1024)
	if file.Size > maxSize {
		return fiber.NewError(fiber.StatusBadRequest, "File too large (max 50MB)")
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".pdf":  true, ".txt": true, ".md": true, ".doc": true, ".docx": true,
		".zip":  true, ".tar": true, ".gz": true,
		".json": true, ".xml": true, ".csv": true,
	}

	if !allowedExts[ext] {
		return fiber.NewError(fiber.StatusBadRequest, "File type not allowed")
	}

	// Get content type from header
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		// Try to guess from extension
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".pdf":
			contentType = "application/pdf"
		default:
			contentType = "application/octet-stream"
		}
	}

	// Save file
	filePath, isImage, err := h.noteManager.SaveFile(file.Filename, fileData, contentType)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save file: "+err.Error())
	}

	return c.JSON(map[string]interface{}{
		"filePath":    filePath,
		"isImage":     isImage,
		"contentType": contentType,
	})
}

// GetLinks returns information about archived links/sites
func (h *FilesHandler) GetLinks(c *fiber.Ctx) error {
	linkGroups, err := h.noteManager.GetArchivedLinks()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to get links: "+err.Error())
	}

	// Generate HTML output (similar to Python version)
	var htmlParts []string
	var markdownParts []string

	// Sort domains and build response
	for domain, data := range linkGroups {
		domainData := data.(map[string]interface{})
		archives := domainData["archives"].([]map[string]string)

		htmlParts = append(htmlParts, `<div class="archived-link">`)
		htmlParts = append(htmlParts, `<a href="#">`+domain+`</a>`)

		for _, archive := range archives {
			filename := archive["filename"]
			timestamp := archive["timestamp"]

			htmlParts = append(htmlParts,
				`<span class="archive-reference">`+
					`<a href="/assets/sites/`+filename+`" target="_blank">`+
					`site archive [`+timestamp+`]</a>`+
					`<span style="color:red;cursor:pointer;font-size:0.5rem; margin-left:5px;" `+
					`onclick="deleteArchive('`+filename+`')">delete</span>`+
					`</span>`)

			markdownParts = append(markdownParts,
				`[`+domain+` - [`+timestamp+`]](/assets/sites/`+filename+`)`)
		}

		htmlParts = append(htmlParts, `</div>`)
	}

	result := map[string]interface{}{
		"html":     strings.Join(htmlParts, "\n"),
		"markdown": strings.Join(markdownParts, "\n"),
	}

	return c.JSON(result)
}

// DeleteArchive deletes an archived website file
func (h *FilesHandler) DeleteArchive(c *fiber.Ctx) error {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request format")
	}

	if req.Filename == "" {
		return fiber.NewError(fiber.StatusBadRequest, "No filename provided")
	}

	if err := h.noteManager.DeleteArchivedSite(req.Filename); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete archive: "+err.Error())
	}

	return c.JSON(models.APIResponse{
		Status: "success",
	})
}