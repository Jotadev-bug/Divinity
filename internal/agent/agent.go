package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
)

type Request struct {
	Task        string
	Workspace   string
	RunID       string
	AgentName   string
	ProjectRoot string
}

type Runner interface {
	Run(context.Context, Request) execx.Result
}

func New(cfg config.AgentConfig) (Runner, error) {
	switch strings.ToLower(cfg.Type) {
	case "", "shell", "cli", "gemini":
		return Shell{Config: cfg}, nil
	case "openai-compatible", "openai", "api", "groq", "openrouter", "lmstudio", "vllm":
		return OpenAICompatible{Config: cfg}, nil
	case "agentic", "tool-loop", "ollama-agent":
		return ToolLoop{Config: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported agent type %q for %s", cfg.Type, cfg.Name)
	}
}

type Shell struct {
	Config config.AgentConfig
}

func (s Shell) Run(ctx context.Context, req Request) execx.Result {
	args := make([]string, 0, len(s.Config.Args))
	for _, arg := range s.Config.Args {
		args = append(args, expand(arg, req))
	}

	env := []string{
		"DIVINITY_TASK=" + req.Task,
		"DIVINITY_WORKTREE=" + req.Workspace,
		"DIVINITY_RUN_ID=" + req.RunID,
		"DIVINITY_AGENT=" + req.AgentName,
		"DIVINITY_PROJECT_ROOT=" + req.ProjectRoot,
	}
	for key, value := range s.Config.Env {
		env = append(env, key+"="+expand(value, req))
	}

	command := expand(s.Config.Command, req)
	if strings.TrimSpace(command) == "" {
		return execx.Result{ExitCode: 1, Output: "agent command cannot be empty", Err: fmt.Errorf("agent command cannot be empty")}
	}
	if command == "divinity-example-agent" {
		return exampleAgent(ctx, req)
	}

	return execx.Run(ctx, req.Workspace, env, command, args...)
}

func expand(value string, req Request) string {
	replacer := strings.NewReplacer(
		"{{task}}", req.Task,
		"{{worktree}}", req.Workspace,
		"{{run_id}}", req.RunID,
		"{{agent}}", req.AgentName,
		"{{project_root}}", req.ProjectRoot,
	)
	return replacer.Replace(value)
}

func exampleAgent(ctx context.Context, req Request) execx.Result {
	filename := "DIVINITY_EXAMPLE.md"
	content := "# Divinity example agent output\n\nTask:\n\n" + req.Task + "\n"
	path := req.Workspace + string(os.PathSeparator) + filename
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return execx.Result{ExitCode: 1, Output: err.Error(), Err: err}
	}
	return execx.Run(ctx, req.Workspace, nil, "git", "status", "--short")
}

type OpenAICompatible struct {
	Config config.AgentConfig
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (a OpenAICompatible) Run(ctx context.Context, req Request) execx.Result {
	start := time.Now()
	result := execx.Result{StartedAt: start}

	baseURL := strings.TrimRight(expand(a.Config.BaseURL, req), "/")
	if baseURL == "" {
		return failedResult(start, "api agent base_url cannot be empty")
	}
	if a.Config.Model == "" {
		return failedResult(start, "api agent model cannot be empty")
	}

	apiKeyEnv := a.Config.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = "OPENAI_API_KEY"
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" && !isLocalBaseURL(baseURL) {
		return failedResult(start, fmt.Sprintf("environment variable %s is required", apiKeyEnv))
	}

	system := strings.TrimSpace(a.Config.System)
	if system == "" {
		system = "You are a coding agent working inside an isolated Git worktree. Produce a concise implementation plan or review. If you cannot edit files directly, explain exactly what should change."
	}

	payload := chatRequest{
		Model: a.Config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: apiPrompt(req)},
		},
		Temperature: a.Config.Temperature,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return failedResult(start, err.Error())
	}

	endpoint := baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return failedResult(start, err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := http.Client{Timeout: 5 * time.Minute}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return failedResult(start, err.Error())
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return failedResult(start, err.Error())
	}

	result.Command = "POST " + endpoint
	result.Output = string(body)
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(start)

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		result.ExitCode = 1
		result.Err = fmt.Errorf("api request failed with status %d", httpResp.StatusCode)
		return result
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return failedResult(start, err.Error())
	}
	if parsed.Error != nil {
		return failedResult(start, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return failedResult(start, "api response did not include choices")
	}

	content := parsed.Choices[0].Message.Content
	outputFile := a.Config.OutputFile
	if outputFile == "" {
		outputFile = "DIVINITY_AGENT_OUTPUT.md"
	}
	outputPath := filepath.Join(req.Workspace, outputFile)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return failedResult(start, err.Error())
	}
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return failedResult(start, err.Error())
	}

	result.Output = content
	result.ExitCode = 0
	result.Err = nil
	return result
}

func apiPrompt(req Request) string {
	return fmt.Sprintf(`Task:
%s

Workspace:
%s

Project root:
%s

Run ID:
%s

Important:
- You are running as agent %q.
- Divinity will store your response in this worktree and compare it with other agents.
- Be specific, implementation-oriented, and concise.
`, req.Task, req.Workspace, req.ProjectRoot, req.RunID, req.AgentName)
}

func failedResult(start time.Time, message string) execx.Result {
	finished := time.Now()
	return execx.Result{
		ExitCode:   1,
		Output:     message,
		Err:        errors.New(message),
		StartedAt:  start,
		FinishedAt: finished,
		Duration:   finished.Sub(start),
	}
}

func isLocalBaseURL(baseURL string) bool {
	return strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, "[::1]")
}
