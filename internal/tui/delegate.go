package tui

import (
	"fmt"
	"io"
	"strings"

	"cli-tg-chat-summary/internal/telegram"

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

	icon := chatIcon(i.chat)
	title := i.chat.Title
	badge := formatUnreadBadge(i.chat.UnreadCount)

	str := fmt.Sprintf("%s %s%s", icon, title, badge)
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

	badge := formatUnreadBadge(i.topic.UnreadCount)
	str := fmt.Sprintf("%s %s%s", iconTopic, i.topic.Title, badge)
	renderListItem(w, str, index == m.Index())
}

// chatIcon returns an emoji icon based on chat type.
func chatIcon(chat telegram.Chat) string {
	switch {
	case chat.IsForum:
		return iconForum
	case chat.IsBot:
		return iconBot
	case chat.IsUser:
		return iconPrivate
	case chat.IsChannel:
		return iconChannel
	default:
		return iconGroup
	}
}

// formatUnreadBadge returns a styled unread count or empty string.
func formatUnreadBadge(count int) string {
	if count <= 0 {
		return ""
	}
	return unreadBadge.Render(fmt.Sprintf(" ⬤ %d", count))
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

// modeItem represents an export mode option.
type modeItem struct {
	mode  ExportMode
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
