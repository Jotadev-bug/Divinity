package model

import (
	"fmt"
	"strings"
	"time"
)

type Run struct {
	ID               string        `json:"id"`
	Task             string        `json:"task"`
	CreatedAt        time.Time     `json:"created_at"`
	CompletedAt      time.Time     `json:"completed_at"`
	RecommendedAgent string        `json:"recommended_agent"`
	Agents           []AgentResult `json:"agents"`
}

type AgentResult struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Status          string            `json:"status"`
	Score           int               `json:"score"`
	StartedAt       time.Time         `json:"started_at"`
	CompletedAt     time.Time         `json:"completed_at"`
	DurationMillis  int64             `json:"duration_millis"`
	WorktreePath    string            `json:"worktree_path"`
	Branch          string            `json:"branch"`
	LogPath         string            `json:"log_path"`
	DiffPath        string            `json:"diff_path"`
	Error           string            `json:"error,omitempty"`
	Validation      []ValidationRun   `json:"validation"`
	FilesChanged    int               `json:"files_changed"`
	LinesAdded      int               `json:"lines_added"`
	LinesDeleted    int               `json:"lines_deleted"`
	EvaluationNotes []string          `json:"evaluation_notes"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type ValidationRun struct {
	Name           string `json:"name"`
	Command        string `json:"command"`
	ExitCode       int    `json:"exit_code"`
	DurationMillis int64  `json:"duration_millis"`
	LogPath        string `json:"log_path"`
}

func (r Run) TextSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run %s\n", r.ID)
	fmt.Fprintf(&b, "Task: %s\n", r.Task)
	if r.RecommendedAgent != "" {
		fmt.Fprintf(&b, "Recommendation: %s\n", r.RecommendedAgent)
	}
	for _, agent := range r.Agents {
		fmt.Fprintf(&b, "\n[%s] %s score=%d files=%d +%d -%d\n", agent.Status, agent.Name, agent.Score, agent.FilesChanged, agent.LinesAdded, agent.LinesDeleted)
		if agent.Error != "" {
			fmt.Fprintf(&b, "Error: %s\n", agent.Error)
		}
		for _, check := range agent.Validation {
			fmt.Fprintf(&b, "  validation %s exit=%d log=%s\n", check.Name, check.ExitCode, check.LogPath)
		}
		if agent.DiffPath != "" {
			fmt.Fprintf(&b, "  diff: %s\n", agent.DiffPath)
		}
		if agent.LogPath != "" {
			fmt.Fprintf(&b, "  log: %s\n", agent.LogPath)
		}
		for _, note := range agent.EvaluationNotes {
			fmt.Fprintf(&b, "  - %s\n", note)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
