package cli

import (
	"github.com/divinity/divinity/internal/tui"
	"github.com/spf13/cobra"
)

var cfgPath string

func Execute() error {
	root := &cobra.Command{
		Use:   "divinity",
		Short: "Multi-agent orchestration for coding agents",
		Long:  "Divinity coordinates coding agents in isolated Git worktrees, evaluates their results, and keeps the human in control.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Launch(cfgPath)
		},
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "path to a Divinity config file")

	root.AddCommand(initCmd())
	root.AddCommand(appCmd())
	root.AddCommand(agentsCmd())
	root.AddCommand(doctorCmd())
	root.AddCommand(presetsCmd())
	root.AddCommand(runCmd(false))
	root.AddCommand(runCmd(true))
	root.AddCommand(diffCmd())
	root.AddCommand(applyCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(reviewCmd())
	root.AddCommand(versionCmd())

	return root.Execute()
}
