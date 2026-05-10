package cli

import (
	"fmt"
	"os"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/model"
	"github.com/divinity/divinity/internal/store"
	"github.com/spf13/cobra"
)

func diffCmd() *cobra.Command {
	var agentName string

	cmd := &cobra.Command{
		Use:   "diff [run-id]",
		Short: "Print a saved candidate diff",
		Args:  cobra.MaximumNArgs(1),
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
				fmt.Fprintf(cmd.OutOrStdout(), "No diff captured for %s in %s.\n", agent.Name, run.ID)
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "agent name; defaults to the recommended agent")
	return cmd
}

func selectRunAgent(run model.Run, agentName string) (model.AgentResult, error) {
	if agentName == "" {
		agentName = run.RecommendedAgent
	}
	if agentName == "" && len(run.Agents) == 1 {
		return run.Agents[0], nil
	}
	for _, agent := range run.Agents {
		if agent.Name == agentName {
			return agent, nil
		}
	}
	if agentName == "" {
		return model.AgentResult{}, fmt.Errorf("run %s has no recommended agent", run.ID)
	}
	return model.AgentResult{}, fmt.Errorf("agent %q not found in run %s", agentName, run.ID)
}
