package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/store"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Divinity in this repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}

			if err := store.EnsureLayout(root); err != nil {
				return err
			}

			path := filepath.Join(root, config.DefaultFileName)
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("%s already exists; pass --force to overwrite it", config.DefaultFileName)
			}

			if err := config.WriteDefault(path); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Divinity in %s\n", root)
			fmt.Fprintf(cmd.OutOrStdout(), "Edit %s to configure agents and validation checks.\n", config.DefaultFileName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	return cmd
}
