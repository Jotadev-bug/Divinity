package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name   string
	OK     bool
	Detail string
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "doctor",
		Short:        "Check whether Divinity is ready to run",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := config.LoadAllowMissing(cfgPath)
			if err != nil {
				return err
			}

			checks := []doctorCheck{
				checkCommand("go"),
				checkCommand("git"),
				checkGitRepo(root),
				checkConfig(cfg),
			}
			for _, agent := range cfg.Agents {
				checks = append(checks, checkAgent(agent)...)
			}

			failures := 0
			for _, check := range checks {
				marker := "ok"
				if !check.OK {
					marker = "fail"
					failures++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s", marker, check.Name)
				if check.Detail != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " - %s", check.Detail)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if failures > 0 {
				return fmt.Errorf("%d doctor check(s) failed", failures)
			}
			return nil
		},
	}
}

func checkCommand(name string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorCheck{Name: name + " on PATH", OK: false, Detail: "not found"}
	}
	return doctorCheck{Name: name + " on PATH", OK: true, Detail: path}
}

func checkGitRepo(root string) doctorCheck {
	result := execx.RunGit(context.Background(), root, "rev-parse", "--is-inside-work-tree")
	if result.ExitCode != 0 {
		return doctorCheck{Name: "Git repository", OK: false, Detail: strings.TrimSpace(result.Output)}
	}
	head := execx.RunGit(context.Background(), root, "rev-parse", "--verify", "HEAD")
	if head.ExitCode != 0 {
		return doctorCheck{Name: "Git repository", OK: false, Detail: "repo has no commits; create an initial commit before running agents"}
	}
	return doctorCheck{Name: "Git repository", OK: true, Detail: root}
}

func checkConfig(cfg config.Config) doctorCheck {
	if len(cfg.Agents) == 0 {
		return doctorCheck{Name: "Configured agents", OK: false, Detail: "no agents configured"}
	}
	return doctorCheck{Name: "Configured agents", OK: true, Detail: fmt.Sprintf("%d agent(s)", len(cfg.Agents))}
}

func checkAgent(agent config.AgentConfig) []doctorCheck {
	name := "Agent " + agent.Name
	kind := strings.ToLower(agent.Type)
	if kind == "" {
		kind = "shell"
	}
	switch kind {
	case "shell", "cli", "gemini":
		if strings.TrimSpace(agent.Command) == "" {
			return []doctorCheck{{Name: name, OK: false, Detail: "missing command"}}
		}
		if agent.Command == "divinity-example-agent" {
			return []doctorCheck{{Name: name, OK: true, Detail: "built-in example agent"}}
		}
		_, err := exec.LookPath(agent.Command)
		return []doctorCheck{{Name: name, OK: err == nil, Detail: shellAgentDetail(agent.Command, err)}}
	case "openai-compatible", "openai", "api", "groq", "openrouter", "lmstudio", "vllm", "agentic", "tool-loop", "ollama-agent":
		var checks []doctorCheck
		checks = append(checks, doctorCheck{Name: name + " model", OK: agent.Model != "", Detail: empty(agent.Model)})
		checks = append(checks, doctorCheck{Name: name + " base URL", OK: agent.BaseURL != "", Detail: empty(agent.BaseURL)})
		if agent.APIKeyEnv != "" && !isLocalURL(agent.BaseURL) {
			_, ok := os.LookupEnv(agent.APIKeyEnv)
			checks = append(checks, doctorCheck{Name: name + " API key env", OK: ok, Detail: agent.APIKeyEnv})
		}
		if isLocalURL(agent.BaseURL) {
			checks = append(checks, checkEndpoint(name+" endpoint", agent.BaseURL))
		}
		return checks
	default:
		return []doctorCheck{{Name: name, OK: false, Detail: "unsupported type " + agent.Type}}
	}
}

func shellAgentDetail(command string, err error) string {
	if err != nil {
		return command + " not found on PATH"
	}
	return command
}

func isLocalURL(value string) bool {
	return strings.Contains(value, "localhost") || strings.Contains(value, "127.0.0.1") || strings.Contains(value, "[::1]")
}

func checkEndpoint(name, baseURL string) doctorCheck {
	client := http.Client{Timeout: 2 * time.Second}
	url := strings.TrimRight(baseURL, "/") + "/models"
	resp, err := client.Get(url)
	if err != nil {
		return doctorCheck{Name: name, OK: false, Detail: err.Error()}
	}
	defer resp.Body.Close()
	return doctorCheck{Name: name, OK: resp.StatusCode >= 200 && resp.StatusCode < 500, Detail: fmt.Sprintf("GET %s -> %d", url, resp.StatusCode)}
}
