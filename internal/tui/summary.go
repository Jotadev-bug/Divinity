package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/divinity/divinity/internal/model"
)

type summaryModel struct {
	run    model.Run
	width  int
	height int
}

const minPanelWidth = 36

var (
	bg        = lipgloss.Color("235")
	fg        = lipgloss.Color("252")
	muted     = lipgloss.Color("244")
	dim       = lipgloss.Color("240")
	cyan      = lipgloss.Color("81")
	green     = lipgloss.Color("114")
	yellow    = lipgloss.Color("222")
	red       = lipgloss.Color("210")
	violet    = lipgloss.Color("183")
	border    = lipgloss.Color("238")
	borderHot = lipgloss.Color("74")

	appStyle      = lipgloss.NewStyle().Padding(1, 2).Background(bg).Foreground(fg)
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	taglineStyle  = lipgloss.NewStyle().Foreground(fg)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	okStyle       = lipgloss.NewStyle().Bold(true).Foreground(green)
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	failStyle     = lipgloss.NewStyle().Bold(true).Foreground(red)
	accentStyle   = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	commandStyle  = lipgloss.NewStyle().Bold(true).Foreground(violet)
	inputStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
	footerStyle   = lipgloss.NewStyle().Foreground(muted)
	sectionBorder = lipgloss.RoundedBorder()
)

func ShowSummary(run model.Run) error {
	_, err := tea.NewProgram(summaryModel{run: run, width: 120, height: 36}).Run()
	return err
}

func (m summaryModel) Init() tea.Cmd {
	return nil
}

func (m summaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m summaryModel) View() string {
	width := clamp(m.width, 88, 180)
	contentWidth := width - 4
	if contentWidth < 80 {
		contentWidth = 80
	}

	body := []string{
		m.header(contentWidth),
		m.rule(contentWidth),
	}

	if contentWidth >= 118 {
		rightWidth := 31
		leftWidth := contentWidth - rightWidth - 2
		topLeft := lipgloss.JoinHorizontal(lipgloss.Top,
			m.sessionPanel((leftWidth-2)/2),
			"  ",
			m.statusPanel(leftWidth-(leftWidth-2)/2-2),
		)
		left := lipgloss.JoinVertical(lipgloss.Left,
			topLeft,
			m.agentsPanel(leftWidth),
			m.logsPanel(leftWidth, 8),
			m.commandBar(leftWidth),
		)
		right := lipgloss.JoinVertical(lipgloss.Left,
			m.shortcutsPanel(rightWidth),
			m.checksPanel(rightWidth),
			m.usagePanel(rightWidth),
		)
		body = append(body, lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	} else {
		body = append(body,
			m.sessionPanel(contentWidth),
			m.statusPanel(contentWidth),
			m.agentsPanel(contentWidth),
			m.checksPanel(contentWidth),
			m.logsPanel(contentWidth, 7),
			m.commandBar(contentWidth),
		)
	}

	body = append(body, m.footer(contentWidth))
	return appStyle.Width(width).Render(strings.Join(body, "\n"))
}

func (m summaryModel) header(width int) string {
	logo := strings.Join([]string{
		" ____  _ _   _ ___ _   _ ___ _______   __",
		"|  _ \\(_) | | |_ _| \\ | |_ _|_   _\\ \\ / /",
		"| | | | | | | || ||  \\| || |  | |  \\ V / ",
		"| |_| | | |_| || || |\\  || |  | |   | |  ",
		"|____/|_|\\___/|___|_| \\_|___| |_|   |_|  ",
	}, "\n")
	tagline := strings.Join([]string{
		taglineStyle.Render("Orchestrate."),
		taglineStyle.Render("Execute."),
		taglineStyle.Render("Elevate."),
		"",
		mutedStyle.Render("Type ") + commandStyle.Render("/help") + mutedStyle.Render(" for available commands"),
	}, "\n")

	joined := lipgloss.JoinHorizontal(lipgloss.Top, logoStyle.Render(logo), "   ", tagline)
	return lipgloss.NewStyle().Width(width).Render(joined)
}

func (m summaryModel) sessionPanel(width int) string {
	rows := []kv{
		{"ID", m.run.ID},
		{"Task", m.run.Task},
		{"Repo", currentDir()},
		{"Winner", emptyDash(m.run.RecommendedAgent)},
		{"Started", m.run.CreatedAt.Format("15:04:05") + " (" + formatDuration(m.run.CompletedAt.Sub(m.run.CreatedAt)) + ")"},
	}
	return panel("SESSION", width, rowsBlock(rows, width-4), green)
}

func (m summaryModel) statusPanel(width int) string {
	passed, failed, waiting := m.checkCounts()
	status := okStyle.Render("COMPLETE")
	if failed > 0 {
		status = failStyle.Render("NEEDS REVIEW")
	}
	rows := []string{
		fmt.Sprintf("%-10s %s", "Overall:", status),
		fmt.Sprintf("%-10s %s 100%%", "Progress:", progressBar(100, 30)),
		fmt.Sprintf("%-10s %d completed", "Agents:", len(m.run.Agents)),
		fmt.Sprintf("%-10s %s, %s, %s", "Checks:", okStyle.Render(fmt.Sprintf("%d passed", passed)), failStyle.Render(fmt.Sprintf("%d failed", failed)), mutedStyle.Render(fmt.Sprintf("%d waiting", waiting))),
	}
	return panel("STATUS", width, strings.Join(rows, "\n"), green)
}

func (m summaryModel) agentsPanel(width int) string {
	nameW := 18
	statusW := 13
	scoreW := 9
	diffW := 14
	pathW := max(12, width-nameW-statusW-scoreW-diffW-14)

	lines := []string{
		fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s", nameW, "NAME", statusW, "STATUS", scoreW, "SCORE", diffW, "DIFF", pathW, "WORKTREE"),
		dimStyle.Render("  " + strings.Repeat("-", max(10, width-8))),
	}
	for _, agent := range m.run.Agents {
		status := colorStatus(agent.Status)
		diff := fmt.Sprintf("+%d -%d", agent.LinesAdded, agent.LinesDeleted)
		worktree := agent.WorktreePath
		if worktree == "" {
			worktree = "-"
		}
		marker := okStyle.Render("*")
		if agent.Status != "succeeded" {
			marker = failStyle.Render("*")
		}
		lines = append(lines, fmt.Sprintf("%s %-*s %-*s %-*d %-*s %-*s",
			marker,
			nameW,
			truncate(agent.Name, nameW),
			statusW,
			status,
			scoreW,
			agent.Score,
			diffW,
			diff,
			pathW,
			truncate(cleanPath(worktree), pathW),
		))
	}
	return panel("AGENTS", width, strings.Join(lines, "\n"), cyan)
}

func (m summaryModel) checksPanel(width int) string {
	checks := m.flattenChecks()
	if len(checks) == 0 {
		return panel("CHECKS", width, mutedStyle.Render("No validation checks configured."), cyan)
	}

	lines := make([]string, 0, len(checks))
	for _, check := range checks {
		label := truncate(check.Name, max(8, width-18))
		state := okStyle.Render("Passed")
		if check.ExitCode != 0 {
			state = failStyle.Render("Failed")
		}
		lines = append(lines, fmt.Sprintf("%-*s %s", max(10, width-18), label, state))
	}
	return panel("CHECKS", width, strings.Join(lines, "\n"), cyan)
}

func (m summaryModel) shortcutsPanel(width int) string {
	rows := []string{
		commandStyle.Render("/run") + "      Run a new task",
		commandStyle.Render("/agents") + "   Manage agents",
		commandStyle.Render("/status") + "   Show full status",
		commandStyle.Render("/logs") + "     Show logs",
		commandStyle.Render("/review") + "   Review results",
		commandStyle.Render("/config") + "   Edit config",
		commandStyle.Render("/quit") + "     Exit Divinity",
	}
	return panel("SHORTCUTS", width, strings.Join(rows, "\n"), cyan)
}

func (m summaryModel) usagePanel(width int) string {
	duration := formatDuration(m.run.CompletedAt.Sub(m.run.CreatedAt))
	if m.run.CompletedAt.IsZero() {
		duration = formatDuration(time.Since(m.run.CreatedAt))
	}
	rows := []kv{
		{"Time", duration},
		{"Agents", fmt.Sprintf("%d", len(m.run.Agents))},
		{"Results", fmt.Sprintf("%d files changed", totalFiles(m.run.Agents))},
		{"Mode", "local worktrees"},
	}
	return panel("USAGE", width, rowsBlock(rows, width-4), cyan)
}

func (m summaryModel) logsPanel(width, maxLines int) string {
	agent := m.recommendedAgent()
	title := "LOGS"
	if agent.Name != "" {
		title = "LOGS (" + agent.Name + ")"
	}

	lines := readLogPreview(agent.LogPath, maxLines)
	if len(lines) == 0 {
		lines = []string{mutedStyle.Render("No log output captured for this run.")}
	}
	for i, line := range lines {
		lines[i] = truncate(line, width-6)
	}
	return panel(title, width, strings.Join(lines, "\n"), cyan)
}

func (m summaryModel) commandBar(width int) string {
	text := dimStyle.Render("> ") + mutedStyle.Render("Type your message or command...")
	return inputStyle.Width(width - 2).Render(text)
}

func (m summaryModel) footer(width int) string {
	left := failStyle.Render("review required")
	if m.failedAgents() == 0 {
		left = okStyle.Render("ready")
	}
	center := mutedStyle.Render("divinity v0.1.0")
	right := okStyle.Render("* Connected") + mutedStyle.Render("  ") + commandStyle.Render("Enter/q to close")
	gap := width - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	if gap < 4 {
		return footerStyle.Width(width).Render(strings.Join([]string{left, center, right}, "  "))
	}
	return footerStyle.Width(width).Render(left + strings.Repeat(" ", gap/2) + center + strings.Repeat(" ", gap-gap/2) + right)
}

func panel(title string, width int, body string, titleColor lipgloss.Color) string {
	if width < minPanelWidth {
		width = minPanelWidth
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	return lipgloss.NewStyle().
		Width(width).
		Border(sectionBorder).
		BorderForeground(border).
		Padding(1, 2).
		Render(titleStyle.Render("* "+title) + "\n" + body)
}

func rowsBlock(rows []kv, width int) string {
	lines := make([]string, 0, len(rows))
	valueW := max(8, width-12)
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s %s", row.key+":", truncate(row.value, valueW)))
	}
	return strings.Join(lines, "\n")
}

func (m summaryModel) rule(width int) string {
	return dimStyle.Render(strings.Repeat("-", width))
}

func (m summaryModel) checkCounts() (passed, failed, waiting int) {
	for _, agent := range m.run.Agents {
		for _, check := range agent.Validation {
			if check.ExitCode == 0 {
				passed++
			} else {
				failed++
			}
		}
	}
	return passed, failed, waiting
}

func (m summaryModel) flattenChecks() []model.ValidationRun {
	var checks []model.ValidationRun
	for _, agent := range m.run.Agents {
		checks = append(checks, agent.Validation...)
	}
	return checks
}

func (m summaryModel) recommendedAgent() model.AgentResult {
	for _, agent := range m.run.Agents {
		if agent.Name == m.run.RecommendedAgent {
			return agent
		}
	}
	if len(m.run.Agents) > 0 {
		return m.run.Agents[0]
	}
	return model.AgentResult{}
}

func (m summaryModel) failedAgents() int {
	var count int
	for _, agent := range m.run.Agents {
		if agent.Status != "succeeded" {
			count++
		}
	}
	return count
}

func readLogPreview(path string, maxLines int) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	for i, line := range lines {
		lines[i] = mutedStyle.Render("> ") + line
	}
	return lines
}

func progressBar(percent, width int) string {
	if width <= 0 {
		return ""
	}
	filled := percent * width / 100
	if filled > width {
		filled = width
	}
	return okStyle.Render(strings.Repeat("|", filled)) + dimStyle.Render(strings.Repeat("-", width-filled))
}

func colorStatus(status string) string {
	switch status {
	case "succeeded":
		return okStyle.Render("SUCCEEDED")
	case "failed":
		return failStyle.Render("FAILED")
	default:
		return warnStyle.Render(strings.ToUpper(status))
	}
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func currentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "-"
	}
	return dir
}

func cleanPath(path string) string {
	if path == "" {
		return "-"
	}
	if rel, err := filepath.Rel(currentDirNoClean(), path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func currentDirNoClean() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:max(0, width-1)] + "."
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func totalFiles(agents []model.AgentResult) int {
	var total int
	for _, agent := range agents {
		total += agent.FilesChanged
	}
	return total
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type kv struct {
	key   string
	value string
}
