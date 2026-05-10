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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
)

const maxToolBytes = 80 * 1024

type ToolLoop struct {
	Config config.AgentConfig
}

type toolAction struct {
	Action  string `json:"action"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Command string `json:"command,omitempty"`
	Summary string `json:"summary,omitempty"`
}

func (a ToolLoop) Run(ctx context.Context, req Request) execx.Result {
	start := time.Now()
	maxSteps := a.Config.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 8
	}

	messages := []chatMessage{
		{Role: "system", Content: a.toolSystemPrompt()},
		{Role: "user", Content: fmt.Sprintf("Task:\n%s\n\nWorkspace: %s\n\nInitial files:\n%s", req.Task, req.Workspace, listFiles(req.Workspace, ".", 80))},
	}

	var transcript strings.Builder
	for step := 1; step <= maxSteps; step++ {
		content, err := a.complete(ctx, messages)
		if err != nil {
			return failedResult(start, err.Error())
		}
		fmt.Fprintf(&transcript, "\n## Step %d model\n\n%s\n", step, content)

		action, err := parseToolAction(content)
		if err != nil {
			observation := "Invalid tool JSON: " + err.Error() + "\nReturn only one JSON object matching the tool protocol."
			messages = append(messages, chatMessage{Role: "assistant", Content: content})
			messages = append(messages, chatMessage{Role: "user", Content: observation})
			fmt.Fprintf(&transcript, "\n## Step %d observation\n\n%s\n", step, observation)
			continue
		}

		observation, done := a.executeTool(ctx, req.Workspace, action)
		messages = append(messages, chatMessage{Role: "assistant", Content: content})
		messages = append(messages, chatMessage{Role: "user", Content: observation})
		fmt.Fprintf(&transcript, "\n## Step %d observation\n\n%s\n", step, observation)
		if done {
			return successResult(start, transcript.String())
		}
	}

	fmt.Fprintf(&transcript, "\nStopped after %d steps without finish.\n", maxSteps)
	return successResult(start, transcript.String())
}

func (a ToolLoop) toolSystemPrompt() string {
	if strings.TrimSpace(a.Config.System) != "" {
		return a.Config.System + "\n\n" + toolProtocol()
	}
	return "You are an agentic coding model. You can inspect and edit files only by requesting tools. Work carefully, make small changes, and finish with a concise summary.\n\n" + toolProtocol()
}

func toolProtocol() string {
	return `Return exactly one JSON object and no markdown.

Available actions:
{"action":"list_files","path":"."}
{"action":"read_file","path":"relative/path"}
{"action":"write_file","path":"relative/path","content":"complete file content"}
{"action":"run_command","command":"configured command exactly"}
{"action":"finish","summary":"what changed and how to validate"}

Rules:
- Use relative paths only.
- Do not use absolute paths.
- write_file replaces the whole file.
- If you need to create a file, call write_file first, then wait for the observation, then call finish.
- Never combine write_file and finish in the same JSON object.
- run_command only works for commands allowed in divinity.yaml.
- Call finish when the task is complete.`
}

func (a ToolLoop) complete(ctx context.Context, messages []chatMessage) (string, error) {
	baseURL := strings.TrimRight(a.Config.BaseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("agentic agent base_url cannot be empty")
	}
	if a.Config.Model == "" {
		return "", fmt.Errorf("agentic agent model cannot be empty")
	}

	apiKeyEnv := a.Config.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = "OPENAI_API_KEY"
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" && !isLocalBaseURL(baseURL) {
		return "", fmt.Errorf("environment variable %s is required", apiKeyEnv)
	}

	payload := chatRequest{
		Model:       a.Config.Model,
		Messages:    messages,
		Temperature: a.Config.Temperature,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := httpRequest(ctx, baseURL+"/chat/completions", data, apiKey)
	if err != nil {
		return "", err
	}
	return doChatRequest(httpReq)
}

func (a ToolLoop) executeTool(ctx context.Context, workspace string, action toolAction) (string, bool) {
	switch action.Action {
	case "list_files":
		path := action.Path
		if path == "" {
			path = "."
		}
		return "Files:\n" + listFiles(workspace, path, 200), false
	case "read_file":
		path, err := safePath(workspace, action.Path)
		if err != nil {
			return "read_file error: " + err.Error(), false
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "read_file error: " + err.Error(), false
		}
		return "File " + action.Path + ":\n" + truncateBytes(string(data), maxToolBytes), false
	case "write_file":
		path, err := safePath(workspace, action.Path)
		if err != nil {
			return "write_file error: " + err.Error(), false
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "write_file error: " + err.Error(), false
		}
		if err := os.WriteFile(path, []byte(action.Content), 0644); err != nil {
			return "write_file error: " + err.Error(), false
		}
		return "Wrote " + action.Path, false
	case "run_command":
		if !a.commandAllowed(action.Command) {
			return "run_command denied. Allowed commands: " + strings.Join(a.allowedCommands(), ", "), false
		}
		res := execx.Run(ctx, workspace, nil, "powershell", "-NoProfile", "-Command", action.Command)
		return fmt.Sprintf("Command: %s\nExit: %d\nOutput:\n%s", action.Command, res.ExitCode, truncateBytes(res.Output, maxToolBytes)), false
	case "finish":
		if strings.TrimSpace(action.Summary) == "" {
			action.Summary = "Finished."
		}
		return "Finished: " + action.Summary, true
	default:
		return "unknown action: " + action.Action, false
	}
}

func (a ToolLoop) allowedCommands() []string {
	if len(a.Config.AllowedCommands) > 0 {
		return a.Config.AllowedCommands
	}
	return []string{"git status --short", "go test ./..."}
}

func (a ToolLoop) commandAllowed(command string) bool {
	command = strings.TrimSpace(command)
	for _, allowed := range a.allowedCommands() {
		if command == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}

func parseToolAction(content string) (toolAction, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}
	if strings.Count(content, `"action"`) > 1 {
		if recovered, ok := recoverWriteFileAction(content); ok {
			return recovered, nil
		}
		return toolAction{}, fmt.Errorf("multiple action keys in one response; return exactly one tool call")
	}

	var action toolAction
	if err := json.Unmarshal([]byte(content), &action); err != nil {
		return toolAction{}, err
	}
	if strings.TrimSpace(action.Action) == "" {
		return toolAction{}, fmt.Errorf("missing action")
	}
	return action, nil
}

func recoverWriteFileAction(content string) (toolAction, bool) {
	if !strings.Contains(content, `"write_file"`) {
		return toolAction{}, false
	}
	path, ok := extractJSONStringField(content, "path")
	if !ok {
		return toolAction{}, false
	}
	fileContent, ok := extractJSONStringField(content, "content")
	if !ok {
		return toolAction{}, false
	}
	return toolAction{Action: "write_file", Path: path, Content: fileContent}, true
}

func extractJSONStringField(content, field string) (string, bool) {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(field) + `"\s*:\s*"((?:\\.|[^"\\])*)"`)
	match := re.FindStringSubmatch(content)
	if len(match) != 2 {
		return "", false
	}
	var value string
	if err := json.Unmarshal([]byte(`"`+match[1]+`"`), &value); err != nil {
		return "", false
	}
	return value, true
}

func listFiles(root, rel string, limit int) string {
	base, err := safePath(root, rel)
	if err != nil {
		return err.Error()
	}
	var files []string
	_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil || len(files) >= limit {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() && (name == ".git" || name == ".divinity" || name == ".gocache" || name == ".gopath") {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err == nil {
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	sort.Strings(files)
	if len(files) == 0 {
		return "No files found."
	}
	return strings.Join(files, "\n")
}

func safePath(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		rel = "."
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(cleanRoot, filepath.Clean(rel)))
	if err != nil {
		return "", err
	}
	if path != cleanRoot && !strings.HasPrefix(path, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return path, nil
}

func successResult(start time.Time, output string) execx.Result {
	finished := time.Now()
	return execx.Result{
		ExitCode:   0,
		Output:     output,
		StartedAt:  start,
		FinishedAt: finished,
		Duration:   finished.Sub(start),
	}
}

func truncateBytes(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes] + "\n...[truncated]..."
}

func httpRequest(ctx context.Context, endpoint string, data []byte, apiKey string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req, nil
}

func doChatRequest(req *http.Request) (string, error) {
	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, string(body))
	}
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("api response did not include choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
