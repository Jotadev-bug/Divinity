package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Command    string
	ExitCode   int
	Output     string
	Duration   time.Duration
	Err        error
	StartedAt  time.Time
	FinishedAt time.Time
}

func Run(ctx context.Context, dir string, env []string, command string, args ...string) Result {
	start := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	finished := time.Now()

	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	return Result{
		Command:    strings.Join(append([]string{command}, args...), " "),
		ExitCode:   exitCode,
		Output:     out.String(),
		Duration:   finished.Sub(start),
		Err:        err,
		StartedAt:  start,
		FinishedAt: finished,
	}
}

func RunGit(ctx context.Context, dir string, args ...string) Result {
	return Run(ctx, dir, nil, "git", args...)
}

func RequireOK(result Result) error {
	if result.Err == nil && result.ExitCode == 0 {
		return nil
	}
	if strings.TrimSpace(result.Output) == "" {
		return result.Err
	}
	return fmt.Errorf("%s failed with exit code %d: %s", result.Command, result.ExitCode, strings.TrimSpace(result.Output))
}
