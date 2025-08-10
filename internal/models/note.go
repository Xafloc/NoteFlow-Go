package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const NoteSeparator = "\n<!-- note -->\n"

// Note represents a single note with content and tasks
type Note struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tasks     []*Task   `json:"tasks"`
}

// NewNote creates a new note with the given title and content
func NewNote(title, content string) *Note {
	note := &Note{
		Title:     title,
		Content:   content,
		Timestamp: time.Now(),
		Tasks:     make([]*Task, 0),
	}
	note.parseTasks()
	return note
}

// NewNoteFromText creates a note from raw markdown text
func NewNoteFromText(text string) (*Note, error) {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty note text")
	}

	header := strings.TrimPrefix(lines[0], "## ")
	
	// Parse timestamp and title from header
	timestampPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})(?: - (.*))?$`)
	matches := timestampPattern.FindStringSubmatch(header)
	
	var timestamp time.Time
	var title string
	
	if len(matches) >= 2 {
		var err error
		timestamp, err = time.Parse("2006-01-02 15:04:05", matches[1])
		if err != nil {
			timestamp = time.Now()
		}
		if len(matches) >= 3 {
			title = matches[2]
		}
	} else {
		timestamp = time.Now()
		title = header
	}

	content := ""
	if len(lines) > 1 {
		content = strings.TrimSpace(lines[1])
	}

	note := &Note{
		Title:     title,
		Content:   content,
		Timestamp: timestamp,
		Tasks:     make([]*Task, 0),
	}
	note.parseTasks()
	return note, nil
}

// parseTasks extracts tasks from the note content
func (n *Note) parseTasks() {
	n.Tasks = make([]*Task, 0)
	
	checkboxPattern := regexp.MustCompile(`\[([xX ])\]`)
	matches := checkboxPattern.FindAllStringSubmatchIndex(n.Content, -1)
	
	for i, match := range matches {
		checked := strings.ToLower(n.Content[match[2]:match[3]]) == "x"
		taskText := n.extractTaskText(match[0])
		
		task := &Task{
			Index:   i, // Will be updated by manager with global index
			Checked: checked,
			Text:    taskText,
		}
		n.Tasks = append(n.Tasks, task)
	}
}

// extractTaskText gets the full text of a task item
func (n *Note) extractTaskText(checkboxPos int) string {
	content := n.Content[checkboxPos:]
	lineEnd := strings.Index(content, "\n")
	if lineEnd == -1 {
		lineEnd = len(content)
	}
	return strings.TrimSpace(content[:lineEnd])
}

// Update updates the note's title and content, reparsing tasks
func (n *Note) Update(title, content string) {
	n.Title = title
	n.Content = content
	n.parseTasks()
}

// UpdateTask updates a specific task's completion status
func (n *Note) UpdateTask(taskIndex int, checked bool) bool {
	for _, task := range n.Tasks {
		if task.Index == taskIndex {
			oldMark := "[x]"
			newMark := "[ ]"
			if checked {
				oldMark = "[ ]"
				newMark = "[x]"
			}
			
			// Replace the checkbox in the original task text
			oldLine := task.Text
			newLine := strings.Replace(oldLine, oldMark, newMark, 1)
			
			// Update note content
			n.Content = strings.Replace(n.Content, oldLine, newLine, 1)
			
			// Update task
			task.Text = newLine
			task.Checked = checked
			return true
		}
	}
	return false
}

// GetUncheckedTasks returns all unchecked tasks in this note
func (n *Note) GetUncheckedTasks() []*TaskInfo {
	var tasks []*TaskInfo
	for _, task := range n.Tasks {
		if !task.Checked {
			// Clean the task text by removing checkbox markers
			cleanText := strings.TrimSpace(
				strings.Replace(
					strings.Replace(task.Text, "[x]", "", 1),
					"[ ]", "", 1,
				),
			)
			
			taskInfo := &TaskInfo{
				Index:     task.Index,
				Text:      cleanText,
				NoteTitle: n.Title,
				Timestamp: n.Timestamp.Format("2006-01-02 15:04:05"),
			}
			tasks = append(tasks, taskInfo)
		}
	}
	return tasks
}

// Render converts the note to markdown format for storage
func (n *Note) Render() string {
	timestampStr := n.Timestamp.Format("2006-01-02 15:04:05")
	titleStr := ""
	if n.Title != "" {
		titleStr = " - " + n.Title
	}
	
	return fmt.Sprintf("## %s%s\n\n%s\n", timestampStr, titleStr, n.Content)
}