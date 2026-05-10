package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
)

type Request struct {
	Task         string
	Workspace   string
	RunID       string
	AgentName   string
	ProjectRoot string
}

type Runner interface {
	Run(context.Context, Request) execx.Result
}

func New(cfg config.AgentConfig) (Runner, error) {
	switch strings.ToLower(cfg.Type) {
	case "", "shell", "cli", "gemini":
		return Shell{Config: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported agent type %q for %s", cfg.Type, cfg.Name)
	}
}

type Shell struct {
	Config config.AgentConfig
}

func (s Shell) Run(ctx context.Context, req Request) execx.Result {
	args := make([]string, 0, len(s.Config.Args))
	for _, arg := range s.Config.Args {
		args = append(args, expand(arg, req))
	}

	env := []string{
		"DIVINITY_TASK=" + req.Task,
		"DIVINITY_WORKTREE=" + req.Workspace,
		"DIVINITY_RUN_ID=" + req.RunID,
		"DIVINITY_AGENT=" + req.AgentName,
		"DIVINITY_PROJECT_ROOT=" + req.ProjectRoot,
	}
	for key, value := range s.Config.Env {
		env = append(env, key+"="+expand(value, req))
	}

	command := expand(s.Config.Command, req)
	if strings.TrimSpace(command) == "" {
		return execx.Result{ExitCode: 1, Output: "agent command cannot be empty", Err: fmt.Errorf("agent command cannot be empty")}
	}
	if command == "divinity-example-agent" {
		return exampleAgent(ctx, req)
	}

	return execx.Run(ctx, req.Workspace, env, command, args...)
}

func expand(value string, req Request) string {
	replacer := strings.NewReplacer(
		"{{task}}", req.Task,
		"{{worktree}}", req.Workspace,
		"{{run_id}}", req.RunID,
		"{{agent}}", req.AgentName,
		"{{project_root}}", req.ProjectRoot,
	)
	return replacer.Replace(value)
}

func exampleAgent(ctx context.Context, req Request) execx.Result {
	filename := "DIVINITY_EXAMPLE.md"
	content := "# Divinity example agent output\n\nTask:\n\n" + req.Task + "\n"
	path := req.Workspace + string(os.PathSeparator) + filename
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return execx.Result{ExitCode: 1, Output: err.Error(), Err: err}
	}
	return execx.Run(ctx, req.Workspace, nil, "git", "status", "--short")
}
