package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/divinity/divinity/internal/config"
	"github.com/divinity/divinity/internal/execx"
	"github.com/divinity/divinity/internal/model"
	"github.com/divinity/divinity/internal/orchestrator"
	"github.com/divinity/divinity/internal/store"
)

type appModel struct {
	cfg        config.Config
	root       string
	configPath string
	input      string
	width      int
	height     int
	messages   []timelineItem
	lastRun    string
	running    bool
	err        string
	startedAt  time.Time
	mode       string
	setup      setupState
	spin       int
}

type timelineItem struct {
	at    time.Time
	kind  string
	title string
	body  string
}

type runFinishedMsg struct {
	runID string
	text  string
	err   error
}

type tickMsg time.Time

type setupState struct {
	Provider string
	Step     int
	Values   map[string]string
}

var (
	appTextStyle    = lipgloss.NewStyle().Foreground(fg)
	appLogoStyle    = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	appStarStyle    = lipgloss.NewStyle().Foreground(violet)
	appHintStyle    = lipgloss.NewStyle().Foreground(muted)
	appPromptStyle  = lipgloss.NewStyle().Foreground(violet).Bold(true)
	appInputStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cyan).Padding(0, 1)
	appNoticeStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(yellow).Padding(0, 1).Foreground(yellow)
	appFooterStyle  = lipgloss.NewStyle().Foreground(muted)
	appSystemStyle  = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	appTaskStyle    = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	appResultStyle  = lipgloss.NewStyle().Foreground(green).Bold(true)
	appErrorStyle   = lipgloss.NewStyle().Foreground(red).Bold(true)
	appCommandStyle = lipgloss.NewStyle().Foreground(violet).Bold(true)
)

func Launch(cfgPath string) error {
	cfg, root, configPath, err := bootstrapApp(cfgPath)
	if err != nil {
		return err
	}
	mode := "main"
	messages := []timelineItem{
		{at: time.Now(), kind: "system", title: "Connect an agent", body: setupText()},
	}
	if needsSetup(cfg) {
		mode = "setup"
		messages = nil
	}

	model := appModel{
		cfg:        cfg,
		root:       root,
		configPath: configPath,
		width:      120,
		height:     36,
		startedAt:  time.Now(),
		mode:       mode,
		setup:      setupState{Values: map[string]string{}},
		messages:   messages,
	}

	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func bootstrapApp(cfgPath string) (config.Config, string, string, error) {
	cfg, root, err := config.LoadAllowMissing(cfgPath)
	if err != nil {
		return config.Config{}, "", "", err
	}
	if err := store.EnsureLayout(root); err != nil {
		return config.Config{}, "", "", err
	}

	path := cfgPath
	if path == "" {
		path = filepath.Join(root, config.DefaultFileName)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteDefault(path); err != nil {
			return config.Config{}, "", "", err
		}
	}

	return cfg, root, path, nil
}

func (m appModel) Init() tea.Cmd {
	return tickCmd()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case runFinishedMsg:
		m.running = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "error", title: "Run failed", body: msg.err.Error()})
			return m, nil
		}
		m.err = ""
		m.lastRun = msg.runID
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "result", title: "Run complete", body: msg.text})
	case tickMsg:
		m.spin++
		return m, tickCmd()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m.submit()
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.input += msg.String()
			}
		}
	}
	return m, nil
}

func (m appModel) submit() (tea.Model, tea.Cmd) {
	value := normalizeInput(m.input)
	m.input = ""
	if m.mode == "setup" {
		return m.handleSetupInput(value)
	}
	if value == "" {
		return m, nil
	}
	if m.running {
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "warn", title: "Run already active", body: "Wait for the current task to finish before starting another."})
		return m, nil
	}

	if strings.HasPrefix(value, "/") {
		return m.handleCommand(value)
	}
	return m.startRun(value)
}

func (m appModel) handleCommand(value string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(value)
	command := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(value, fields[0]))

	switch command {
	case "/q", "/quit", "/exit":
		return m, tea.Quit
	case "/help":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Commands", body: "/setup\n/run <task>\n/agents\n/status\n/review\n/diff\n/logs\n/apply yes\n/config\n/clear\n/help\n/quit"})
	case "/setup":
		m.mode = "setup"
		m.setup = setupState{Values: map[string]string{}}
	case "/clear":
		m.messages = nil
		m.lastRun = ""
		m.err = ""
	case "/run":
		if arg == "" {
			m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "warn", title: "Missing task", body: "Use /run <task>, or just type the task directly."})
			return m, nil
		}
		return m.startRun(arg)
	case "/agents":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Configured agents", body: m.agentsText()})
	case "/status":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Recent runs", body: m.statusText()})
	case "/review":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Latest review", body: m.reviewText()})
	case "/diff":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Latest diff", body: m.diffText()})
	case "/logs":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Latest logs", body: m.logsText()})
	case "/apply":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Apply", body: m.applyText(arg)})
	case "/config":
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "system", title: "Config", body: filepath.Join(m.root, config.DefaultFileName)})
	default:
		m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "warn", title: "Unknown command", body: value + "\nUse /help to see available commands."})
	}
	return m, nil
}

func (m appModel) startRun(task string) (tea.Model, tea.Cmd) {
	m.running = true
	m.err = ""
	m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "task", title: "Task", body: task})
	cfg := m.cfg
	root := m.root

	return m, func() tea.Msg {
		runner := orchestrator.New(root, cfg)
		run, err := runner.Run(context.Background(), orchestrator.RunRequest{Task: task})
		if err != nil {
			return runFinishedMsg{err: err}
		}
		return runFinishedMsg{runID: run.ID, text: compactRunText(run)}
	}
}

func (m appModel) View() string {
	width := clamp(m.width, 78, 160)
	inner := width - 4
	if inner < 72 {
		inner = 72
	}
	if m.mode == "setup" {
		return appTextStyle.Width(width).Padding(1, 2).Render(strings.Join([]string{
			"",
			m.banner(inner),
			"",
			m.setupPanel(inner),
			"",
			m.inputBox(inner),
			m.footer(inner),
		}, "\n"))
	}

	sections := []string{
		"",
		m.banner(inner),
		"",
		m.tips(inner),
		"",
		m.notice(inner),
		"",
		m.transcript(inner),
		"",
		m.inputBox(inner),
		m.footer(inner),
	}

	return appTextStyle.Width(width).Padding(1, 2).Render(strings.Join(sections, "\n"))
}

func (m appModel) banner(width int) string {
	logo := strings.Join([]string{
		" ____  _____     _____ _   _ ___ _______   __",
		"|  _ \\|_ _\\ \\   / /_ _| \\ | |_ _|_   _\\ \\ / /",
		"| | | || | \\ \\ / / | ||  \\| || |  | |  \\ V / ",
		"| |_| || |  \\ V /  | || |\\  || |  | |   | |  ",
		"|____/|___|  \\_/  |___|_| \\_|___| |_|   |_|  ",
	}, "\n")

	stars := appStarStyle.Render("* * *")
	lines := strings.Split(appLogoStyle.Render(logo), "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(line)
	}
	title := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(stars + "  " + appLogoStyle.Render("DIVINITY") + "  " + stars)
	return title + "\n" + strings.Join(lines, "\n")
}

func (m appModel) tips(width int) string {
	lines := []string{
		"Connect a model first:",
		"1. Run " + appCommandStyle.Render("divinity presets") + " to list supported setup snippets.",
		"2. Add a snippet to " + appCommandStyle.Render("divinity.yaml") + " and run " + appCommandStyle.Render("/agents") + ".",
		"3. Use " + appCommandStyle.Render("/setup") + " anytime for Claude/Gemini/OpenAI/Groq/Ollama examples.",
		"4. Then submit a task, inspect with " + appCommandStyle.Render("/diff") + ", and approve with " + appCommandStyle.Render("/apply yes") + ".",
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m appModel) notice(width int) string {
	text := "Divinity runs agents in isolated Git worktrees. Run inside a project repository with at least one commit."
	return appNoticeStyle.Width(width - 4).Render(text)
}

func (m appModel) transcript(width int) string {
	maxLines := max(5, m.height-22)
	items := m.messages
	if len(items) > maxLines {
		items = items[len(items)-maxLines:]
	}

	lines := make([]string, 0, len(items)*3)
	for _, item := range items {
		header := item.at.Format("15:04:05") + " " + itemStyle(item.kind).Render(item.title)
		lines = append(lines, header)
		for _, line := range strings.Split(item.body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lines = append(lines, "  "+truncate(line, width-4))
		}
	}
	if m.running {
		lines = append(lines, "")
		lines = append(lines, appTaskStyle.Render(spinnerFrame(m.spin)+" Generating..."))
		lines = append(lines, "  Running agents in isolated worktrees.")
		lines = append(lines, "  Live token streaming is coming next; completed output appears here automatically.")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m appModel) inputBox(width int) string {
	value := m.input
	if m.running {
		value = appHintStyle.Render("Running. Wait for this task to finish...")
	} else if value == "" {
		value = appHintStyle.Render("Type your message or @path/to/file")
	} else {
		value += appHintStyle.Render("|")
	}
	return appInputStyle.Width(width - 4).Render(appPromptStyle.Render("> ") + value)
}

func (m appModel) footer(width int) string {
	left := "~"
	status := "ready"
	if m.running {
		status = "running"
	}
	if m.err != "" {
		status = "error"
	}
	center := fmt.Sprintf("%s  %s  %d agents", status, "divinity v0.1.0", len(m.cfg.Agents))
	if m.lastRun != "" {
		center += "  latest " + m.lastRun
	}
	right := "Use /quit to exit"
	return spacedLine(width, appHintStyle.Render(left), appHintStyle.Render(center), appHintStyle.Render(right))
}

func (m appModel) agentsText() string {
	if len(m.cfg.Agents) == 0 {
		return "No agents configured."
	}
	lines := make([]string, 0, len(m.cfg.Agents))
	for _, agent := range m.cfg.Agents {
		kind := agent.Type
		if kind == "" {
			kind = "shell"
		}
		lines = append(lines, fmt.Sprintf("%s  %s", agent.Name, kind))
	}
	return strings.Join(lines, "\n")
}

func (m appModel) statusText() string {
	runs, err := store.ListRuns(m.root)
	if err != nil {
		return err.Error()
	}
	if len(runs) == 0 {
		return "No runs yet."
	}
	limit := 5
	if len(runs) < limit {
		limit = len(runs)
	}
	lines := make([]string, 0, limit)
	for _, run := range runs[:limit] {
		lines = append(lines, fmt.Sprintf("%s  winner=%s  agents=%d", run.ID, emptyDash(run.RecommendedAgent), len(run.Agents)))
	}
	return strings.Join(lines, "\n")
}

func (m appModel) reviewText() string {
	run, err := store.LoadRun(m.root, "")
	if err != nil {
		return "No latest run found."
	}
	return compactRunText(run)
}

func (m appModel) diffText() string {
	run, agent, err := m.latestAgent()
	if err != nil {
		return err.Error()
	}
	if agent.DiffPath == "" {
		return "No diff path for " + agent.Name + "."
	}
	data, err := os.ReadFile(agent.DiffPath)
	if err != nil {
		return err.Error()
	}
	if len(data) == 0 {
		return "No diff captured for " + agent.Name + " in " + run.ID + "."
	}
	return truncateBytesForTUI(string(data), 8000)
}

func (m appModel) logsText() string {
	_, agent, err := m.latestAgent()
	if err != nil {
		return err.Error()
	}
	if agent.LogPath == "" {
		return "No log path for " + agent.Name + "."
	}
	data, err := os.ReadFile(agent.LogPath)
	if err != nil {
		return err.Error()
	}
	if len(data) == 0 {
		return "No logs captured for " + agent.Name + "."
	}
	return truncateBytesForTUI(string(data), 8000)
}

func (m appModel) applyText(arg string) string {
	if strings.ToLower(strings.TrimSpace(arg)) != "yes" {
		return "This will apply the latest recommended diff to your current workspace.\nUse /apply yes to approve."
	}
	run, agent, err := m.latestAgent()
	if err != nil {
		return err.Error()
	}
	if agent.DiffPath == "" {
		return "No diff path for " + agent.Name + "."
	}
	data, err := os.ReadFile(agent.DiffPath)
	if err != nil {
		return err.Error()
	}
	if len(data) == 0 {
		return "Agent " + agent.Name + " produced an empty diff."
	}
	status := execx.RunGit(context.Background(), m.root, "status", "--short")
	if status.ExitCode != 0 {
		return execx.RequireOK(status).Error()
	}
	if strings.TrimSpace(status.Output) != "" {
		return "Working tree is not clean. Commit or stash changes before applying."
	}
	check := execx.RunGit(context.Background(), m.root, "apply", "--check", agent.DiffPath)
	if err := execx.RequireOK(check); err != nil {
		return err.Error()
	}
	applied := execx.RunGit(context.Background(), m.root, "apply", agent.DiffPath)
	if err := execx.RequireOK(applied); err != nil {
		return err.Error()
	}
	return "Applied " + agent.Name + " from " + run.ID + ". Run your tests, then commit manually if satisfied."
}

func (m appModel) latestAgent() (model.Run, model.AgentResult, error) {
	run, err := store.LoadRun(m.root, "")
	if err != nil {
		return model.Run{}, model.AgentResult{}, err
	}
	agentName := run.RecommendedAgent
	if agentName == "" && len(run.Agents) == 1 {
		return run, run.Agents[0], nil
	}
	for _, agent := range run.Agents {
		if agent.Name == agentName {
			return run, agent, nil
		}
	}
	return model.Run{}, model.AgentResult{}, fmt.Errorf("no recommended agent found for %s", run.ID)
}

func compactRunText(run model.Run) string {
	lines := []string{
		"Run: " + run.ID,
		"Task: " + run.Task,
		"Winner: " + emptyDash(run.RecommendedAgent),
	}
	for _, agent := range run.Agents {
		lines = append(lines, fmt.Sprintf("%s  %s  score=%d  files=%d +%d -%d", agent.Status, agent.Name, agent.Score, agent.FilesChanged, agent.LinesAdded, agent.LinesDeleted))
		if agent.Error != "" {
			lines = append(lines, "  error: "+agent.Error)
		}
		if agent.LogPath != "" {
			if logText := readShortFile(agent.LogPath, 2400); logText != "" {
				lines = append(lines, "")
				lines = append(lines, "Output from "+agent.Name+":")
				lines = append(lines, indentText(logText, "  "))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func itemStyle(kind string) lipgloss.Style {
	switch kind {
	case "error":
		return appErrorStyle
	case "warn", "task":
		return appTaskStyle
	case "result":
		return appResultStyle
	default:
		return appSystemStyle
	}
}

func spacedLine(width int, left, center, right string) string {
	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)
	gap := width - leftW - centerW - rightW
	if gap < 4 {
		return strings.Join([]string{left, center, right}, "  ")
	}
	return left + strings.Repeat(" ", gap/2) + center + strings.Repeat(" ", gap-gap/2) + right
}

func normalizeInput(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}

func truncateBytesForTUI(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes] + "\n...[truncated]..."
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func spinnerFrame(index int) string {
	frames := []string{"|", "/", "-", "\\"}
	return frames[index%len(frames)]
}

func readShortFile(path string, maxBytes int) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimSpace(truncateBytesForTUI(string(data), maxBytes))
}

func indentText(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func setupText() string {
	return strings.Join([]string{
		"Divinity runs configured agents in isolated Git worktrees.",
		"",
		"Fast setup commands:",
		"  divinity presets",
		"  divinity presets gemini",
		"  divinity presets opencode",
		"  divinity presets aider",
		"  divinity presets groq",
		"  divinity presets ollama",
		"",
		"CLI coding agents can edit files:",
		"  Claude Code: configure as type=shell once the claude command is installed.",
		"  Gemini CLI:  divinity presets gemini",
		"  OpenCode:    divinity presets opencode",
		"  Aider:       divinity presets aider",
		"",
		"API/local models are best as reviewers/planners:",
		"  Groq/OpenAI-compatible: set an API key env var, then use type=openai-compatible.",
		"  Ollama/LM Studio/vLLM: start the local server, then use its /v1 base URL.",
		"",
		"After editing divinity.yaml:",
		"  divinity doctor",
		"  divinity agents",
		"",
		"Inside this TUI:",
		"  /agents   show configured agents",
		"  /run      run a task",
		"  /diff     inspect result",
		"  /apply yes approve result",
	}, "\n")
}

func needsSetup(cfg config.Config) bool {
	if len(cfg.Agents) == 0 {
		return true
	}
	return len(cfg.Agents) == 1 && cfg.Agents[0].Name == "echo-plan" && cfg.Agents[0].Command == "divinity-example-agent"
}

func (m appModel) setupPanel(width int) string {
	body := setupMenu()
	if m.setup.Provider != "" {
		body = "Provider: " + setupProviderName(m.setup.Provider) + "\n\n" + m.setupPrompt()
	}
	return appNoticeStyle.Width(width - 4).Render(body)
}

func setupMenu() string {
	return strings.Join([]string{
		"Connect a provider. Type a number and press Enter:",
		"",
		"1. Claude Code",
		"2. Gemini CLI",
		"3. OpenCode",
		"4. Aider",
		"5. OpenAI-compatible API",
		"6. Groq",
		"7. Ollama",
		"8. Skip setup for now",
	}, "\n")
}

func (m appModel) handleSetupInput(value string) (tea.Model, tea.Cmd) {
	if m.setup.Provider == "" {
		provider := setupProviderFromChoice(value)
		if provider == "" {
			m.messages = append(m.messages, timelineItem{at: time.Now(), kind: "warn", title: "Setup", body: "Choose a number from 1 to 8."})
			return m, nil
		}
		if provider == "skip" {
			m.mode = "main"
			m.messages = []timelineItem{{at: time.Now(), kind: "system", title: "Setup skipped", body: "Use /setup anytime, or edit divinity.yaml manually."}}
			return m, nil
		}
		m.setup.Provider = provider
		m.setup.Step = 0
		m.setup.Values = map[string]string{}
		return m, nil
	}

	fields := setupFields(m.setup.Provider)
	if m.setup.Step < len(fields) {
		field := fields[m.setup.Step]
		if strings.TrimSpace(value) == "-" {
			value = ""
		}
		if value == "" {
			value = field.Default
		}
		m.setup.Values[field.Key] = value
		m.setup.Step++
	}
	if m.setup.Step >= len(fields) {
		agent := buildSetupAgent(m.setup.Provider, m.setup.Values)
		cfg := m.cfg
		cfg.Version = 1
		cfg.Agents = []config.AgentConfig{agent}
		if len(cfg.Validation) == 0 {
			cfg.Validation = []config.ValidationCheck{{Name: "git-status", Command: "git", Args: []string{"status", "--short"}}}
		}
		if cfg.Preferences.MaxParallelAgents <= 0 {
			cfg.Preferences.MaxParallelAgents = 4
		}
		if err := config.Write(m.configPath, cfg); err != nil {
			m.messages = []timelineItem{{at: time.Now(), kind: "error", title: "Setup failed", body: err.Error()}}
			m.mode = "main"
			return m, nil
		}
		m.cfg = cfg
		m.mode = "main"
		m.messages = []timelineItem{{at: time.Now(), kind: "system", title: "Agent connected", body: "Wrote " + m.configPath + "\n\n" + m.agentsText() + "\n\nRun /run <task> to test it."}}
		return m, nil
	}
	return m, nil
}

func (m appModel) setupPrompt() string {
	fields := setupFields(m.setup.Provider)
	if m.setup.Step >= len(fields) {
		return "Saving setup..."
	}
	field := fields[m.setup.Step]
	lines := []string{field.Label}
	if field.Default != "" {
		lines = append(lines, "Default: "+field.Default)
	}
	if field.Help != "" {
		lines = append(lines, field.Help)
	}
	lines = append(lines, "", "Type a value and press Enter. Press Enter with no value to use the default.")
	return strings.Join(lines, "\n")
}

type setupField struct {
	Key     string
	Label   string
	Default string
	Help    string
}

func setupFields(provider string) []setupField {
	switch provider {
	case "claude":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "claude-implementer"},
			{Key: "command", Label: "Claude command", Default: "claude", Help: "Use the command you normally run for Claude Code."},
		}
	case "gemini":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "gemini-implementer"},
			{Key: "command", Label: "Gemini command", Default: "gemini"},
		}
	case "opencode":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "opencode-implementer"},
			{Key: "command", Label: "OpenCode command", Default: "opencode"},
		}
	case "aider":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "aider-implementer"},
			{Key: "command", Label: "Aider command", Default: "aider"},
		}
	case "openai":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "openai-reviewer"},
			{Key: "base_url", Label: "OpenAI-compatible base URL", Default: "https://api.openai.com/v1"},
			{Key: "model", Label: "Model", Default: "gpt-4.1-mini"},
			{Key: "api_key_env", Label: "API key environment variable name", Default: "OPENAI_API_KEY", Help: "Store the secret in your shell, not in divinity.yaml."},
		}
	case "groq":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "groq-reviewer"},
			{Key: "model", Label: "Groq model", Default: "openai/gpt-oss-20b"},
			{Key: "api_key_env", Label: "API key environment variable name", Default: "GROQ_API_KEY"},
		}
	case "ollama":
		return []setupField{
			{Key: "name", Label: "Agent name", Default: "ollama-reviewer"},
			{Key: "model", Label: "Ollama model", Default: "qwen2.5-coder:1.5b"},
			{Key: "base_url", Label: "Ollama OpenAI-compatible base URL", Default: "http://127.0.0.1:11434/v1"},
		}
	}
	return nil
}

func buildSetupAgent(provider string, values map[string]string) config.AgentConfig {
	name := values["name"]
	switch provider {
	case "claude", "gemini", "opencode", "aider":
		agent := config.AgentConfig{Name: name, Type: "shell", Command: values["command"]}
		switch provider {
		case "gemini":
			agent.Args = []string{"--approval-mode=yolo", "--prompt", geminiPromptTemplate()}
			agent.Env = map[string]string{"GEMINI_SANDBOX": "false"}
		case "opencode":
			agent.Args = []string{"run", "{{task}}"}
		case "aider":
			agent.Args = []string{"--message", "{{task}}"}
		default:
			agent.Args = []string{"{{task}}"}
		}
		return agent
	case "openai":
		return config.AgentConfig{Name: name, Type: "openai-compatible", BaseURL: values["base_url"], Model: values["model"], APIKeyEnv: values["api_key_env"], OutputFile: "DIVINITY_OPENAI_REVIEW.md"}
	case "groq":
		return config.AgentConfig{Name: name, Type: "openai-compatible", BaseURL: "https://api.groq.com/openai/v1", Model: values["model"], APIKeyEnv: values["api_key_env"], OutputFile: "DIVINITY_GROQ_REVIEW.md"}
	case "ollama":
		return config.AgentConfig{Name: name, Type: "openai-compatible", BaseURL: values["base_url"], Model: values["model"], OutputFile: "DIVINITY_OLLAMA_REVIEW.md"}
	default:
		return config.AgentConfig{Name: "echo-plan", Type: "shell", Command: "divinity-example-agent", Args: []string{"{{task}}"}}
	}
}

func geminiPromptTemplate() string {
	return strings.Join([]string{
		"You are Gemini CLI running as a Divinity coding agent inside an isolated Git worktree.",
		"Make the requested code or file changes directly in the current working directory.",
		"Do not only describe the solution. Create, edit, and run commands as needed.",
		"",
		"Task:",
		"{{task}}",
	}, "\n")
}

func setupProviderFromChoice(choice string) string {
	switch strings.TrimSpace(choice) {
	case "1":
		return "claude"
	case "2":
		return "gemini"
	case "3":
		return "opencode"
	case "4":
		return "aider"
	case "5":
		return "openai"
	case "6":
		return "groq"
	case "7":
		return "ollama"
	case "8":
		return "skip"
	default:
		return ""
	}
}

func setupProviderName(provider string) string {
	switch provider {
	case "claude":
		return "Claude Code"
	case "gemini":
		return "Gemini CLI"
	case "opencode":
		return "OpenCode"
	case "aider":
		return "Aider"
	case "openai":
		return "OpenAI-compatible API"
	case "groq":
		return "Groq"
	case "ollama":
		return "Ollama"
	default:
		return provider
	}
}
