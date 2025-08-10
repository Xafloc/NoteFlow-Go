package models

// Task represents a checkbox task within a note
type Task struct {
	Index   int    `json:"index"`   // Unique global identifier
	Checked bool   `json:"checked"` // Completion state
	Text    string `json:"text"`    // Full task text including checkbox
}

// TaskInfo represents task information for API responses
type TaskInfo struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	NoteTitle string `json:"note_title"`
	Timestamp string `json:"timestamp"`
}

// TaskUpdate represents a task update request
type TaskUpdate struct {
	Checked bool `json:"checked"`
}