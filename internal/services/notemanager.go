package services

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Xafloc/NoteFlow-Go/internal/models"
	"github.com/Xafloc/NoteFlow-Go/internal/storage"
	"github.com/go-shiori/obelisk"
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

// archiveWebsite downloads a webpage and produces a single self-contained HTML
// file with every CSS/JS/image/font inlined as a data URI. The heavy lifting
// (DOM parsing, relative URL resolution, recursive @import, concurrent fetch,
// per-resource caching) is delegated to go-shiori/obelisk — a maintained Go
// port of monolith. We previously rolled this by hand with regexes, which
// worked but mis-handled inline JavaScript template literals, sponsor-badge
// sprite maps, and refetched duplicate resources dozens of times per save.
func (nm *NoteManager) archiveWebsite(websiteURL string) (*ArchiveInfo, error) {
	parsedURL, err := url.Parse(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Tuning rationale:
	//   - DisableJS: archived pages are read-only snapshots. The original
	//     JS calls dead endpoints, so inlining megabytes of bundles just
	//     bloats the file with code that does nothing useful.
	//   - DisableEmbeds: iframes archived offline are blank anyway — skip
	//     them to save a fetch per embed.
	//   - DisableMedias is deliberately NOT set. Obelisk's "medias" includes
	//     <img>/<picture>/<source>, not just <video>/<audio> — turning it on
	//     strips real images. (Verified against process-html.go upstream.)
	//   - MaxRetries=0: a failed resource stays failed. Retries doubled
	//     wall-clock on flaky CDN endpoints with no quality gain.
	//   - RequestTimeout=30s: generous enough for slow CDNs (we saw a
	//     lobste.rs body read trip a 15s ceiling) but still bounded.
	//   - MaxConcurrentDownload=16: obelisk's default is 10. Pages with
	//     many small image references benefit from more parallelism.
	arc := &obelisk.Archiver{
		UserAgent:             "NoteFlow-Go archive",
		RequestTimeout:        30 * time.Second,
		MaxConcurrentDownload: 16,
		MaxRetries:            0,
		SkipResourceURLError:  true,
		DisableJS:             true,
		DisableEmbeds:         true,
		EnableLog:             false,
	}
	arc.Validate()

	// Overall archive deadline. Without this, a single hung resource
	// retry can wedge the save handler for minutes. 90s is enough for
	// real news/forum pages even with their long resource lists.
	archiveCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	body, _, err := arc.Archive(archiveCtx, obelisk.Request{URL: websiteURL})
	if err != nil {
		return nil, fmt.Errorf("failed to archive: %w", err)
	}

	title := nm.extractTitle(string(body), parsedURL.Host)

	timestamp := time.Now()
	filename := fmt.Sprintf("%s_%s-%s.html",
		timestamp.Format("2006_01_02_150405"),
		nm.sanitizeFilename(title),
		nm.sanitizeFilename(parsedURL.Host))

	sitesDir := filepath.Join(nm.storage.BasePath, "assets", "sites")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sites directory: %w", err)
	}

	// Prepend the standard "you're looking at the archived copy" banner just
	// inside <body>. Obelisk doesn't inject any marker of its own, so without
	// this an archived page is visually indistinguishable from the live one.
	withBanner := injectArchiveBanner(string(body), websiteURL, timestamp)

	filePath := filepath.Join(sitesDir, filename)
	if err := os.WriteFile(filePath, []byte(withBanner), 0644); err != nil {
		return nil, fmt.Errorf("failed to save archived file: %w", err)
	}

	return &ArchiveInfo{
		Title:     title,
		FilePath:  filepath.Join("assets", "sites", filename),
		Timestamp: timestamp,
	}, nil
}

// injectArchiveBanner prepends our archive-attribution box immediately after
// the <body> tag. If there is no <body> (only true for hand-rolled fragment
// HTML), the banner is prepended to the document instead.
func injectArchiveBanner(htmlBody, originalURL string, archivedAt time.Time) string {
	stamp := archivedAt.Format("2006-01-02 15:04:05")
	banner := fmt.Sprintf(`
<!-- ARCHIVED PAGE - Original URL: %s - Archived: %s -->
<div style="background: #fff3cd; border: 1px solid #ffeaa7; padding: 10px; margin: 10px 0; border-radius: 4px; font-family: Arial, sans-serif;">
	📄 <strong>Archived Page</strong> - Original: <a href="%s" target="_blank">%s</a> - Archived: %s
</div>
`, originalURL, stamp, originalURL, originalURL, stamp)

	bodyRe := regexp.MustCompile(`(?i)(<body[^>]*>)`)
	if bodyRe.MatchString(htmlBody) {
		return bodyRe.ReplaceAllString(htmlBody, "${1}"+banner)
	}
	return banner + htmlBody
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