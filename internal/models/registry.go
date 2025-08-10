package models

import (
	"time"
)

// FolderRegistry represents a registered NoteFlow folder in the database
type FolderRegistry struct {
	ID       int       `json:"id" db:"id"`
	Path     string    `json:"path" db:"path"`
	LastScan time.Time `json:"last_scan" db:"last_scan"`
	Active   bool      `json:"active" db:"active"`
}

// GlobalTask represents a task from any registered folder
type GlobalTask struct {
	ID          int       `json:"id" db:"id"`
	FolderID    int       `json:"folder_id" db:"folder_id"`
	FilePath    string    `json:"file_path" db:"file_path"`
	LineNumber  int       `json:"line_number" db:"line_number"`
	Content     string    `json:"content" db:"content"`
	Completed   bool      `json:"completed" db:"completed"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
	
	// Joined fields from folder
	FolderPath  string    `json:"folder_path,omitempty"`
}

// TaskSummary provides aggregated task information for a folder
type TaskSummary struct {
	FolderPath      string `json:"folder_path"`
	TotalTasks      int    `json:"total_tasks"`
	CompletedTasks  int    `json:"completed_tasks"`
	PendingTasks    int    `json:"pending_tasks"`
	LastUpdated     time.Time `json:"last_updated"`
}

// GlobalTasksResponse represents the response for global tasks endpoint
type GlobalTasksResponse struct {
	Tasks     []GlobalTask  `json:"tasks"`
	Summaries []TaskSummary `json:"summaries"`
	Total     int           `json:"total"`
}