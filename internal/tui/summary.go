package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SummaryModel shows the result of a save and waits for user confirmation.
type SummaryModel struct {
	title      string
	filename   string
	count      int
	markStatus string
	quitting   bool
	done       bool
}

// NewSummaryModel creates a new save summary display.
func NewSummaryModel(title, filename string, count int, markStatus string) SummaryModel {
	return SummaryModel{
		title:      title,
		filename:   filename,
		count:      count,
		markStatus: markStatus,
	}
}

func (m SummaryModel) Init() tea.Cmd { return nil }

func (m SummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			m.done = true
			return m, nil
		}
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'q' || msg.Runes[0] == 'Q') {
			m.quitting = true
			m.done = true
			return m, nil
		}
	}
	return m, nil
}

func (m SummaryModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header with success indicator
	b.WriteString(titleStyle.Render(modeUnreadStyle.Render("✓") + " Save complete"))
	b.WriteString("\n\n")

	// Details
	b.WriteString(infoStyle.Render(fmt.Sprintf("Chat:     %s", m.title)))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render(fmt.Sprintf("Messages: %d", m.count)))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render(fmt.Sprintf("Database: %s", m.filename)))

	if m.markStatus != "" {
		b.WriteString("\n\n")
		b.WriteString(infoStyle.Render(statusBarStyle.Render(m.markStatus)))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press Enter to return to chat list."))

	return b.String()
}

func (m SummaryModel) Done() bool {
	return m.done
}
