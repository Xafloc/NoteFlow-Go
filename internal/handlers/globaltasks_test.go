package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Xafloc/NoteFlow-Go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// These tests cover the v1.4 folders-management endpoints: add an arbitrary
// path the user typed in, sync just that folder, soft-forget it (audit row
// preserved), and the failure paths for each.

func setupFoldersApp(t *testing.T) (*fiber.App, *services.TaskRegistryService, string) {
	t.Helper()
	// Redirect ~/.config so the test never touches the real DB.
	t.Setenv("HOME", t.TempDir())
	registry, err := services.NewTaskRegistryService()
	if err != nil {
		t.Fatalf("NewTaskRegistryService: %v", err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	h := NewGlobalTasksHandler(registry)
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).SendString(err.Error())
		},
	})
	app.Get("/api/global-folders", h.GetActiveFolders)
	app.Post("/api/global-folders/add", h.AddFolder)
	app.Post("/api/global-folders/:id/forget", h.ForgetFolder)
	app.Post("/api/global-folders/:id/sync", h.SyncFolder)
	return app, registry, t.TempDir() // returned tempdir is a fresh empty folder we can use as a project root
}

// envelope mirrors models.APIResponse without needing to import the type.
type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func decode(t *testing.T, resp *http.Response) envelope {
	t.Helper()
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("decode: %v\nbody: %s", err, string(body))
	}
	return env
}

func TestAddFolder_ValidPath(t *testing.T) {
	app, _, projDir := setupFoldersApp(t)

	body, _ := json.Marshal(map[string]string{"path": projDir})
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, string(buf))
	}
	env := decode(t, resp)
	if env.Status != "success" {
		t.Errorf("status = %q, want success", env.Status)
	}
	// notes.md must have been created (NewNoteManager creates it if missing)
	if _, err := os.Stat(filepath.Join(projDir, "notes.md")); err != nil {
		t.Errorf("notes.md not created: %v", err)
	}
}

func TestAddFolder_NonExistentPath(t *testing.T) {
	app, _, _ := setupFoldersApp(t)

	body, _ := json.Marshal(map[string]string{"path": "/this/should/not/exist/anywhere"})
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for nonexistent path, got %d", resp.StatusCode)
	}
}

func TestAddFolder_FilePathRejected(t *testing.T) {
	// A regular file (not a directory) must be rejected.
	app, _, projDir := setupFoldersApp(t)
	filePath := filepath.Join(projDir, "iam-a-file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"path": filePath})
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for file-as-path, got %d", resp.StatusCode)
	}
	env := decode(t, resp)
	if !strings.Contains(env.Message, "not a directory") {
		t.Errorf("expected 'not a directory' in message; got %q", env.Message)
	}
}

func TestAddFolder_EmptyPath(t *testing.T) {
	app, _, _ := setupFoldersApp(t)
	body, _ := json.Marshal(map[string]string{"path": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty path, got %d", resp.StatusCode)
	}
}

func TestForgetFolder_SoftDeletesAndPreservesAuditRow(t *testing.T) {
	app, registry, projDir := setupFoldersApp(t)

	// Add a folder, capture its id.
	body, _ := json.Marshal(map[string]string{"path": projDir})
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	env := decode(t, resp)
	var folder struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &folder); err != nil {
		t.Fatalf("decode folder: %v", err)
	}

	// Forget it.
	forgetReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/global-folders/%d/forget", folder.ID), nil)
	forgetResp, _ := app.Test(forgetReq)
	if forgetResp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(forgetResp.Body)
		t.Fatalf("forget: status %d, body %s", forgetResp.StatusCode, string(buf))
	}

	// GetActiveFolders must no longer include it.
	listReq := httptest.NewRequest(http.MethodGet, "/api/global-folders", nil)
	listResp, _ := app.Test(listReq)
	listEnv := decode(t, listResp)
	if !strings.Contains(string(listEnv.Data), "[]") && strings.Contains(string(listEnv.Data), projDir) {
		t.Errorf("forgotten folder still in active list: %s", string(listEnv.Data))
	}

	// Re-adding via the same path must resurrect (same id) thanks to the
	// existing RegisterFolder upsert.
	reAddBody, _ := json.Marshal(map[string]string{"path": projDir})
	reAddReq := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(reAddBody))
	reAddReq.Header.Set("Content-Type", "application/json")
	reAddResp, _ := app.Test(reAddReq)
	reAddEnv := decode(t, reAddResp)
	var reAdded struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(reAddEnv.Data, &reAdded); err != nil {
		t.Fatalf("decode re-added: %v", err)
	}
	if reAdded.ID != folder.ID {
		t.Errorf("re-adding forgotten folder should resurrect id %d, got %d", folder.ID, reAdded.ID)
	}

	_ = registry // keeps the linter quiet about an otherwise-unused var
}

func TestForgetFolder_InvalidID(t *testing.T) {
	app, _, _ := setupFoldersApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/not-a-number/forget", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-integer id, got %d", resp.StatusCode)
	}
}

func TestSyncFolder_RoundTrip(t *testing.T) {
	app, _, projDir := setupFoldersApp(t)

	// Seed notes.md with one task so the sync has something to record.
	if err := os.WriteFile(
		filepath.Join(projDir, "notes.md"),
		[]byte("## 2026-05-13 09:00:00 - test\n\n- [ ] sync me\n"),
		0644,
	); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	// Add.
	addBody, _ := json.Marshal(map[string]string{"path": projDir})
	addReq := httptest.NewRequest(http.MethodPost, "/api/global-folders/add", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addResp, _ := app.Test(addReq)
	addEnv := decode(t, addResp)
	var folder struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(addEnv.Data, &folder); err != nil {
		t.Fatalf("decode folder: %v", err)
	}

	// Sync explicitly.
	syncReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/global-folders/%d/sync", folder.ID), nil)
	syncResp, _ := app.Test(syncReq)
	if syncResp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(syncResp.Body)
		t.Fatalf("sync: status %d, body %s", syncResp.StatusCode, string(buf))
	}
}

func TestSyncFolder_UnknownID(t *testing.T) {
	app, _, _ := setupFoldersApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/global-folders/99999/sync", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for unknown folder, got %d", resp.StatusCode)
	}
}
