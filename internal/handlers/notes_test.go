package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darren/noteflow-go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// First handler-layer tests for the project — Goal 3 ("boring/reliable")
// has been calling for these. We use Fiber's built-in test server with an
// in-memory NoteManager rooted at t.TempDir(). This means each test gets
// a fresh notes.md and never touches a real config dir.

func setupNotesApp(t *testing.T) *fiber.App {
	t.Helper()
	dir := t.TempDir()
	mgr, err := services.NewNoteManager(dir)
	if err != nil {
		t.Fatalf("NewNoteManager: %v", err)
	}
	h := NewNotesHandler(mgr)

	app := fiber.New(fiber.Config{
		// Surface handler errors as the response body for clearer test failures.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).SendString(err.Error())
		},
	})
	app.Get("/notes", h.GetNotes)
	app.Post("/notes", h.AddNote)
	app.Get("/notes/:index", h.GetNote)
	return app
}

func TestNotesHandler_GetNotes_Empty(t *testing.T) {
	app := setupNotesApp(t)

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	// Empty notes file renders to empty HTML (no notes to display).
	if strings.TrimSpace(string(body)) != "" {
		t.Errorf("expected empty body for no notes, got: %q", string(body))
	}
}

func TestNotesHandler_AddNote_JSON(t *testing.T) {
	app := setupNotesApp(t)

	body := bytes.NewBufferString(`{"title":"smoke","content":"first note body"}`)
	req := httptest.NewRequest(http.MethodPost, "/notes", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, string(buf))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["status"] != "success" {
		t.Errorf("response status = %v, want success", out["status"])
	}

	// Verify it was actually persisted via a second GET.
	getResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/notes/0", nil))
	if err != nil {
		t.Fatalf("GET /notes/0: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", getResp.StatusCode)
	}
	getBody, _ := io.ReadAll(getResp.Body)
	if !strings.Contains(string(getBody), "first note body") {
		t.Errorf("persisted body missing expected content: %s", string(getBody))
	}
	if !strings.Contains(string(getBody), "smoke") {
		t.Errorf("persisted body missing title: %s", string(getBody))
	}
}

func TestNotesHandler_AddNote_EmptyContentRejected(t *testing.T) {
	app := setupNotesApp(t)
	body := bytes.NewBufferString(`{"title":"x","content":""}`)
	req := httptest.NewRequest(http.MethodPost, "/notes", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		buf, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 400; body = %s", resp.StatusCode, string(buf))
	}
}

func TestNotesHandler_AddNote_InvalidJSON(t *testing.T) {
	app := setupNotesApp(t)
	body := bytes.NewBufferString(`{not json}`)
	req := httptest.NewRequest(http.MethodPost, "/notes", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for malformed JSON", resp.StatusCode)
	}
}

func TestNotesHandler_GetNote_OutOfRange(t *testing.T) {
	app := setupNotesApp(t)
	req := httptest.NewRequest(http.MethodGet, "/notes/9999", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for out-of-range index", resp.StatusCode)
	}
}

func TestNotesHandler_GetNote_NonIntegerIndex(t *testing.T) {
	app := setupNotesApp(t)
	req := httptest.NewRequest(http.MethodGet, "/notes/not-a-number", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for non-integer index", resp.StatusCode)
	}
}
