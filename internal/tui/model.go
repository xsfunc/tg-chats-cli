package tui

import (
	"fmt"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/telegram"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HistoryMode determines how messages are selected for saving.
type HistoryMode int

const (
	ModeUnread HistoryMode = iota
	ModeDateRange
)

// viewState tracks which UI screen is active.
type viewState int

const (
	stateChatList   viewState = iota // main chat selection
	stateModeList                    // history mode selection
	stateSinceInput                  // date range: start date input
	stateUntilInput                  // date range: end date input
)

// ModelOptions configures the initial state of Model.
type ModelOptions struct {
	Mode  HistoryMode
	Since time.Time
	Until time.Time
}

// Model is the main TUI model for chat selection.
type Model struct {
	list         list.Model
	modeList     list.Model
	selected     *telegram.Chat
	quitting     bool
	done         bool
	canceled     bool
	markReadFunc func(telegram.Chat) error
	statusMsg    string
	errorMsg     string
	mode         HistoryMode
	state        viewState
	sinceInput   textinput.Model
	untilInput   textinput.Model
	since        time.Time
	until        time.Time
}

type statusClearMsg struct{}

// NewModel creates a new chat selection model.
func NewModel(chats []telegram.Chat, markReadFunc func(telegram.Chat) error, opts ModelOptions) Model {
	items := make([]list.Item, len(chats))
	for i, chat := range chats {
		items[i] = chatItem{chat: chat}
	}

	l := list.New(items, chatDelegate{}, defaultListWidth, defaultListHeight)
	l.Title = "Select Chat to Summarize"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = helpStyle

	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "mark read")),
			key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("^r", "mark read")),
			key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
		}
	}

	modeItems := []list.Item{
		modeItem{mode: ModeUnread, label: "Unread messages"},
		modeItem{mode: ModeDateRange, label: "Date range"},
	}
	modeList := list.New(modeItems, modeDelegate{}, defaultListWidth, 8)
	modeList.Title = "Select History Mode"
	modeList.SetShowStatusBar(false)
	modeList.SetFilteringEnabled(false)
	modeList.Styles.Title = titleStyle
	modeList.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	modeList.Styles.HelpStyle = helpStyle

	sinceInput := textinput.New()
	sinceInput.Placeholder = "YYYY-MM-DD"
	sinceInput.CharLimit = 10
	sinceInput.Width = 12

	untilInput := textinput.New()
	untilInput.Placeholder = "YYYY-MM-DD (optional)"
	untilInput.CharLimit = 10
	untilInput.Width = 12

	mode := opts.Mode
	if mode != ModeDateRange {
		mode = ModeUnread
	}

	if !opts.Since.IsZero() {
		sinceInput.SetValue(opts.Since.Format(dateFormat))
	}
	if !opts.Until.IsZero() {
		untilInput.SetValue(opts.Until.Format(dateFormat))
	}

	return Model{
		list:         l,
		modeList:     modeList,
		markReadFunc: markReadFunc,
		mode:         mode,
		state:        stateChatList,
		sinceInput:   sinceInput,
		untilInput:   untilInput,
		since:        opts.Since,
		until:        opts.Until,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.applyWindowSize(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case statusClearMsg:
		m.statusMsg = ""
		return m, nil
	}

	return m.updateActiveComponent(msg)
}

// handleKeyMsg routes key presses based on current state.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateModeList:
		return m.handleModeListKeys(msg)
	case stateSinceInput:
		return m.handleSinceInputKeys(msg)
	case stateUntilInput:
		return m.handleUntilInputKeys(msg)
	default:
		return m.handleChatListKeys(msg)
	}
}

// handleChatListKeys handles keys in the main chat list view.
func (m Model) handleChatListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keypress := msg.String(); keypress {
	case "ctrl+c", "esc":
		m.quitting = true
		m.done = true
		m.canceled = true
		return m, nil

	case "q", "Q":
		if m.list.FilterState() != list.Filtering {
			m.quitting = true
			m.done = true
			m.canceled = true
			return m, nil
		}

	case "m":
		if m.list.FilterState() != list.Filtering {
			m.state = stateModeList
			m.errorMsg = ""
			return m, nil
		}

	case "enter":
		i, ok := m.list.SelectedItem().(chatItem)
		if ok {
			m.selected = &i.chat
		}
		m.done = true
		return m, nil

	case "ctrl+r":
		return m.handleMarkAsRead()
	}

	return m.updateActiveComponent(msg)
}

// handleModeListKeys handles keys in the mode selection view.
func (m Model) handleModeListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keypress := msg.String(); keypress {
	case "ctrl+c", "esc":
		m.state = stateChatList
		return m, nil

	case "enter":
		i, ok := m.modeList.SelectedItem().(modeItem)
		if ok {
			if i.mode == ModeDateRange {
				m.state = stateSinceInput
				m.errorMsg = ""
				m.sinceInput.Focus()
				m.untilInput.Blur()
				return m, textinput.Blink
			}
			m.mode = ModeUnread
			m.state = stateChatList
		}
		return m, nil
	}

	return m.updateActiveComponent(msg)
}

// handleSinceInputKeys handles keys when entering the start date.
func (m Model) handleSinceInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keypress := msg.String(); keypress {
	case "ctrl+c":
		m.quitting = true
		m.done = true
		m.canceled = true
		return m, nil

	case "esc":
		m.errorMsg = ""
		m.state = stateChatList
		m.sinceInput.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.sinceInput.Value())
		if value == "" {
			m.errorMsg = "Start date is required (YYYY-MM-DD)"
			return m, nil
		}
		parsed, err := time.Parse(dateFormat, value)
		if err != nil {
			m.errorMsg = "Invalid date format. Use YYYY-MM-DD"
			return m, nil
		}
		m.since = parsed
		m.errorMsg = ""
		m.state = stateUntilInput
		m.sinceInput.Blur()
		m.untilInput.Focus()
		return m, textinput.Blink
	}

	return m.updateActiveComponent(msg)
}

// handleUntilInputKeys handles keys when entering the end date.
func (m Model) handleUntilInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keypress := msg.String(); keypress {
	case "ctrl+c":
		m.quitting = true
		m.done = true
		m.canceled = true
		return m, nil

	case "esc":
		m.errorMsg = ""
		m.state = stateChatList
		m.untilInput.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.untilInput.Value())
		if value == "" {
			m.until = time.Now()
		} else {
			parsed, err := time.Parse(dateFormat, value)
			if err != nil {
				m.errorMsg = "Invalid date format. Use YYYY-MM-DD"
				return m, nil
			}
			if parsed.Before(m.since) {
				m.errorMsg = "End date cannot be before start date"
				return m, nil
			}
			m.until = parsed.Add(24*time.Hour - time.Nanosecond)
		}
		m.mode = ModeDateRange
		m.errorMsg = ""
		m.state = stateChatList
		m.untilInput.Blur()
		return m, nil
	}

	return m.updateActiveComponent(msg)
}

// handleMarkAsRead marks the selected chat as read.
func (m Model) handleMarkAsRead() (tea.Model, tea.Cmd) {
	if m.list.FilterState() == list.Filtering {
		return m.updateActiveComponent(tea.KeyMsg{})
	}
	if m.markReadFunc == nil {
		m.statusMsg = "Error: Mark as read not available"
		return m, nil
	}

	i, ok := m.list.SelectedItem().(chatItem)
	if !ok {
		return m, nil
	}

	err := m.markReadFunc(i.chat)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
	} else {
		m.statusMsg = fmt.Sprintf("✓ Marked %s as read", i.chat.Title)

		// Update item in list to show 0 unread
		idx := m.list.Index()
		newChat := i.chat
		newChat.UnreadCount = 0
		items := m.list.Items()
		items[idx] = chatItem{chat: newChat}
		m.list.SetItems(items)
	}

	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

// updateActiveComponent forwards messages to the currently active component.
func (m Model) updateActiveComponent(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateModeList:
		m.modeList, cmd = m.modeList.Update(msg)
	case stateSinceInput:
		m.sinceInput, cmd = m.sinceInput.Update(msg)
	case stateUntilInput:
		m.untilInput, cmd = m.untilInput.Update(msg)
	default:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

// applyWindowSize updates list dimensions based on terminal size.
func (m *Model) applyWindowSize(msg tea.WindowSizeMsg) {
	width := listWidthForWindow(msg.Width)
	m.list.SetWidth(width)
	m.list.SetHeight(listHeightForWindow(msg.Height, 2))
	m.modeList.SetWidth(width)
	m.modeList.SetHeight(listHeightForWindow(msg.Height, 2))
}

func (m Model) View() string {
	if m.quitting {
		return quitTextStyle.Render("Bye!")
	}
	if m.selected != nil {
		return quitTextStyle.Render(fmt.Sprintf("Selected: %s", m.selected.Title))
	}

	switch m.state {
	case stateModeList:
		return m.viewModeList()
	case stateSinceInput:
		return m.viewSinceInput()
	case stateUntilInput:
		return m.viewUntilInput()
	default:
		return m.viewChatList()
	}
}

func (m Model) viewChatList() string {
	if len(m.list.Items()) == 0 {
		return m.viewEmptyList()
	}

	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return b.String()
}

func (m Model) viewEmptyList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select Chat to Summarize"))
	b.WriteString("\n\n")
	b.WriteString(emptyListStyle.Render("  No chats with unread messages found."))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press q or esc to quit."))
	return b.String()
}

func (m Model) viewModeList() string {
	var b strings.Builder
	b.WriteString(m.modeList.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: select  esc: back"))
	return b.String()
}

func (m Model) viewSinceInput() string {
	return renderDateInput("Start date (YYYY-MM-DD)", m.sinceInput, m.errorMsg, "")
}

func (m Model) viewUntilInput() string {
	sinceHint := fmt.Sprintf("Since: %s", m.since.Format(dateFormat))
	return renderDateInput("End date (YYYY-MM-DD, leave empty for now)", m.untilInput, m.errorMsg, sinceHint)
}

func renderDateInput(title string, input textinput.Model, errMsg, hint string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n  ")
	b.WriteString(input.View())
	if hint != "" {
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("  " + hint))
	}
	if errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(errMsg))
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))
	return b.String()
}

func (m Model) renderStatusBar() string {
	var b strings.Builder

	// Mode indicator with color
	b.WriteString("Mode: ")
	if m.mode == ModeDateRange {
		b.WriteString(modeDateStyle.Render("DATE RANGE"))
		if !m.since.IsZero() {
			b.WriteString(" ")
			b.WriteString(statusBarStyle.Render(fmt.Sprintf("(%s → %s)",
				m.since.Format(dateFormat),
				m.until.Format(dateFormat))))
		}
	} else {
		b.WriteString(modeUnreadStyle.Render("UNREAD"))
	}

	// Chat info
	if chat := m.currentChat(); chat != nil {
		b.WriteString(statusBarStyle.Render(fmt.Sprintf(" | %s | ID: %d", chatTypeLabel(*chat), chat.ID)))
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString(statusBarStyle.Render(" | " + m.statusMsg))
	}

	return statusBarStyle.Render(b.String())
}

func chatTypeLabel(chat telegram.Chat) string {
	switch {
	case chat.IsForum:
		return "forum"
	case chat.IsBot:
		return "bot"
	case chat.IsUser:
		return "private"
	case chat.IsChannel:
		return "channel"
	default:
		return "group"
	}
}

func (m Model) currentChat() *telegram.Chat {
	i, ok := m.list.SelectedItem().(chatItem)
	if !ok {
		return nil
	}
	return &i.chat
}

func (m Model) GetSelected() *telegram.Chat { return m.selected }
func (m Model) Done() bool                  { return m.done }
func (m Model) Canceled() bool              { return m.canceled }
func (m Model) GetHistoryMode() HistoryMode { return m.mode }

func (m Model) GetDateRange() (time.Time, time.Time, bool) {
	if m.mode != ModeDateRange {
		return time.Time{}, time.Time{}, false
	}
	return m.since, m.until, true
}

func modeLabel(mode HistoryMode) string {
	if mode == ModeDateRange {
		return "Date range"
	}
	return "Unread"
}

func listWidthForWindow(width int) int {
	adjusted := width - 2
	if adjusted < minListWidth {
		return minListWidth
	}
	return adjusted
}

func listHeightForWindow(height, extraLines int) int {
	adjusted := height - extraLines
	if adjusted < minListHeight {
		return minListHeight
	}
	return adjusted
}

// TopicModel is a TUI model for selecting forum topics.
type TopicModel struct {
	list     list.Model
	selected *telegram.Topic
	quitting bool
	done     bool
	canceled bool
}

// NewTopicModel creates a new topic selection model.
func NewTopicModel(topics []telegram.Topic) TopicModel {
	items := make([]list.Item, len(topics))
	for i, topic := range topics {
		items[i] = topicItem{topic: topic}
	}

	l := list.New(items, topicDelegate{}, defaultListWidth, defaultListHeight)
	l.Title = "Select Topic to Summarize"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().PaddingLeft(4)
	l.Styles.HelpStyle = helpStyle

	return TopicModel{list: l}
}

func (m TopicModel) Init() tea.Cmd {
	return nil
}

func (m TopicModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := listWidthForWindow(msg.Width)
		m.list.SetWidth(width)
		m.list.SetHeight(listHeightForWindow(msg.Height, 0))
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m TopicModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keypress := msg.String(); keypress {
	case "ctrl+c", "esc":
		m.quitting = true
		m.done = true
		m.canceled = true
		return m, nil

	case "q", "Q":
		if m.list.FilterState() != list.Filtering {
			m.quitting = true
			m.done = true
			m.canceled = true
			return m, nil
		}

	case "enter":
		i, ok := m.list.SelectedItem().(topicItem)
		if ok {
			m.selected = &i.topic
		}
		m.done = true
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m TopicModel) View() string {
	if m.quitting {
		return quitTextStyle.Render("Bye!")
	}
	if m.selected != nil {
		return quitTextStyle.Render(fmt.Sprintf("Selected topic: %s", m.selected.Title))
	}

	if len(m.list.Items()) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Select Topic to Summarize"))
		b.WriteString("\n\n")
		b.WriteString(emptyListStyle.Render("  No topics found in this forum."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press q or esc to go back."))
		return b.String()
	}

	return m.list.View()
}

func (m TopicModel) GetSelected() *telegram.Topic { return m.selected }
func (m TopicModel) Done() bool                   { return m.done }
func (m TopicModel) Canceled() bool               { return m.canceled }
