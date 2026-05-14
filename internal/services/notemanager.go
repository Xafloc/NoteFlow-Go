package services

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/storage"
)

// NoteManager manages notes and tasks for a specific project
type NoteManager struct {
	notes         []*models.Note
	checkboxIndex int
	storage       *storage.FileStorage
	renderer      *MarkdownRenderer
	mu            sync.RWMutex
	needsSave     bool
}

// NewNoteManager creates a new note manager for the given base path
func NewNoteManager(basePath string) (*NoteManager, error) {
	storage := storage.NewFileStorage(basePath)
	renderer := NewMarkdownRenderer()

	// Ensure necessary directories exist
	if err := storage.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	manager := &NoteManager{
		notes:         make([]*models.Note, 0),
		checkboxIndex: 0,
		storage:       storage,
		renderer:      renderer,
	}

	// Load existing notes
	if err := manager.loadNotes(); err != nil {
		return nil, fmt.Errorf("failed to load notes: %w", err)
	}

	return manager, nil
}

// loadNotes loads all notes from storage
func (nm *NoteManager) loadNotes() error {
	notes, err := nm.storage.LoadNotes()
	if err != nil {
		return err
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.notes = notes
	nm.assignTaskIndices()

	return nil
}

// assignTaskIndices assigns unique indices to all tasks
func (nm *NoteManager) assignTaskIndices() {
	index := 0
	for _, note := range nm.notes {
		for _, task := range note.Tasks {
			task.Index = index
			index++
		}
	}
	nm.checkboxIndex = index
}

// AddNote adds a new note to the collection
func (nm *NoteManager) AddNote(title, content string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Process any +http links and +file: snippets in content.
	processedContent, err := nm.processArchiveLinks(content)
	if err != nil {
		// Log error but continue with original content
		processedContent = content
	}
	processedContent = nm.processCodeSnippets(processedContent)

	note := models.NewNote(title, processedContent)
	
	// Assign task indices
	for _, task := range note.Tasks {
		task.Index = nm.checkboxIndex
		nm.checkboxIndex++
	}

	// Insert at the beginning (newest first)
	nm.notes = append([]*models.Note{note}, nm.notes...)
	nm.needsSave = true

	return nm.save()
}

// UpdateNote updates an existing note
func (nm *NoteManager) UpdateNote(index int, title, content string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if index < 0 || index >= len(nm.notes) {
		return fmt.Errorf("note index %d out of range", index)
	}

	// Process any +http links and +file: snippets in content.
	processedContent, err := nm.processArchiveLinks(content)
	if err != nil {
		// Log error but continue with original content
		processedContent = content
	}
	processedContent = nm.processCodeSnippets(processedContent)

	note := nm.notes[index]
	oldTaskCount := len(note.Tasks)

	note.Update(title, processedContent)

	// Update task indices if task count changed
	if len(note.Tasks) != oldTaskCount {
		nm.reassignTaskIndicesFromNote(index)
	}

	nm.needsSave = true
	return nm.save()
}

// DeleteNote removes a note from the collection
func (nm *NoteManager) DeleteNote(index int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if index < 0 || index >= len(nm.notes) {
		return fmt.Errorf("note index %d out of range", index)
	}

	// Remove note from slice
	nm.notes = append(nm.notes[:index], nm.notes[index+1:]...)
	
	// Reassign all task indices since we removed a note
	nm.assignTaskIndices()
	
	nm.needsSave = true
	return nm.save()
}

// GetNote returns a note by index
func (nm *NoteManager) GetNote(index int) (*models.Note, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if index < 0 || index >= len(nm.notes) {
		return nil, fmt.Errorf("note index %d out of range", index)
	}

	return nm.notes[index], nil
}

// GetAllNotes returns all notes
func (nm *NoteManager) GetAllNotes() []*models.Note {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// Return a copy to prevent external modification
	notes := make([]*models.Note, len(nm.notes))
	copy(notes, nm.notes)
	return notes
}

// GetActiveTasksreturns all unchecked tasks across all notes
func (nm *NoteManager) GetActiveTasks() []*models.TaskInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var tasks []*models.TaskInfo
	for _, note := range nm.notes {
		tasks = append(tasks, note.GetUncheckedTasks()...)
	}
	return tasks
}

// UpdateTask updates a task's completion status
func (nm *NoteManager) UpdateTask(taskIndex int, checked bool) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Find the task across all notes
	for _, note := range nm.notes {
		if note.UpdateTask(taskIndex, checked) {
			nm.needsSave = true
			return nm.save()
		}
	}

	return fmt.Errorf("task with index %d not found", taskIndex)
}

// RenderNotesHTML returns HTML representation of all notes
func (nm *NoteManager) RenderNotesHTML() (string, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var htmlParts []string

	for i, note := range nm.notes {
		timestamp := note.Timestamp.Format("2006-01-02 15:04:05")
		titleDisplay := timestamp
		if note.Title != "" {
			titleDisplay += " - " + note.Title
		}

		noteHTML, err := nm.renderer.RenderNoteHTML(note.Content, titleDisplay, note.Title, i)
		if err != nil {
			return "", fmt.Errorf("failed to render note %d: %w", i, err)
		}

		htmlParts = append(htmlParts, noteHTML)
	}

	return strings.Join(htmlParts, ""), nil
}

// save persists notes to storage if needed
func (nm *NoteManager) save() error {
	if !nm.needsSave {
		return nil
	}

	if err := nm.storage.SaveNotes(nm.notes); err != nil {
		return fmt.Errorf("failed to save notes: %w", err)
	}

	nm.needsSave = false
	return nil
}

// reassignTaskIndicesFromNote reassigns task indices starting from a specific note
func (nm *NoteManager) reassignTaskIndicesFromNote(startNoteIndex int) {
	index := nm.checkboxIndex

	// Count existing tasks before the start note
	for i := 0; i < startNoteIndex && i < len(nm.notes); i++ {
		for range nm.notes[i].Tasks {
			index--
		}
	}

	// Reassign from start note onwards
	for i := startNoteIndex; i < len(nm.notes); i++ {
		for _, task := range nm.notes[i].Tasks {
			task.Index = index
			index++
		}
	}

	// Update the global counter
	nm.checkboxIndex = index
}

// codeSnippetSigilRE matches the +file: sigil used to attach code snippets:
//
//	+file:relative/path.go             — entire file
//	+file:relative/path.go#10          — line 10 only
//	+file:relative/path.go#10-25       — lines 10 through 25 (inclusive)
//
// The path must be relative to the project root and may not escape it via
// `..` segments or absolute paths (see resolveSnippetPath). The grammar
// deliberately stops at whitespace and `#` so a path containing spaces must
// be avoided — same constraint as URLs in markdown.
var codeSnippetSigilRE = regexp.MustCompile(`\+file:([^\s#]+)(?:#(\d+)(?:-(\d+))?)?`)

// snippetLangByExt maps common file extensions to fenced-code-block language
// hints. Anything not listed renders as a plain block (no syntax highlight).
var snippetLangByExt = map[string]string{
	".go":   "go",
	".js":   "javascript",
	".ts":   "typescript",
	".jsx":  "jsx",
	".tsx":  "tsx",
	".py":   "python",
	".rb":   "ruby",
	".rs":   "rust",
	".java": "java",
	".c":    "c",
	".h":    "c",
	".cpp":  "cpp",
	".hpp":  "cpp",
	".cs":   "csharp",
	".php":  "php",
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "bash",
	".fish": "fish",
	".sql":  "sql",
	".html": "html",
	".css":  "css",
	".scss": "scss",
	".json": "json",
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".xml":  "xml",
	".md":   "markdown",
	".mod":  "go-mod",
	".sum":  "",
	".txt":  "",
}

// processCodeSnippets resolves +file: sigils to fenced code blocks containing
// the referenced content. It mirrors the +http archive flow: replacement is
// eager (happens once at note save time) so the resulting notes.md is
// readable offline and parseable by AI agents without further resolution.
//
// Errors are non-fatal — a sigil that can't be resolved (missing file, path
// escape attempt, read failure) is left in place and logged. This matches
// how processArchiveLinks handles bad URLs.
//
// Path sandboxing: paths must be relative to the project root and the
// resolved absolute path must remain under that root. Symlinks pointing
// outside the project are rejected by the prefix check after Abs/Clean.
func (nm *NoteManager) processCodeSnippets(content string) string {
	return codeSnippetSigilRE.ReplaceAllStringFunc(content, func(match string) string {
		m := codeSnippetSigilRE.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		relPath := m[1]
		startLine, endLine := 0, 0
		if len(m) >= 3 && m[2] != "" {
			fmt.Sscanf(m[2], "%d", &startLine)
		}
		if len(m) >= 4 && m[3] != "" {
			fmt.Sscanf(m[3], "%d", &endLine)
		}

		absPath, ok := resolveSnippetPath(nm.storage.BasePath, relPath)
		if !ok {
			log.Printf("Warning: snippet path %q is outside project root or invalid", relPath)
			return match
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("Warning: failed to read snippet %s: %v", absPath, err)
			return match
		}

		snippet, displayRange, err := extractSnippetLines(string(data), startLine, endLine)
		if err != nil {
			log.Printf("Warning: %v", err)
			return match
		}

		lang := snippetLangByExt[strings.ToLower(filepath.Ext(relPath))]
		ref := relPath
		if displayRange != "" {
			ref = relPath + "#" + displayRange
		}

		// Emit a markdown fenced block. Leading newline so it stands on its
		// own line even when the sigil was mid-paragraph; trailing newline
		// keeps the closing fence isolated from any following text.
		return fmt.Sprintf("\n```%s\n// %s\n%s\n```\n", lang, ref, snippet)
	})
}

// resolveSnippetPath joins relPath onto basePath and verifies the result
// stays inside basePath after symlink resolution. Returns the absolute path
// and ok=true on success; ok=false rejects the path.
//
// This is the security boundary for the +file: sigil. Anything that gets
// past this function can be read into a note. Rejections:
//   - Absolute relPath
//   - Empty relPath
//   - relPath that climbs out of basePath via `..`
//   - Resolved symlink target outside basePath
func resolveSnippetPath(basePath, relPath string) (string, bool) {
	if relPath == "" || filepath.IsAbs(relPath) {
		return "", false
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", false
	}
	absJoined, err := filepath.Abs(filepath.Join(basePath, relPath))
	if err != nil {
		return "", false
	}
	// EvalSymlinks both sides so platform-level symlinks (macOS's /var ->
	// /private/var, or any user-created link inside the project tree) don't
	// produce a spurious mismatch in the prefix check below.
	if eval, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = eval
	}
	if eval, err := filepath.EvalSymlinks(absJoined); err == nil {
		absJoined = eval
	}
	baseWithSep := absBase + string(filepath.Separator)
	if absJoined != absBase && !strings.HasPrefix(absJoined, baseWithSep) {
		return "", false
	}
	return absJoined, true
}

// extractSnippetLines slices the content to the requested line range.
// startLine and endLine are 1-based and inclusive; 0 means "unspecified."
//
//	(0, 0)       — return the entire file
//	(N, 0)       — return just line N
//	(N, M)       — return lines N through M (inclusive)
//
// Returns the snippet text, a human-friendly range string for the comment
// header ("" when full file, "10" for one line, "10-25" for a range), and
// an error for out-of-range or inverted ranges.
func extractSnippetLines(content string, startLine, endLine int) (string, string, error) {
	if startLine == 0 && endLine == 0 {
		return strings.TrimRight(content, "\n"), "", nil
	}
	if endLine == 0 {
		endLine = startLine
	}
	if startLine < 1 || endLine < startLine {
		return "", "", fmt.Errorf("invalid snippet line range %d-%d", startLine, endLine)
	}
	lines := strings.Split(content, "\n")
	// A file that ends with "\n" produces a trailing empty string from Split;
	// drop it so line numbers line up with what an editor would show.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if startLine > len(lines) {
		return "", "", fmt.Errorf("snippet start line %d exceeds file length %d", startLine, len(lines))
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	selected := lines[startLine-1 : endLine]
	rangeStr := fmt.Sprintf("%d", startLine)
	if endLine != startLine {
		rangeStr = fmt.Sprintf("%d-%d", startLine, endLine)
	}
	return strings.Join(selected, "\n"), rangeStr, nil
}

// processArchiveLinks processes +http links in content and archives the websites
func (nm *NoteManager) processArchiveLinks(content string) (string, error) {
	// Regular expression to match +http(s)://... links
	re := regexp.MustCompile(`\+https?://[^\s\)]+`)
	
	// Find all matches
	matches := re.FindAllString(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	processedContent := content
	
	for _, match := range matches {
		// Remove the + prefix to get the actual URL
		url := strings.TrimPrefix(match, "+")
		
		// Archive the website
		archiveInfo, err := nm.archiveWebsite(url)
		if err != nil {
			log.Printf("Warning: failed to archive %s: %v", url, err)
			continue
		}
		
		// Replace +URL with archived link reference
		archiveLink := fmt.Sprintf("[%s](%s) (archived %s)", 
			archiveInfo.Title, 
			archiveInfo.FilePath, 
			archiveInfo.Timestamp.Format("2006-01-02 15:04"))
		
		processedContent = strings.Replace(processedContent, match, archiveLink, 1)
	}
	
	return processedContent, nil
}

// ArchiveInfo contains information about an archived website
type ArchiveInfo struct {
	Title     string
	FilePath  string
	Timestamp time.Time
}

// archiveCtx holds per-archive state. The two caches (image and text) prevent
// the resource-inlining code from refetching the same URL many times within a
// single page — modern pages reference the same tracking pixel / sprite-svg /
// logo dozens of times, and the prior implementation issued a fresh HTTP GET
// for each reference. We also negative-cache failures (empty string) so a 404
// or DNS failure costs one round trip per URL, not one per reference.
//
// The client carries a per-request timeout so a slow origin server can't hang
// the entire save handler — Go's default http.Client has no timeout, which
// allowed the prior code to wedge indefinitely on unresponsive resources.
type archiveCtx struct {
	imgCache map[string]string // resolved URL -> data URI ("" = negative cache)
	txtCache map[string]string // resolved URL -> text content ("" = negative cache)
	client   *http.Client
}

func newArchiveCtx() *archiveCtx {
	return &archiveCtx{
		imgCache: make(map[string]string),
		txtCache: make(map[string]string),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// archiveWebsite downloads and archives a website with inlined resources
func (nm *NoteManager) archiveWebsite(websiteURL string) (*ArchiveInfo, error) {
	// Parse the URL
	parsedURL, err := url.Parse(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	ctx := newArchiveCtx()

	// Download the webpage
	resp, err := ctx.client.Get(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download webpage: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	// Read the HTML content
	htmlContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Extract title from HTML
	title := nm.extractTitle(string(htmlContent), parsedURL.Host)
	
	// Create filename in format expected by storage: YYYY_MM_DD_HHMMSS_title-domain.html
	timestamp := time.Now()
	filename := fmt.Sprintf("%s_%s-%s.html", 
		timestamp.Format("2006_01_02_150405"),
		nm.sanitizeFilename(title),
		nm.sanitizeFilename(parsedURL.Host))
	
	// Ensure sites directory exists
	sitesDir := filepath.Join(nm.storage.BasePath, "assets", "sites")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sites directory: %w", err)
	}
	
	// Process HTML to inline all external resources
	processedHTML := nm.inlineAllResources(ctx, string(htmlContent), websiteURL)
	
	// Save the archived file
	filePath := filepath.Join(sitesDir, filename)
	if err := os.WriteFile(filePath, []byte(processedHTML), 0644); err != nil {
		return nil, fmt.Errorf("failed to save archived file: %w", err)
	}
	
	// Create relative path for linking
	relativePath := filepath.Join("assets", "sites", filename)
	
	return &ArchiveInfo{
		Title:     title,
		FilePath:  relativePath,
		Timestamp: timestamp,
	}, nil
}

// extractTitle extracts the title from HTML content. Decodes HTML
// entities like `&#x27;` (apostrophe) and `&amp;` (ampersand) so the
// returned title is the actual text — otherwise titles like "It's FOSS"
// end up with literal `&#x27;` in the filename, which then breaks the
// delete-button HTML attribute (the browser re-decodes the entity
// mid-attribute and terminates the JS string early).
func (nm *NoteManager) extractTitle(htmlContent, host string) string {
	titleRe := regexp.MustCompile(`<title[^>]*>([^<]*)</title>`)
	matches := titleRe.FindStringSubmatch(htmlContent)

	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
		return html.UnescapeString(strings.TrimSpace(matches[1]))
	}

	return host
}

// sanitizeFilename removes invalid characters from filenames
func (nm *NoteManager) sanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := re.ReplaceAllString(filename, "_")
	
	// Limit length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	
	return strings.Trim(sanitized, "_")
}

// inlineAllResources performs comprehensive resource inlining
func (nm *NoteManager) inlineAllResources(ctx *archiveCtx, htmlContent, baseURL string) string {
	// Add archive header
	archiveHeader := fmt.Sprintf(`
<!-- ARCHIVED PAGE - Original URL: %s - Archived: %s -->
<div style="background: #fff3cd; border: 1px solid #ffeaa7; padding: 10px; margin: 10px 0; border-radius: 4px; font-family: Arial, sans-serif;">
	📄 <strong>Archived Page</strong> - Original: <a href="%s" target="_blank">%s</a> - Archived: %s
</div>
`, baseURL, time.Now().Format("2006-01-02 15:04:05"), baseURL, baseURL, time.Now().Format("2006-01-02 15:04:05"))

	// Parse base URL for resolving relative URLs
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("Warning: failed to parse base URL %s: %v", baseURL, err)
		return htmlContent
	}

	// Inline CSS stylesheets
	htmlContent = nm.inlineCSS(ctx, htmlContent, baseURLParsed)

	// Inline JavaScript files
	htmlContent = nm.inlineJavaScript(ctx, htmlContent, baseURLParsed)

	// Inline images as base64 data URIs
	htmlContent = nm.inlineImages(ctx, htmlContent, baseURLParsed)

	// Web fonts are inlined inside processCSS via url() rules — no separate pass needed.

	// Process inline CSS styles that may contain background images
	htmlContent = nm.inlineStyleAttributes(ctx, htmlContent, baseURLParsed)
	
	// Insert header after <body> tag
	bodyRe := regexp.MustCompile(`(<body[^>]*>)`)
	htmlContent = bodyRe.ReplaceAllString(htmlContent, `$1`+archiveHeader)
	
	return htmlContent
}

// inlineCSS inlines external CSS stylesheets
func (nm *NoteManager) inlineCSS(ctx *archiveCtx, htmlContent string, baseURL *url.URL) string {
	// Match <link> tags for stylesheets
	linkRe := regexp.MustCompile(`<link[^>]*href=["']([^"']+)["'][^>]*rel=["']stylesheet["'][^>]*>|<link[^>]*rel=["']stylesheet["'][^>]*href=["']([^"']+)["'][^>]*>`)

	return linkRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract href value
		hrefRe := regexp.MustCompile(`href=["']([^"']+)["']`)
		hrefMatch := hrefRe.FindStringSubmatch(match)
		if len(hrefMatch) < 2 {
			return match // Keep original if we can't extract href
		}

		cssURL := hrefMatch[1]

		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, cssURL)
		if resolvedURL == "" {
			return match
		}

		// Download CSS content
		cssContent := nm.downloadResource(ctx, resolvedURL)
		if cssContent == "" {
			return match
		}

		// Process CSS to inline any @import and url() references
		processedCSS := nm.processCSS(ctx, cssContent, resolvedURL)
		
		return fmt.Sprintf(`<style type="text/css">
/* Inlined from: %s */
%s
</style>`, resolvedURL, processedCSS)
	})
}

// inlineJavaScript inlines external JavaScript files
func (nm *NoteManager) inlineJavaScript(ctx *archiveCtx, htmlContent string, baseURL *url.URL) string {
	// Match <script> tags with src attributes
	scriptRe := regexp.MustCompile(`<script[^>]*src=["']([^"']+)["'][^>]*></script>`)

	return scriptRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract src value
		srcRe := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRe.FindStringSubmatch(match)
		if len(srcMatch) < 2 {
			return match
		}

		jsURL := srcMatch[1]

		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, jsURL)
		if resolvedURL == "" {
			return match
		}

		// Download JavaScript content
		jsContent := nm.downloadResource(ctx, resolvedURL)
		if jsContent == "" {
			return match
		}
		
		return fmt.Sprintf(`<script type="text/javascript">
/* Inlined from: %s */
%s
</script>`, resolvedURL, jsContent)
	})
}

// inlineImages inlines images as base64 data URIs
func (nm *NoteManager) inlineImages(ctx *archiveCtx, htmlContent string, baseURL *url.URL) string {
	// Match <img> tags
	imgRe := regexp.MustCompile(`<img[^>]*src=["']([^"']+)["'][^>]*>`)

	htmlContent = imgRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract src value
		srcRe := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRe.FindStringSubmatch(match)
		if len(srcMatch) < 2 {
			return match
		}

		imgURL := srcMatch[1]

		// Skip data URIs
		if strings.HasPrefix(imgURL, "data:") {
			return match
		}

		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, imgURL)
		if resolvedURL == "" {
			return match
		}

		// Download and encode image (cached per-archive)
		dataURI := nm.downloadAndEncodeImage(ctx, resolvedURL)
		if dataURI == "" {
			return match
		}

		// Replace src with data URI
		return srcRe.ReplaceAllString(match, fmt.Sprintf(`src="%s"`, dataURI))
	})

	// Also process JavaScript string references to images
	jsImgRe := regexp.MustCompile(`['"]([^'"]*\.(?:png|jpg|jpeg|gif|svg|webp))['"]`)
	htmlContent = jsImgRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		quote := match[0:1]
		imgURL := match[1 : len(match)-1]

		// Skip data URIs
		if strings.HasPrefix(imgURL, "data:") {
			return match
		}

		// Resolve relative URLs
		resolvedURL := nm.resolveURL(baseURL, imgURL)
		if resolvedURL == "" {
			return match
		}

		// Download and encode image (cached per-archive)
		dataURI := nm.downloadAndEncodeImage(ctx, resolvedURL)
		if dataURI == "" {
			return match
		}

		return fmt.Sprintf(`%s%s%s`, quote, dataURI, quote)
	})

	return htmlContent
}

// inlineStyleAttributes processes inline style attributes to inline background images
func (nm *NoteManager) inlineStyleAttributes(ctx *archiveCtx, htmlContent string, baseURL *url.URL) string {
	// Match style attributes
	styleRe := regexp.MustCompile(`style=["']([^"']*url\([^)]+\)[^"']*)["']`)

	return styleRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract the style content
		styleMatch := styleRe.FindStringSubmatch(match)
		if len(styleMatch) < 2 {
			return match
		}

		styleContent := styleMatch[1]
		quote := match[6:7] // Extract the quote character

		// Process URL references in the style
		processedStyle := nm.processInlineCSS(ctx, styleContent, baseURL)

		return fmt.Sprintf(`style=%s%s%s`, quote, processedStyle, quote)
	})
}

// processInlineCSS processes CSS content for inline styles
func (nm *NoteManager) processInlineCSS(ctx *archiveCtx, cssContent string, baseURL *url.URL) string {
	// Process url() references
	urlRe := regexp.MustCompile(`url\(["']?([^"')\s]+)["']?\)`)
	return urlRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		urlMatch := urlRe.FindStringSubmatch(match)
		if len(urlMatch) < 2 {
			return match
		}

		resourceURL := urlMatch[1]

		// Skip data URIs
		if strings.HasPrefix(resourceURL, "data:") {
			return match
		}

		resolvedURL := nm.resolveURL(baseURL, resourceURL)
		if resolvedURL == "" {
			return match
		}

		// Download and encode the resource (cached per-archive)
		dataURI := nm.downloadAndEncodeImage(ctx, resolvedURL)
		if dataURI != "" {
			return fmt.Sprintf(`url("%s")`, dataURI)
		}

		return match
	})
}

// resolveURL resolves a relative URL against a base URL. Returns "" to signal
// "skip this URL" — used both for unsupported schemes (mailto:, tel:, etc.)
// and for URLs that are obviously unrendered template placeholders. Modern
// pages often embed JavaScript with template strings like `<img src="${i}">`;
// without this guard we percent-encode the `${...}` and fire off doomed HTTP
// requests that 404 dozens of times per page.
func (nm *NoteManager) resolveURL(baseURL *url.URL, targetURL string) string {
	// Skip unrendered template placeholders. These show up inside inline JS
	// strings that the regex extractor can't tell apart from real attributes.
	if strings.Contains(targetURL, "${") || strings.Contains(targetURL, "{{") || strings.Contains(targetURL, "<%") {
		return ""
	}

	// Skip data URIs, mailto, tel, etc.
	if strings.Contains(targetURL, ":") && !strings.HasPrefix(targetURL, "http") && !strings.HasPrefix(targetURL, "//") {
		return ""
	}

	resolvedURL, err := baseURL.Parse(targetURL)
	if err != nil {
		log.Printf("Warning: failed to resolve URL %s against %s: %v", targetURL, baseURL, err)
		return ""
	}

	return resolvedURL.String()
}

// downloadResource downloads a text resource (CSS/JS) and caches the result.
// The cache is keyed by resolved URL and stores empty string for failed fetches
// so a single 404 doesn't get retried for every reference on the page.
func (nm *NoteManager) downloadResource(ctx *archiveCtx, resourceURL string) string {
	if v, ok := ctx.txtCache[resourceURL]; ok {
		return v
	}

	resp, err := ctx.client.Get(resourceURL)
	if err != nil {
		log.Printf("Warning: failed to download resource %s: %v", resourceURL, err)
		ctx.txtCache[resourceURL] = ""
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Warning: HTTP error %d downloading %s", resp.StatusCode, resourceURL)
		ctx.txtCache[resourceURL] = ""
		return ""
	}

	// Limit resource size to prevent memory issues (5MB max)
	const maxSize = 5 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("Warning: failed to read resource %s: %v", resourceURL, err)
		ctx.txtCache[resourceURL] = ""
		return ""
	}

	s := string(content)
	ctx.txtCache[resourceURL] = s
	return s
}

// downloadAndEncodeImage downloads an image and returns it as a base64 data URI.
// Results are cached per-archive to eliminate redundant fetches — modern pages
// reference the same sprite/tracking pixel dozens of times, and the previous
// implementation issued one HTTP GET per reference.
func (nm *NoteManager) downloadAndEncodeImage(ctx *archiveCtx, imageURL string) string {
	if v, ok := ctx.imgCache[imageURL]; ok {
		return v
	}

	resp, err := ctx.client.Get(imageURL)
	if err != nil {
		log.Printf("Warning: failed to download image %s: %v", imageURL, err)
		ctx.imgCache[imageURL] = ""
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Warning: HTTP error %d downloading image %s", resp.StatusCode, imageURL)
		ctx.imgCache[imageURL] = ""
		return ""
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to determine from URL extension
		ext := strings.ToLower(path.Ext(imageURL))
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Skip very large images (1MB max for images)
	const maxImageSize = 1 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxImageSize)

	imageData, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("Warning: failed to read image %s: %v", imageURL, err)
		ctx.imgCache[imageURL] = ""
		return ""
	}

	// Encode as base64 data URI
	encoded := base64.StdEncoding.EncodeToString(imageData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
	ctx.imgCache[imageURL] = dataURI
	return dataURI
}

// processCSS processes CSS content to inline @import and url() references
func (nm *NoteManager) processCSS(ctx *archiveCtx, cssContent, cssURL string) string {
	cssBaseURL, err := url.Parse(cssURL)
	if err != nil {
		return cssContent
	}

	// Process @import rules
	importRe := regexp.MustCompile(`@import\s+(?:url\()?["']([^"']+)["'](?:\))?[^;]*;`)
	cssContent = importRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		importMatch := importRe.FindStringSubmatch(match)
		if len(importMatch) < 2 {
			return match
		}

		importURL := nm.resolveURL(cssBaseURL, importMatch[1])
		if importURL == "" {
			return match
		}

		importedCSS := nm.downloadResource(ctx, importURL)
		if importedCSS == "" {
			return match
		}

		// Recursively process imported CSS
		return fmt.Sprintf("/* Imported from: %s */\n%s", importURL, nm.processCSS(ctx, importedCSS, importURL))
	})

	// Process url() references (fonts, background images, etc.)
	urlRe := regexp.MustCompile(`url\(["']?([^"')\s]+)["']?\)`)
	cssContent = urlRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		urlMatch := urlRe.FindStringSubmatch(match)
		if len(urlMatch) < 2 {
			return match
		}

		resourceURL := urlMatch[1]

		// Skip data URIs
		if strings.HasPrefix(resourceURL, "data:") {
			return match
		}

		resolvedURL := nm.resolveURL(cssBaseURL, resourceURL)
		if resolvedURL == "" {
			return match
		}

		// Determine if this is likely an image or font
		ext := strings.ToLower(path.Ext(resourceURL))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".svg" || ext == ".webp"
		isFont := ext == ".woff" || ext == ".woff2" || ext == ".ttf" || ext == ".otf" || ext == ".eot"

		if isImage || isFont {
			// Convert to data URI (cached per-archive)
			dataURI := nm.downloadAndEncodeImage(ctx, resolvedURL)
			if dataURI != "" {
				return fmt.Sprintf(`url("%s")`, dataURI)
			}
		}

		return match
	})

	return cssContent
}

// GetBasePath returns the base path for this note manager
func (nm *NoteManager) GetBasePath() string {
	return nm.storage.BasePath
}

// SaveFile saves an uploaded file and returns the path
func (nm *NoteManager) SaveFile(filename string, data []byte, contentType string) (string, bool, error) {
	isImage := strings.HasPrefix(contentType, "image/")
	path, err := nm.storage.SaveFile(filename, data, isImage)
	return path, isImage, err
}

// GetArchivedLinks returns information about archived websites
func (nm *NoteManager) GetArchivedLinks() (map[string]interface{}, error) {
	return nm.storage.ListArchivedSites()
}

// DeleteArchivedSite deletes an archived website file
func (nm *NoteManager) DeleteArchivedSite(filename string) error {
	if err := nm.storage.DeleteArchivedSite(filename); err != nil {
		return err
	}

	// Update notes.md to mark references as deleted
	nm.mu.Lock()
	defer nm.mu.Unlock()

	changesMade := false
	for _, note := range nm.notes {
		if strings.Contains(note.Content, filename) {
			lines := strings.Split(note.Content, "\n")
			for i, line := range lines {
				if strings.Contains(line, filename) {
					lines[i] = fmt.Sprintf("~~%s~~ _(archived link deleted)_", line)
					changesMade = true
				}
			}
			note.Content = strings.Join(lines, "\n")
		}
	}

	if changesMade {
		nm.needsSave = true
		return nm.save()
	}

	return nil
}

// HasChanges returns true if the notes have unsaved changes
func (nm *NoteManager) HasChanges() bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.needsSave
}

// GetAllTasks returns all tasks across all notes
func (nm *NoteManager) GetAllTasks() []models.Task {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	var allTasks []models.Task
	for _, note := range nm.notes {
		for _, task := range note.Tasks {
			allTasks = append(allTasks, *task)
		}
	}
	return allTasks
}