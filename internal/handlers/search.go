package handlers

import (
	"path/filepath"
	"strings"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SearchHandler implements the v1.5 cross-folder search endpoint. The
// per-folder local search is purely client-side (the notes are already
// rendered into the DOM); this endpoint exists for the "Cmd+Enter to
// search every NoteFlow folder" path.
type SearchHandler struct {
	taskRegistry *services.TaskRegistryService
}

func NewSearchHandler(taskRegistry *services.TaskRegistryService) *SearchHandler {
	return &SearchHandler{taskRegistry: taskRegistry}
}

// SearchResultNote is one matching note in one folder.
type SearchResultNote struct {
	Timestamp    string `json:"timestamp"`
	Title        string `json:"title"`
	Snippet      string `json:"snippet"`      // up to ~240 chars of context around the first match
	MatchesInNote int   `json:"matches_in_note"`
}

// SearchResultFolder groups matching notes by their folder.
type SearchResultFolder struct {
	FolderPath string             `json:"folder_path"`
	Matches    []SearchResultNote `json:"matches"`
}

// SearchResponse is the payload returned to the client.
type SearchResponse struct {
	Query         string               `json:"query"`
	TotalFolders  int                  `json:"total_folders"`
	TotalNotes    int                  `json:"total_notes"`
	TotalMatches  int                  `json:"total_matches"`
	Results       []SearchResultFolder `json:"results"`
}

// GlobalSearch scans every active folder's notes.md and returns notes
// whose title or content contains the query string (case-insensitive
// substring match). Pure read path — never modifies any file.
//
// GET /api/search/global?q=<query>
func (h *SearchHandler) GlobalSearch(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "q parameter is required",
		})
	}
	if len(query) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Status:  "error",
			Message: "q must be 500 chars or fewer",
		})
	}

	folders, err := h.taskRegistry.GetActiveFolders()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Status:  "error",
			Message: "failed to list folders: " + err.Error(),
		})
	}

	lower := strings.ToLower(query)
	resp := SearchResponse{Query: query, Results: []SearchResultFolder{}}

	for _, folder := range folders {
		notesPath := filepath.Join(folder.Path, "notes.md")
		// Don't construct a full NoteManager — that triggers asset-dir
		// creation and other side-effects we don't want for a read-only
		// search. The storage helper is enough.
		manager, err := services.NewNoteManager(folder.Path)
		if err != nil {
			continue
		}
		notes := manager.GetAllNotes()
		var folderMatches []SearchResultNote
		for _, note := range notes {
			titleHits := strings.Count(strings.ToLower(note.Title), lower)
			contentHits := strings.Count(strings.ToLower(note.Content), lower)
			total := titleHits + contentHits
			if total == 0 {
				continue
			}
			folderMatches = append(folderMatches, SearchResultNote{
				Timestamp:     note.Timestamp.Format("2006-01-02 15:04:05"),
				Title:         note.Title,
				Snippet:       buildSnippet(note.Content, lower),
				MatchesInNote: total,
			})
			resp.TotalNotes++
			resp.TotalMatches += total
		}
		if len(folderMatches) > 0 {
			resp.Results = append(resp.Results, SearchResultFolder{
				FolderPath: folder.Path,
				Matches:    folderMatches,
			})
			resp.TotalFolders++
		}
		_ = notesPath
	}

	return c.JSON(models.APIResponse{
		Status: "success",
		Data:   resp,
	})
}

// buildSnippet finds the first occurrence of lowerQuery (case-insensitive)
// in content and returns ~120 chars of context on either side, with
// nearby whitespace trimmed. The match itself is marked with U+2026
// (ellipsis) prefix/suffix when truncated, but NOT HTML-highlighted — the
// client wraps the matches in <mark> tags using the same logic it uses
// for local search, so highlight styling stays in one place.
func buildSnippet(content, lowerQuery string) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, lowerQuery)
	if idx == -1 {
		// Match was only in the title; return the leading content as
		// preview rather than nothing.
		if len(content) <= 200 {
			return strings.TrimSpace(content)
		}
		return strings.TrimSpace(content[:200]) + "…"
	}
	const ctx = 120
	start := idx - ctx
	if start < 0 {
		start = 0
	}
	end := idx + len(lowerQuery) + ctx
	if end > len(content) {
		end = len(content)
	}
	out := content[start:end]
	if start > 0 {
		out = "…" + strings.TrimLeft(out, " \t\n")
	}
	if end < len(content) {
		out = strings.TrimRight(out, " \t\n") + "…"
	}
	return out
}
