package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MessageModel displays a simple message with title, body, and footer.
type MessageModel struct {
	title    string
	body     string
	footer   string
	quitting bool
	done     bool
}

// NewMessageModel creates a new message display.
func NewMessageModel(title, body, footer string) MessageModel {
	if footer == "" {
		footer = "Press Enter to continue."
	}
	return MessageModel{title: title, body: body, footer: footer}
}

func (m MessageModel) Init() tea.Cmd { return nil }

func (m MessageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m MessageModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	if m.title != "" {
		b.WriteString(titleStyle.Render(m.title))
		b.WriteString("\n\n")
	}
	if m.body != "" {
		b.WriteString(infoStyle.Render(m.body))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render(m.footer))

	return b.String()
}

func (m MessageModel) Done() bool {
	return m.done
}
