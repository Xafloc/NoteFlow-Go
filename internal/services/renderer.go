package services

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// MarkdownRenderer handles markdown to HTML conversion
type MarkdownRenderer struct {
	md goldmark.Markdown
}

// NewMarkdownRenderer creates a new markdown renderer with extensions
func NewMarkdownRenderer() *MarkdownRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,        // GitHub Flavored Markdown
			extension.Table,      // Tables
			extension.Strikethrough, // Strikethrough text
			extension.TaskList,   // Task lists (checkboxes)
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),   // Convert line breaks to <br>
			html.WithXHTML(),       // Use XHTML-style tags
			html.WithUnsafe(),      // Allow raw HTML (needed for custom elements)
		),
	)

	return &MarkdownRenderer{md: md}
}

// RenderToHTML converts markdown content to HTML
func (r *MarkdownRenderer) RenderToHTML(content string) (string, error) {
	// Pre-process content for custom features
	content = r.preprocessContent(content)
	
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}

	html := buf.String()
	
	// Post-process HTML for custom features
	html = r.postprocessHTML(html)
	
	return html, nil
}

// preprocessContent handles custom markdown features before goldmark processing
func (r *MarkdownRenderer) preprocessContent(content string) string {
	// Handle math expressions (MathJax format)
	// Protect inline math $...$ from being processed as markdown
	content = r.protectMathExpressions(content)
	
	// Handle custom checkbox rendering with data attributes
	content = r.preprocessCheckboxes(content)
	
	return content
}

// protectMathExpressions protects math expressions from markdown processing
func (r *MarkdownRenderer) protectMathExpressions(content string) string {
	// Protect display math blocks $$...$$ 
	displayMathPattern := regexp.MustCompile(`\$\$([\s\S]*?)\$\$`)
	content = displayMathPattern.ReplaceAllStringFunc(content, func(match string) string {
		mathContent := strings.Trim(match, "$")
		return fmt.Sprintf(`<div class="math-display">$%s$</div>`, mathContent)
	})
	
	// Protect inline math $...$
	inlineMathPattern := regexp.MustCompile(`\$([^$\n]+)\$`)
	content = inlineMathPattern.ReplaceAllStringFunc(content, func(match string) string {
		mathContent := strings.Trim(match, "$")
		return fmt.Sprintf(`<span class="math-inline">$%s$</span>`, mathContent)
	})
	
	return content
}

// preprocessCheckboxes adds data attributes to checkboxes for JavaScript handling
func (r *MarkdownRenderer) preprocessCheckboxes(content string) string {
	lines := strings.Split(content, "\n")
	taskIndex := 0
	
	for i, line := range lines {
		// Match checkbox patterns
		checkboxPattern := regexp.MustCompile(`^(\s*-\s*)\[([xX ])\](.*)`)
		if matches := checkboxPattern.FindStringSubmatch(line); len(matches) == 4 {
			prefix := matches[1]
			status := matches[2]
			text := matches[3]
			
			checked := strings.ToLower(status) == "x"
			checkedAttr := ""
			if checked {
				checkedAttr = " checked"
			}
			
			// Replace with custom HTML that goldmark will pass through
			customCheckbox := fmt.Sprintf(`%s<input type="checkbox" data-checkbox-index="%d" id="task_%d"%s> %s`, 
				prefix, taskIndex, taskIndex, checkedAttr, strings.TrimSpace(text))
			
			lines[i] = customCheckbox
			taskIndex++
		}
	}
	
	return strings.Join(lines, "\n")
}

// postprocessHTML handles post-processing of the generated HTML
func (r *MarkdownRenderer) postprocessHTML(html string) string {
	// Enhance image handling
	html = r.enhanceImages(html)
	
	// Enhance blockquotes
	html = r.enhanceBlockquotes(html)
	
	// Fix any issues with custom checkboxes
	html = r.fixCheckboxes(html)
	
	return html
}

// enhanceImages wraps images in links for lightbox functionality
func (r *MarkdownRenderer) enhanceImages(html string) string {
	imgPattern := regexp.MustCompile(`<img([^>]*?)src=["']([^"']+)["']([^>]*?)>`)
	
	return imgPattern.ReplaceAllStringFunc(html, func(match string) string {
		// Extract src attribute
		srcPattern := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatches := srcPattern.FindStringSubmatch(match)
		if len(srcMatches) < 2 {
			return match
		}
		
		src := srcMatches[1]
		
		// Remove angle brackets if present (from drag-and-drop)
		src = strings.Trim(src, "<>")
		
		// Wrap in link for lightbox functionality
		if strings.HasPrefix(src, "http") || strings.Contains(src, "/assets/images/") {
			return fmt.Sprintf(
				`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
				src, match,
			)
		}
		
		return match
	})
}

// enhanceBlockquotes adds custom CSS classes to blockquotes
func (r *MarkdownRenderer) enhanceBlockquotes(html string) string {
	return strings.ReplaceAll(html, "<blockquote>", `<blockquote class="markdown-blockquote">`)
}

// fixCheckboxes ensures custom checkboxes are properly formatted
func (r *MarkdownRenderer) fixCheckboxes(html string) string {
	// Remove any <p> tags around standalone checkboxes
	checkboxPattern := regexp.MustCompile(`<p>(\s*<input[^>]*type="checkbox"[^>]*>[^<]*)</p>`)
	html = checkboxPattern.ReplaceAllString(html, `<div class="task-item">$1</div>`)
	
	return html
}

// RenderNoteHTML renders a complete note with proper styling and structure
func (r *MarkdownRenderer) RenderNoteHTML(content, timestamp, title string, noteIndex int) (string, error) {
	renderedContent, err := r.RenderToHTML(content)
	if err != nil {
		return "", err
	}

	noteHTML := fmt.Sprintf(`
<div class="section-container">
    <div id="note-%d" class="notes-item markdown-body" onclick="toggleNote(%d)">
        <div class="post-header">
            <span class="note-title" onclick="event.stopPropagation(); editNote(%d);">Posted: %s (click to edit)</span>
            <span class="delete-label" onclick="event.stopPropagation(); deleteNote(%d);" style="cursor: pointer;">(delete)</span>
            <div class="section-label-menu section-label-menu-expanded">
                <button onclick="event.stopPropagation(); toggleNote(%d)">collapse</button>
                <button onclick="event.stopPropagation(); collapseAll()">collapse all</button>
                <button onclick="event.stopPropagation(); expandAll()">expand all</button>
                <button onclick="event.stopPropagation(); collapseOthers(%d)">focus</button>
            </div>
            <div class="section-label-menu section-label-menu-collapsed" style="display: none;">
                <button onclick="event.stopPropagation(); toggleNote(%d)">expand</button>
                <button onclick="event.stopPropagation(); expandAll()">expand all</button>
            </div>
        </div>
        %s
    </div>
    <div class="section-label">
        <span>n</span>
        <span>o</span>
        <span>t</span>
        <span>e</span>
    </div>
</div>`, noteIndex, noteIndex, noteIndex, timestamp, noteIndex, noteIndex, noteIndex, noteIndex, renderedContent)

	return noteHTML, nil
}