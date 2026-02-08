package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoadingModel shows a spinner with a message while loading.
type LoadingModel struct {
	message string
	spinner spinner.Model
}

// NewLoadingModel creates a new loading indicator.
func NewLoadingModel(message string) LoadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)
	return LoadingModel{message: message, spinner: s}
}

func (m LoadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m LoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m LoadingModel) View() string {
	header := titleStyle.Render(m.message)
	status := infoStyle.Render(fmt.Sprintf("%s working...", m.spinner.View()))
	return lipgloss.JoinVertical(lipgloss.Left, header, status)
}
