package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/divinity/divinity/internal/execx"
	"github.com/divinity/divinity/internal/store"
)

type Manager struct {
	Root string
}

type Worktree struct {
	Path   string
	Branch string
}

func New(root string) Manager {
	return Manager{Root: root}
}

func (m Manager) EnsureGitRepo(ctx context.Context) error {
	result := execx.RunGit(ctx, m.Root, "rev-parse", "--is-inside-work-tree")
	if err := execx.RequireOK(result); err != nil {
		return fmt.Errorf("Divinity run requires a Git repository: %w", err)
	}
	return nil
}

func (m Manager) CreateWorktree(ctx context.Context, runID, agentName string) (Worktree, error) {
	if err := m.EnsureGitRepo(ctx); err != nil {
		return Worktree{}, err
	}

	safeAgent := slug(agentName)
	branch := fmt.Sprintf("divinity/%s/%s", runID, safeAgent)
	path := filepath.Join(m.Root, store.DirName, "worktrees", runID, safeAgent)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Worktree{}, err
	}

	result := execx.RunGit(ctx, m.Root, "worktree", "add", "-b", branch, path, "HEAD")
	if err := execx.RequireOK(result); err != nil {
		return Worktree{}, err
	}

	return Worktree{Path: path, Branch: branch}, nil
}

func (m Manager) RemoveWorktree(ctx context.Context, wt Worktree) error {
	if wt.Path == "" {
		return nil
	}
	result := execx.RunGit(ctx, m.Root, "worktree", "remove", "--force", wt.Path)
	if result.Err != nil {
		return execx.RequireOK(result)
	}
	return nil
}

func (m Manager) Diff(ctx context.Context, wt Worktree) (string, error) {
	result := execx.RunGit(ctx, wt.Path, "diff", "--binary", "HEAD")
	if result.Err != nil {
		return result.Output, execx.RequireOK(result)
	}
	return result.Output, nil
}

func (m Manager) DiffStat(ctx context.Context, wt Worktree) (filesChanged, linesAdded, linesDeleted int) {
	result := execx.RunGit(ctx, wt.Path, "diff", "--numstat", "HEAD")
	for _, line := range strings.Split(result.Output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		filesChanged++
		linesAdded += parseNumstat(fields[0])
		linesDeleted += parseNumstat(fields[1])
	}
	return filesChanged, linesAdded, linesDeleted
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-.")
	if value == "" {
		return "agent"
	}
	return value
}

func parseNumstat(value string) int {
	if value == "-" {
		return 0
	}
	var n int
	fmt.Sscanf(value, "%d", &n)
	return n
}
