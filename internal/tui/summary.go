package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/divinity/divinity/internal/model"
)

type summaryModel struct {
	run model.Run
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	winStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	failStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	muteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func ShowSummary(run model.Run) error {
	_, err := tea.NewProgram(summaryModel{run: run}).Run()
	return err
}

func (m summaryModel) Init() tea.Cmd {
	return nil
}

func (m summaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m summaryModel) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("Divinity run complete"))
	fmt.Fprintf(&b, "%s %s\n", muteStyle.Render("Run:"), m.run.ID)
	fmt.Fprintf(&b, "%s %s\n", muteStyle.Render("Task:"), m.run.Task)
	if m.run.RecommendedAgent != "" {
		fmt.Fprintf(&b, "%s %s\n", muteStyle.Render("Recommendation:"), winStyle.Render(m.run.RecommendedAgent))
	}

	for _, agent := range m.run.Agents {
		status := winStyle.Render(agent.Status)
		if agent.Status != "succeeded" {
			status = failStyle.Render(agent.Status)
		}
		fmt.Fprintf(&b, "\n%s  %s  score=%d  files=%d  +%d -%d\n", status, agent.Name, agent.Score, agent.FilesChanged, agent.LinesAdded, agent.LinesDeleted)
		if agent.Error != "" {
			fmt.Fprintf(&b, "  %s %s\n", failStyle.Render("error:"), agent.Error)
		}
		for _, check := range agent.Validation {
			marker := winStyle.Render("ok")
			if check.ExitCode != 0 {
				marker = failStyle.Render("fail")
			}
			fmt.Fprintf(&b, "  %s validation %s exit=%d\n", marker, check.Name, check.ExitCode)
		}
		if agent.DiffPath != "" {
			fmt.Fprintf(&b, "  %s %s\n", muteStyle.Render("diff:"), agent.DiffPath)
		}
		if agent.LogPath != "" {
			fmt.Fprintf(&b, "  %s %s\n", muteStyle.Render("log:"), agent.LogPath)
		}
	}

	fmt.Fprintf(&b, "\n%s\n", muteStyle.Render("Press Enter or q to close."))
	return b.String()
}
