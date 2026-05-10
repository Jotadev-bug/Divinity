package cli

import (
	"fmt"
	"strings"

	"github.com/divinity/divinity/internal/config"
	"github.com/spf13/cobra"
)

func agentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "List configured agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.LoadAllowMissing(cfgPath)
			if err != nil {
				return err
			}
			if len(cfg.Agents) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agents configured.")
				return nil
			}
			for _, agent := range cfg.Agents {
				kind := agent.Type
				if kind == "" {
					kind = "shell"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s", agent.Name, kind)
				switch strings.ToLower(kind) {
				case "shell", "cli", "gemini":
					fmt.Fprintf(cmd.OutOrStdout(), "  command=%s", empty(agent.Command))
				case "openai-compatible", "openai", "api", "groq", "openrouter", "lmstudio", "vllm", "agentic", "tool-loop", "ollama-agent":
					fmt.Fprintf(cmd.OutOrStdout(), "  model=%s base_url=%s", empty(agent.Model), empty(agent.BaseURL))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func empty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
