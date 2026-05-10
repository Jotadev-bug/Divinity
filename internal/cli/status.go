package cli

import (
	"fmt"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/store"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show recent Divinity runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := config.LoadAllowMissing(cfgPath)
			if err != nil {
				return err
			}

			runs, err := store.ListRuns(root)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No Divinity runs found.")
				return nil
			}

			for _, run := range runs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  winner=%s  agents=%d\n", run.ID, run.CreatedAt.Format("2006-01-02 15:04:05"), run.RecommendedAgent, len(run.Agents))
			}
			return nil
		},
	}
}
