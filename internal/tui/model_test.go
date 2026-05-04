package tui

import (
	"testing"

	"tg-arc/internal/telegram"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	chats := []telegram.Chat{
		{ID: 1, Title: "Chat 1", UnreadCount: 5},
		{ID: 2, Title: "Chat 2", UnreadCount: 10},
		{ID: 3, Title: "Chat 3", UnreadCount: 0},
	}

	model := NewModel(chats, nil, ModelOptions{})

	if model.selected != nil {
		t.Error("expected selected to be nil initially")
	}
	if model.quitting {
		t.Error("expected quitting to be false initially")
	}
	if model.list.Title != "Select Chat to Summarize" {
		t.Errorf("unexpected list title: %s", model.list.Title)
	}
}

func TestNewModel_EmptyChats(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})

	if model.selected != nil {
		t.Error("expected selected to be nil")
	}
}

func TestModel_Init(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestModel_Update_Quit_CtrlC(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
	if !m.Done() || !m.Canceled() {
		t.Error("expected model to be done and canceled after Ctrl+C")
	}
	if cmd != nil {
		t.Error("expected no command after Ctrl+C")
	}
}

func TestModel_Update_Quit_Esc(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after Esc")
	}
	if !m.Done() || !m.Canceled() {
		t.Error("expected model to be done and canceled after Esc")
	}
	if cmd != nil {
		t.Error("expected no command after Esc")
	}
}

func TestModel_Update_Quit_Q(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after 'q'")
	}
	if !m.Done() || !m.Canceled() {
		t.Error("expected model to be done and canceled after 'q'")
	}
	if cmd != nil {
		t.Error("expected no command after 'q'")
	}
}

func TestModel_Update_Enter(t *testing.T) {
	chats := []telegram.Chat{
		{ID: 1, Title: "First Chat", UnreadCount: 5},
		{ID: 2, Title: "Second Chat", UnreadCount: 10},
	}
	model := NewModel(chats, nil, ModelOptions{})

	// Press Enter to select the first item
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	if m.selected == nil {
		t.Error("expected selected to not be nil after Enter")
	} else if m.selected.ID != 1 {
		t.Errorf("expected selected chat ID 1, got %d", m.selected.ID)
	}
	if !m.Done() || m.Canceled() {
		t.Error("expected model to be done and not canceled after Enter")
	}
	if cmd != nil {
		t.Error("expected no command after selection")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd := model.Update(msg)

	m := newModel.(Model)
	// Model should handle window resize without crashing
	if m.quitting {
		t.Error("model should not be quitting after window resize")
	}
	if cmd != nil {
		t.Error("expected no command from window resize")
	}
}

func TestModel_View(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_View_Quitting(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test"}}, nil, ModelOptions{})
	model.quitting = true

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view when quitting")
	}
}

func TestModel_View_Selected(t *testing.T) {
	model := NewModel([]telegram.Chat{{ID: 1, Title: "Test Chat"}}, nil, ModelOptions{})
	model.selected = &telegram.Chat{ID: 1, Title: "Test Chat"}

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view when selected")
	}
}

func TestModel_View_EmptyList(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view for empty list")
	}
}

func TestModel_GetSelected(t *testing.T) {
	model := NewModel([]telegram.Chat{}, nil, ModelOptions{})

	if model.GetSelected() != nil {
		t.Error("expected nil when nothing selected")
	}

	expectedChat := &telegram.Chat{ID: 42, Title: "Selected"}
	model.selected = expectedChat

	if model.GetSelected() != expectedChat {
		t.Error("GetSelected returned wrong chat")
	}
}

func TestChatItem_FilterValue(t *testing.T) {
	it := chatItem{chat: telegram.Chat{Title: "Test Chat"}}

	if it.FilterValue() != "Test Chat" {
		t.Errorf("expected FilterValue 'Test Chat', got '%s'", it.FilterValue())
	}
}

func TestChatDelegate_Height(t *testing.T) {
	d := chatDelegate{}
	if d.Height() != 1 {
		t.Errorf("expected Height 1, got %d", d.Height())
	}
}

func TestChatDelegate_Spacing(t *testing.T) {
	d := chatDelegate{}
	if d.Spacing() != 0 {
		t.Errorf("expected Spacing 0, got %d", d.Spacing())
	}
}

func TestChatDelegate_Update(t *testing.T) {
	d := chatDelegate{}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("expected nil command from Update")
	}
}

func TestModel_Update_CtrlR(t *testing.T) {
	chats := []telegram.Chat{{ID: 1, Title: "Test Chat", UnreadCount: 5}}
	called := false
	markRead := func(c telegram.Chat) error {
		called = true
		if c.ID != 1 {
			t.Errorf("expected chat ID 1, got %d", c.ID)
		}
		return nil
	}

	model := NewModel(chats, markRead, ModelOptions{})

	msg := tea.KeyMsg{Type: tea.KeyCtrlR}
	newModel, _ := model.Update(msg)

	m := newModel.(Model)
	if !called {
		t.Error("expected markReadFunc to be called")
	}
	if m.statusMsg == "" {
		t.Error("expected status message")
	}
}

func TestChatTypeLabel(t *testing.T) {
	tests := []struct {
		name string
		chat telegram.Chat
		want string
	}{
		{name: "forum", chat: telegram.Chat{IsForum: true}, want: "forum"},
		{name: "bot", chat: telegram.Chat{IsBot: true, IsUser: true}, want: "bot"},
		{name: "private", chat: telegram.Chat{IsUser: true}, want: "private"},
		{name: "channel", chat: telegram.Chat{IsChannel: true}, want: "channel"},
		{name: "group", chat: telegram.Chat{}, want: "group"},
	}

	for _, tt := range tests {
		if got := chatTypeLabel(tt.chat); got != tt.want {
			t.Errorf("%s: chatTypeLabel() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestStripEmoji(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Channel Name", "Channel Name"},
		{"📢 News Channel", "News Channel"},
		{"🔥 Hot 🔥 Topic 🔥", "Hot  Topic"},
		{"  spaces  ", "spaces"},
		{"NoEmoji", "NoEmoji"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := StripEmoji(tt.input); got != tt.want {
			t.Errorf("StripEmoji(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatChatLine(t *testing.T) {
	tests := []struct {
		title string
		count int
		want  string
	}{
		{"Chat", 0, "Chat"},
		{"Chat", -1, "Chat"},
		{"Chat", 5, "[5] Chat"},
		{"News", 100, "[100] News"},
	}

	for _, tt := range tests {
		if got := formatChatLine(tt.title, tt.count); got != tt.want {
			t.Errorf("formatChatLine(%q, %d) = %q, want %q", tt.title, tt.count, got, tt.want)
		}
	}
}

// TopicModel tests

func TestNewTopicModel(t *testing.T) {
	topics := []telegram.Topic{
		{ID: 1, Title: "General", UnreadCount: 5},
		{ID: 2, Title: "Off-topic", UnreadCount: 10},
	}

	model := NewTopicModel(topics)

	if model.selected != nil {
		t.Error("expected selected to be nil initially")
	}
	if model.quitting {
		t.Error("expected quitting to be false initially")
	}
	if model.list.Title != "Select Topic to Summarize" {
		t.Errorf("unexpected list title: %s", model.list.Title)
	}
}

func TestTopicModel_Init(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{})
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestTopicModel_Update_Quit(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
	if !m.Done() || !m.Canceled() {
		t.Error("expected topic model to be done and canceled after Ctrl+C")
	}
	if cmd != nil {
		t.Error("expected no command after Ctrl+C")
	}
}

func TestTopicModel_Update_Enter(t *testing.T) {
	topics := []telegram.Topic{
		{ID: 1, Title: "General", UnreadCount: 5},
	}
	model := NewTopicModel(topics)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if m.selected == nil {
		t.Error("expected selected to not be nil after Enter")
	}
	if !m.Done() || m.Canceled() {
		t.Error("expected topic model to be done and not canceled after Enter")
	}
	if cmd != nil {
		t.Error("expected no command after selection")
	}
}

func TestTopicModel_Update_WindowSize(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd := model.Update(msg)

	m := newModel.(TopicModel)
	if m.quitting {
		t.Error("model should not be quitting after window resize")
	}
	if cmd != nil {
		t.Error("expected no command from window resize")
	}
}

func TestTopicModel_View(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{{ID: 1, Title: "Test"}})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestTopicModel_View_EmptyList(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{})
	view := model.View()

	if view == "" {
		t.Error("expected non-empty view for empty list")
	}
}

func TestTopicModel_GetSelected(t *testing.T) {
	model := NewTopicModel([]telegram.Topic{})

	if model.GetSelected() != nil {
		t.Error("expected nil when nothing selected")
	}

	expectedTopic := &telegram.Topic{ID: 42, Title: "Selected"}
	model.selected = expectedTopic

	if model.GetSelected() != expectedTopic {
		t.Error("GetSelected returned wrong topic")
	}
}

func TestTopicItem_FilterValue(t *testing.T) {
	it := topicItem{topic: telegram.Topic{Title: "Test Topic"}}

	if it.FilterValue() != "Test Topic" {
		t.Errorf("expected FilterValue 'Test Topic', got '%s'", it.FilterValue())
	}
}

func TestTopicDelegate_Height(t *testing.T) {
	d := topicDelegate{}
	if d.Height() != 1 {
		t.Errorf("expected Height 1, got %d", d.Height())
	}
}

func TestTopicDelegate_Spacing(t *testing.T) {
	d := topicDelegate{}
	if d.Spacing() != 0 {
		t.Errorf("expected Spacing 0, got %d", d.Spacing())
	}
}

func TestTopicDelegate_Update(t *testing.T) {
	d := topicDelegate{}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("expected nil command from Update")
	}
}

func TestSummaryModel_View(t *testing.T) {
	model := NewSummaryModel("Test Chat", "data/tg-arc.db", 3, "Messages marked as read.")
	view := model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestSummaryModel_Update_Enter(t *testing.T) {
	model := NewSummaryModel("Test Chat", "data/tg-arc.db", 3, "")
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command after Enter")
	}
	m := newModel.(SummaryModel)
	if !m.quitting {
		t.Error("expected quitting to be true")
	}
	if !m.Done() {
		t.Error("expected done to be true")
	}
}

func TestModeLabel(t *testing.T) {
	if got := modeLabel(ModeUnread); got != "Unread" {
		t.Errorf("modeLabel(ModeUnread) = %q, want %q", got, "Unread")
	}
	if got := modeLabel(ModeDateRange); got != "Date range" {
		t.Errorf("modeLabel(ModeDateRange) = %q, want %q", got, "Date range")
	}
}

func TestListDimensions(t *testing.T) {
	// Test minimum bounds
	if w := listWidthForWindow(10); w != minListWidth {
		t.Errorf("listWidthForWindow(10) = %d, want %d", w, minListWidth)
	}
	if h := listHeightForWindow(5, 2); h != minListHeight {
		t.Errorf("listHeightForWindow(5, 2) = %d, want %d", h, minListHeight)
	}

	// Test normal values
	if w := listWidthForWindow(100); w != 98 {
		t.Errorf("listWidthForWindow(100) = %d, want 98", w)
	}
	if h := listHeightForWindow(50, 2); h != 48 {
		t.Errorf("listHeightForWindow(50, 2) = %d, want 48", h)
	}
}
