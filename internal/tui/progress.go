package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressMsg updates the progress display.
type ProgressMsg struct {
	Phase   string
	Parsed  int
	Scanned int
	Batch   int
}

type progressDoneMsg struct{}

// ProgressModel shows message parsing progress.
type ProgressModel struct {
	title   string
	phase   string
	parsed  int
	scanned int
	batches int
	spinner spinner.Model
	msgCh   <-chan tea.Msg
	done    bool
}

// NewProgressModel creates a new progress indicator.
func NewProgressModel(title string, msgCh <-chan tea.Msg) ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)
	return ProgressModel{
		title:   title,
		spinner: s,
		msgCh:   msgCh,
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForProgress(m.msgCh))
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressDoneMsg:
		m.done = true
		return m, nil
	case ProgressMsg:
		m.parsed += msg.Parsed
		m.scanned += msg.Scanned
		m.batches += msg.Batch
		if msg.Phase != "" {
			m.phase = msg.Phase
		}
		return m, waitForProgress(m.msgCh)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m ProgressModel) View() string {
	if m.done {
		return infoStyle.Render(modeUnreadStyle.Render("✓") + " Done.")
	}

	header := titleStyle.Render(fmt.Sprintf("Parsing: %s", m.title))

	status := fmt.Sprintf("%s parsed %d messages", m.spinner.View(), m.parsed)
	if m.scanned > 0 {
		status = fmt.Sprintf("%s parsed %d messages (scanned %d in %d batches)",
			m.spinner.View(), m.parsed, m.scanned, m.batches)
	}

	lines := []string{
		header,
		infoStyle.Render(status),
		helpStyle.Render("q: stop after request  ctrl+c: cancel"),
	}
	if m.phase != "" {
		lines = append(lines, infoStyle.Render(statusBarStyle.Render(fmt.Sprintf("Phase: %s", m.phase))))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m ProgressModel) Done() bool {
	return m.done
}

func (m ProgressModel) StopRequested() ProgressModel {
	m.phase = "stopping after current request"
	return m
}

func waitForProgress(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return progressDoneMsg{}
		}
		return msg
	}
}
