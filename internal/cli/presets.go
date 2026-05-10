package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var presetSnippets = map[string]string{
	"smoke": `agents:
  - name: smoke-agent
    type: shell
    command: powershell
    args:
      - -ExecutionPolicy
      - Bypass
      - -File
      - "{{project_root}}/scripts/divinity-smoke-agent.ps1"
      - "{{task}}"`,
	"gemini": `agents:
  - name: gemini-implementer
    type: shell
    command: gemini
    args:
      - --approval-mode=yolo
      - --prompt
      - |
        You are Gemini CLI running as a Divinity coding agent inside an isolated Git worktree.
        Make the requested code or file changes directly in the current working directory.
        Do not only describe the solution. Create, edit, and run commands as needed.

        Task:
        {{task}}
    env:
      GEMINI_SANDBOX: "false"`,
	"claude": `agents:
  - name: claude-implementer
    type: shell
    command: claude
    args:
      - "{{task}}"`,
	"opencode": `agents:
  - name: opencode-implementer
    type: shell
    command: opencode
    args:
      - run
      - "{{task}}"`,
	"aider": `agents:
  - name: aider-implementer
    type: shell
    command: aider
    args:
      - --message
      - "{{task}}"`,
	"groq": `agents:
  - name: groq-reviewer
    type: openai-compatible
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
    model: openai/gpt-oss-20b
    output_file: DIVINITY_GROQ_REVIEW.md`,
	"ollama": `agents:
  - name: ollama-reviewer
    type: openai-compatible
    base_url: http://127.0.0.1:11434/v1
    model: qwen2.5-coder:1.5b
    output_file: DIVINITY_OLLAMA_REVIEW.md`,
	"ollama-agentic": `agents:
  - name: ollama-agentic
    type: agentic
    base_url: http://127.0.0.1:11434/v1
    model: qwen2.5-coder:1.5b
    max_steps: 16
    allowed_commands:
      - git status --short
      - go test ./...`,
}

func presetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "presets [name]",
		Short: "Print agent configuration snippets",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				names := make([]string, 0, len(presetSnippets))
				for name := range presetSnippets {
					names = append(names, name)
				}
				sort.Strings(names)
				fmt.Fprintln(cmd.OutOrStdout(), "Available presets:")
				for _, name := range names {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nUse divinity presets <name> to print a snippet.")
				return nil
			}
			name := strings.ToLower(args[0])
			snippet, ok := presetSnippets[name]
			if !ok {
				return fmt.Errorf("unknown preset %q", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), snippet)
			return nil
		},
	}
}
