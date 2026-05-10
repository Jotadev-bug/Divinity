package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
	"github.com/divinity/divinity/internal/store"
	"github.com/spf13/cobra"
)

func applyCmd() *cobra.Command {
	var agentName string
	var yes bool
	var allowDirty bool

	cmd := &cobra.Command{
		Use:          "apply [run-id]",
		Short:        "Apply an approved candidate diff to the current workspace",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := config.LoadAllowMissing(cfgPath)
			if err != nil {
				return err
			}

			runID := ""
			if len(args) == 1 {
				runID = args[0]
			}
			run, err := store.LoadRun(root, runID)
			if err != nil {
				return err
			}
			agent, err := selectRunAgent(run, agentName)
			if err != nil {
				return err
			}
			if agent.DiffPath == "" {
				return fmt.Errorf("agent %s has no diff path", agent.Name)
			}
			data, err := os.ReadFile(agent.DiffPath)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("agent %s produced an empty diff", agent.Name)
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "Ready to apply %s from run %s.\n", agent.Name, run.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "Diff: %s\n", agent.DiffPath)
				fmt.Fprintln(cmd.OutOrStdout(), "Rerun with --yes to approve and apply it.")
				return nil
			}
			if !allowDirty {
				status := execx.RunGit(context.Background(), root, "status", "--short")
				if status.ExitCode != 0 {
					return execx.RequireOK(status)
				}
				if strings.TrimSpace(status.Output) != "" {
					return fmt.Errorf("working tree is not clean; commit/stash changes or pass --allow-dirty")
				}
			}

			check := execx.RunGit(context.Background(), root, "apply", "--check", agent.DiffPath)
			if err := execx.RequireOK(check); err != nil {
				return err
			}
			applied := execx.RunGit(context.Background(), root, "apply", agent.DiffPath)
			if err := execx.RequireOK(applied); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Applied %s from run %s.\n", agent.Name, run.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "agent name; defaults to the recommended agent")
	cmd.Flags().BoolVar(&yes, "yes", false, "approve and apply the diff")
	cmd.Flags().BoolVar(&allowDirty, "allow-dirty", false, "apply even when the current working tree has changes")
	return cmd
}
