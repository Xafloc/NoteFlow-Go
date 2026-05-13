package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Xafloc/NoteFlow-Go/internal/services"
	"github.com/gofiber/fiber/v2"
)

// Covers the v1.5 cross-folder search endpoint. Sets up two registered
// folders with known notes.md content, then exercises the query surface.

func setupSearchApp(t *testing.T) (*fiber.App, *services.TaskRegistryService, string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	registry, err := services.NewTaskRegistryService()
	if err != nil {
		t.Fatalf("NewTaskRegistryService: %v", err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirA, "notes.md"), []byte(
		"## 2026-05-13 09:00:00 - Sprint planning\n\n"+
			"- [ ] ship the release notes\n"+
			"- [ ] talk to the team about cache eviction\n"+
			"<!-- note -->\n"+
			"## 2026-05-12 14:00:00 - Random thoughts\n\n"+
			"Looked at Postgres tuning today. Nothing actionable.\n",
	), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "notes.md"), []byte(
		"## 2026-05-13 11:00:00 - Code review\n\n"+
			"The cache eviction logic in foo.go has a race condition. Tighten the locking.\n",
	), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.AddFolderByPath(dirA); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.AddFolderByPath(dirB); err != nil {
		t.Fatal(err)
	}

	h := NewSearchHandler(registry)
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).SendString(err.Error())
		},
	})
	app.Get("/api/search/global", h.GlobalSearch)
	return app, registry, dirA, dirB
}

func runSearch(t *testing.T, app *fiber.App, query string) SearchResponse {
	t.Helper()
	path := "/api/search/global?q=" + url.QueryEscape(query)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d, body %s", resp.StatusCode, string(body))
	}
	var env struct {
		Status string         `json:"status"`
		Data   SearchResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Status != "success" {
		t.Fatalf("status %q, want success", env.Status)
	}
	return env.Data
}

func TestGlobalSearch_TermInMultipleFolders(t *testing.T) {
	app, _, dirA, dirB := setupSearchApp(t)
	res := runSearch(t, app, "cache eviction")

	if res.Query != "cache eviction" {
		t.Errorf("Query echo wrong: %q", res.Query)
	}
	if res.TotalFolders != 2 {
		t.Errorf("expected 2 matching folders, got %d", res.TotalFolders)
	}
	if res.TotalNotes != 2 {
		t.Errorf("expected 2 matching notes (one per folder), got %d", res.TotalNotes)
	}
	if res.TotalMatches < 2 {
		t.Errorf("expected at least 2 substring matches, got %d", res.TotalMatches)
	}
	// Folders should appear in the results.
	pathsFound := make(map[string]bool)
	for _, f := range res.Results {
		pathsFound[f.FolderPath] = true
	}
	if !pathsFound[dirA] {
		t.Errorf("folder %q missing from results", dirA)
	}
	if !pathsFound[dirB] {
		t.Errorf("folder %q missing from results", dirB)
	}
}

func TestGlobalSearch_TitleOnlyMatch(t *testing.T) {
	// "sprint" only appears in the title of one note. Should still surface.
	app, _, _, _ := setupSearchApp(t)
	res := runSearch(t, app, "sprint")
	if res.TotalNotes != 1 {
		t.Errorf("expected 1 matching note for title-only term, got %d", res.TotalNotes)
	}
	if len(res.Results) != 1 || len(res.Results[0].Matches) != 1 {
		t.Fatalf("unexpected result shape: %+v", res.Results)
	}
	if !strings.Contains(strings.ToLower(res.Results[0].Matches[0].Title), "sprint") {
		t.Errorf("expected matching note's title to contain 'sprint', got %q",
			res.Results[0].Matches[0].Title)
	}
	if res.Results[0].Matches[0].Snippet == "" {
		t.Errorf("title-only match should still include a preview snippet")
	}
}

func TestGlobalSearch_CaseInsensitive(t *testing.T) {
	app, _, _, _ := setupSearchApp(t)
	lower := runSearch(t, app, "postgres")
	upper := runSearch(t, app, "POSTGRES")
	mixed := runSearch(t, app, "PoStGrEs")
	if lower.TotalMatches != upper.TotalMatches || upper.TotalMatches != mixed.TotalMatches {
		t.Errorf("case-insensitivity broken: lower=%d upper=%d mixed=%d",
			lower.TotalMatches, upper.TotalMatches, mixed.TotalMatches)
	}
	if lower.TotalMatches == 0 {
		t.Errorf("expected at least one match for 'postgres'")
	}
}

func TestGlobalSearch_NoMatches(t *testing.T) {
	app, _, _, _ := setupSearchApp(t)
	res := runSearch(t, app, "this-string-cannot-possibly-appear-anywhere")
	if res.TotalNotes != 0 || res.TotalMatches != 0 || res.TotalFolders != 0 {
		t.Errorf("expected zero totals, got %+v", res)
	}
	if len(res.Results) != 0 {
		t.Errorf("expected empty results slice, got %d", len(res.Results))
	}
}

func TestGlobalSearch_EmptyQueryRejected(t *testing.T) {
	app, _, _, _ := setupSearchApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/search/global?q=", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty query, got %d", resp.StatusCode)
	}
}

func TestGlobalSearch_SnippetTrimsAroundFirstMatch(t *testing.T) {
	app, _, _, _ := setupSearchApp(t)
	res := runSearch(t, app, "race condition")
	if len(res.Results) == 0 {
		t.Fatalf("expected results")
	}
	snippet := res.Results[0].Matches[0].Snippet
	if !strings.Contains(strings.ToLower(snippet), "race condition") {
		t.Errorf("snippet should contain the matched text: %q", snippet)
	}
	if len(snippet) > 300 {
		t.Errorf("snippet length %d exceeds reasonable bound", len(snippet))
	}
}
