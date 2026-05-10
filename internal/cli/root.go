package cli

import (
	"github.com/spf13/cobra"
)

var cfgPath string

func Execute() error {
	root := &cobra.Command{
		Use:   "divinity",
		Short: "Multi-agent orchestration for coding agents",
		Long:  "Divinity coordinates coding agents in isolated Git worktrees, evaluates their results, and keeps the human in control.",
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "path to a Divinity config file")

	root.AddCommand(initCmd())
	root.AddCommand(runCmd(false))
	root.AddCommand(runCmd(true))
	root.AddCommand(statusCmd())
	root.AddCommand(reviewCmd())
	root.AddCommand(versionCmd())

	return root.Execute()
}
