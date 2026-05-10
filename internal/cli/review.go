package cli

import (
	"fmt"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/store"
	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review [run-id]",
		Short: "Display a saved run summary",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := config.LoadAllowMissing(cfgPath)
			if err != nil {
				return err
			}

			var runID string
			if len(args) == 1 {
				runID = args[0]
			}

			run, err := store.LoadRun(root, runID)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), run.TextSummary())
			return nil
		},
	}
}
