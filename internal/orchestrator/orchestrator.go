package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/divinity/divinity/internal/agent"
	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/eval"
	"github.com/divinity/divinity/internal/execx"
	"github.com/divinity/divinity/internal/model"
	"github.com/divinity/divinity/internal/store"
	"github.com/divinity/divinity/internal/workspace"
)

type RunRequest struct {
	Task          string
	AgentNames    []string
	KeepWorktrees bool
}

type Orchestrator struct {
	root string
	cfg  config.Config
}

func New(root string, cfg config.Config) Orchestrator {
	return Orchestrator{root: root, cfg: cfg}
}

func (o Orchestrator) Run(ctx context.Context, req RunRequest) (model.Run, error) {
	if strings.TrimSpace(req.Task) == "" {
		return model.Run{}, fmt.Errorf("task cannot be empty")
	}

	agents, err := o.cfg.SelectAgents(req.AgentNames)
	if err != nil {
		return model.Run{}, err
	}
	if len(agents) == 0 {
		return model.Run{}, fmt.Errorf("no agents selected")
	}

	run := model.Run{
		ID:        newRunID(),
		Task:      req.Task,
		CreatedAt: time.Now(),
	}

	if err := store.EnsureLayout(o.root); err != nil {
		return model.Run{}, err
	}
	if err := workspace.New(o.root).EnsureGitRepo(ctx); err != nil {
		return model.Run{}, err
	}

	limit := o.cfg.Preferences.MaxParallelAgents
	if limit <= 0 {
		limit = 4
	}
	sem := make(chan struct{}, limit)
	results := make([]model.AgentResult, len(agents))
	var wg sync.WaitGroup

	for i, agentCfg := range agents {
		wg.Add(1)
		go func(index int, cfg config.AgentConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = o.runAgent(ctx, run.ID, req, cfg)
		}(i, agentCfg)
	}

	wg.Wait()

	run.CompletedAt = time.Now()
	run.Agents = results
	run.RecommendedAgent = eval.Recommend(append([]model.AgentResult(nil), results...))

	if err := store.SaveRun(o.root, run); err != nil {
		return model.Run{}, err
	}
	return run, nil
}

func (o Orchestrator) runAgent(ctx context.Context, runID string, req RunRequest, cfg config.AgentConfig) model.AgentResult {
	start := time.Now()
	result := model.AgentResult{
		Name:      cfg.Name,
		Type:      cfg.Type,
		Status:    "failed",
		StartedAt: start,
		Metadata:  map[string]string{"description": cfg.Description},
	}

	wm := workspace.New(o.root)
	wt, err := wm.CreateWorktree(ctx, runID, cfg.Name)
	if err != nil {
		result.Error = err.Error()
		result.CompletedAt = time.Now()
		result.DurationMillis = result.CompletedAt.Sub(start).Milliseconds()
		eval.Score(&result)
		return result
	}
	result.WorktreePath = wt.Path
	result.Branch = wt.Branch

	runDir := store.RunDir(o.root, runID)
	agentDir := filepath.Join(runDir, sanitize(cfg.Name))
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		result.Error = err.Error()
		result.CompletedAt = time.Now()
		result.DurationMillis = result.CompletedAt.Sub(start).Milliseconds()
		eval.Score(&result)
		return result
	}

	runner, err := agent.New(cfg)
	if err != nil {
		result.Error = err.Error()
	} else {
		agentResult := runner.Run(ctx, agent.Request{
			Task:         req.Task,
			Workspace:   wt.Path,
			RunID:       runID,
			AgentName:   cfg.Name,
			ProjectRoot: o.root,
		})
		result.LogPath = filepath.Join(agentDir, "agent.log")
		_ = os.WriteFile(result.LogPath, []byte(agentResult.Output), 0644)
		if agentResult.ExitCode == 0 {
			result.Status = "succeeded"
		} else {
			result.Error = strings.TrimSpace(agentResult.Output)
			if result.Error == "" && agentResult.Err != nil {
				result.Error = agentResult.Err.Error()
			}
		}
	}

	result.Validation = o.runValidation(ctx, wt.Path, agentDir)

	diff, diffErr := wm.Diff(ctx, wt)
	if diffErr == nil {
		result.DiffPath = filepath.Join(agentDir, "diff.patch")
		_ = os.WriteFile(result.DiffPath, []byte(diff), 0644)
	}
	result.FilesChanged, result.LinesAdded, result.LinesDeleted = wm.DiffStat(ctx, wt)

	if !req.KeepWorktrees {
		_ = wm.RemoveWorktree(ctx, wt)
		result.WorktreePath = ""
	}

	result.CompletedAt = time.Now()
	result.DurationMillis = result.CompletedAt.Sub(start).Milliseconds()
	eval.Score(&result)
	return result
}

func (o Orchestrator) runValidation(ctx context.Context, dir, agentDir string) []model.ValidationRun {
	runs := make([]model.ValidationRun, 0, len(o.cfg.Validation))
	for _, check := range o.cfg.Validation {
		start := time.Now()
		res := execx.Run(ctx, dir, nil, check.Command, check.Args...)
		logPath := filepath.Join(agentDir, "validation-"+sanitize(check.Name)+".log")
		_ = os.WriteFile(logPath, []byte(res.Output), 0644)
		runs = append(runs, model.ValidationRun{
			Name:           check.Name,
			Command:        res.Command,
			ExitCode:       res.ExitCode,
			DurationMillis: time.Since(start).Milliseconds(),
			LogPath:        logPath,
		})
	}
	return runs
}

func newRunID() string {
	return "task-" + time.Now().Format("20060102-150405-000000000")
}

func sanitize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	value = replacer.Replace(value)
	if value == "" {
		return "check"
	}
	return value
}
