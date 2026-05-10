package cli

import (
	"github.com/divinity/divinity/internal/tui"
	"github.com/spf13/cobra"
)

func appCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "app",
		Short: "Open the interactive Divinity TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Launch(cfgPath)
		},
	}
}
