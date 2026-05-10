package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/orchestrator"
	"github.com/divinity/divinity/internal/store"
	"github.com/divinity/divinity/internal/tui"
	"github.com/spf13/cobra"
)

func runCmd(compare bool) *cobra.Command {
	var agents []string
	var noTUI bool
	var keepWorktrees bool

	use := "run [task]"
	short := "Run a task through one or more agents"
	if compare {
		use = "compare [task]"
		short = "Run the same task through multiple agents and compare results"
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if err := store.EnsureLayout(root); err != nil {
				return err
			}

			req := orchestrator.RunRequest{
				Task:          strings.TrimSpace(args[0]),
				AgentNames:    agents,
				KeepWorktrees: keepWorktrees,
			}

			runner := orchestrator.New(root, cfg)
			result, err := runner.Run(context.Background(), req)
			if err != nil {
				return err
			}

			if !noTUI {
				return tui.ShowSummary(result)
			}

			fmt.Fprintln(cmd.OutOrStdout(), result.TextSummary())
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&agents, "agent", nil, "agent name to run; may be passed multiple times")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "print a plain text summary")
	cmd.Flags().BoolVar(&keepWorktrees, "keep-worktrees", false, "keep generated worktrees after the run")
	return cmd
}
