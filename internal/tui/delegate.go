package tui

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"tg-arc/internal/telegram"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// chatItem wraps a telegram.Chat for use in a list.
type chatItem struct {
	chat telegram.Chat
}

func (i chatItem) FilterValue() string { return i.chat.Title }

// topicItem wraps a telegram.Topic for use in a list.
type topicItem struct {
	topic telegram.Topic
}

func (i topicItem) FilterValue() string { return i.topic.Title }

// chatDelegate renders chat items with icons and unread badges.
type chatDelegate struct{}

func (d chatDelegate) Height() int                             { return 1 }
func (d chatDelegate) Spacing() int                            { return 0 }
func (d chatDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d chatDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(chatItem)
	if !ok {
		return
	}

	title := StripEmoji(i.chat.Title)
	str := formatChatLine(title, i.chat.UnreadCount)
	renderListItem(w, str, index == m.Index())
}

// topicDelegate renders topic items with icons and unread badges.
type topicDelegate struct{}

func (d topicDelegate) Height() int                             { return 1 }
func (d topicDelegate) Spacing() int                            { return 0 }
func (d topicDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d topicDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(topicItem)
	if !ok {
		return
	}

	title := StripEmoji(i.topic.Title)
	str := formatChatLine(title, i.topic.UnreadCount)
	renderListItem(w, str, index == m.Index())
}

// emojiPattern matches Unicode emoji ranges.
var emojiPattern = regexp.MustCompile(`[\x{1F300}-\x{1FAF8}\x{2600}-\x{26FF}\x{2700}-\x{27BF}\x{FE00}-\x{FE0F}\x{1F000}-\x{1F0FF}\x{2300}-\x{23FF}\x{2B50}-\x{2B55}\x{200D}\x{20E3}\x{E0020}-\x{E007F}\x{1F1E0}-\x{1F1FF}]+`)

// StripEmoji removes emoji characters from the string and trims extra spaces.
func StripEmoji(s string) string {
	cleaned := emojiPattern.ReplaceAllString(s, "")
	return strings.TrimSpace(cleaned)
}

// formatChatLine returns formatted line: [count] Title or just Title if count is 0.
func formatChatLine(title string, count int) string {
	if count > 0 {
		return fmt.Sprintf("[%d] %s", count, title)
	}
	return title
}

// renderListItem writes a styled list item to the writer.
func renderListItem(w io.Writer, str string, selected bool) {
	if selected {
		str = selectedStyle.Render("▸ " + str)
	} else {
		str = itemStyle.Render(str)
	}
	_, _ = fmt.Fprint(w, str)
}

// modeItem represents a history mode option.
type modeItem struct {
	mode  HistoryMode
	label string
}

func (i modeItem) FilterValue() string { return i.label }
func (i modeItem) Title() string       { return i.label }
func (i modeItem) Description() string { return "" }

// modeDelegate renders mode selection items.
type modeDelegate struct{}

func (d modeDelegate) Height() int                             { return 1 }
func (d modeDelegate) Spacing() int                            { return 0 }
func (d modeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d modeDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(modeItem)
	if !ok {
		return
	}

	var icon string
	if i.mode == ModeUnread {
		icon = modeUnreadStyle.Render("◉")
	} else {
		icon = modeDateStyle.Render("◉")
	}

	str := fmt.Sprintf("%s %s", icon, i.label)
	if index == m.Index() {
		str = selectedStyle.Render("▸ " + strings.TrimPrefix(str, " "))
	} else {
		str = itemStyle.Render(str)
	}
	_, _ = fmt.Fprint(w, str)
}
