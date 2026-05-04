package app

import (
	"context"
	"fmt"

	"tg-arc/internal/telegram"
	"tg-arc/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type appState int

const (
	stateLoadingChats appState = iota
	stateChatList
	stateLoadingTopics
	stateTopicList
	stateProgress
	stateSummary
	stateMessage
	stateExit
)

type chatsLoadedMsg struct {
	chats []telegram.Chat
	err   error
}

type topicsLoadedMsg struct {
	topics []telegram.Topic
	err    error
}

type fetchResultMsg struct {
	result telegram.MessageFetchResult
	err    error
}

type appModel struct {
	app         *App
	ctx         context.Context
	opts        RunOptions
	state       appState
	messageNext appState

	loading  tui.LoadingModel
	message  tui.MessageModel
	chat     tui.Model
	topic    tui.TopicModel
	progress tui.ProgressModel
	summary  tui.SummaryModel

	selectedChat  *telegram.Chat
	selectedTopic *telegram.Topic
	displayTitle  string
	fetchHandle   *fetchHandle
	err           error
}

func newAppModel(app *App, ctx context.Context, opts RunOptions) appModel {
	m := appModel{app: app, ctx: ctx, opts: opts, state: stateLoadingChats}
	m.loading = tui.NewLoadingModel("Fetching chats...")
	return m
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(m.loading.Init(), fetchChatsCmd(m.ctx, m.app.tgClient))
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if isCtrlC(msg) {
		m.cancelFetch()
		m.state = stateExit
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case chatsLoadedMsg:
		if msg.err != nil {
			return m.setMessage("Error", msg.err.Error(), "Press Enter to exit.", stateExit, msg.err), nil
		}
		if len(msg.chats) == 0 {
			return m.setMessage("", "No chats found.", "Press Enter to exit.", stateExit, nil), nil
		}
		m.chat = m.newChatModel(msg.chats)
		m.state = stateChatList
		return m, nil

	case topicsLoadedMsg:
		if msg.err != nil {
			return m.setMessage("Error", msg.err.Error(), "Press Enter to exit.", stateExit, msg.err), nil
		}
		if len(msg.topics) == 0 {
			return m.setMessage("", "No topics found in forum.", "Press Enter to return.", stateLoadingChats, nil), nil
		}
		m.topic = tui.NewTopicModel(msg.topics)
		m.state = stateTopicList
		return m, nil

	case fetchResultMsg:
		return m.handleFetchResult(msg)
	}

	switch m.state {
	case stateLoadingChats, stateLoadingTopics:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.loading.Update(msg)
		m.loading = updated.(tui.LoadingModel)
		return m, cmd

	case stateChatList:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.chat.Update(msg)
		m.chat = updated.(tui.Model)
		if m.chat.Done() {
			if m.chat.Canceled() {
				m.state = stateExit
				return m, tea.Quit
			}
			selected := m.chat.GetSelected()
			if selected == nil {
				return m.setMessage("", "No chat selected.", "Press Enter to exit.", stateExit, nil), nil
			}
			m.selectedChat = selected
			m.selectedTopic = nil
			m.applyHistoryMode()
			if selected.IsForum {
				m.loading = tui.NewLoadingModel(fmt.Sprintf("Fetching topics for forum %s...", selected.Title))
				m.state = stateLoadingTopics
				return m, tea.Batch(m.loading.Init(), fetchTopicsCmd(m.ctx, m.app.tgClient, selected.ID))
			}
			return m.startFetch()
		}
		return m, cmd

	case stateTopicList:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.topic.Update(msg)
		m.topic = updated.(tui.TopicModel)
		if m.topic.Done() {
			if m.topic.Canceled() {
				m.selectedTopic = nil
				m.state = stateChatList
				return m, nil
			}
			selected := m.topic.GetSelected()
			if selected == nil {
				return m.setMessage("", "No topic selected.", "Press Enter to return.", stateLoadingChats, nil), nil
			}
			m.selectedTopic = selected
			return m.startFetch()
		}
		return m, cmd

	case stateProgress:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.progress.Update(msg)
		m.progress = updated.(tui.ProgressModel)
		return m, cmd

	case stateSummary:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.summary.Update(msg)
		m.summary = updated.(tui.SummaryModel)
		if m.summary.Done() {
			m.loading = tui.NewLoadingModel("Fetching chats...")
			m.state = stateLoadingChats
			return m, tea.Batch(m.loading.Init(), fetchChatsCmd(m.ctx, m.app.tgClient))
		}
		return m, cmd

	case stateMessage:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.message.Update(msg)
		m.message = updated.(tui.MessageModel)
		if m.message.Done() {
			if m.messageNext == stateExit {
				m.state = stateExit
				return m, tea.Quit
			}
			m.loading = tui.NewLoadingModel("Fetching chats...")
			m.state = stateLoadingChats
			return m, tea.Batch(m.loading.Init(), fetchChatsCmd(m.ctx, m.app.tgClient))
		}
		return m, cmd

	default:
		return m, nil
	}
}

func (m appModel) View() string {
	switch m.state {
	case stateLoadingChats, stateLoadingTopics:
		return m.loading.View()
	case stateChatList:
		return m.chat.View()
	case stateTopicList:
		return m.topic.View()
	case stateProgress:
		return m.progress.View()
	case stateSummary:
		return m.summary.View()
	case stateMessage:
		return m.message.View()
	default:
		return ""
	}
}

func (m appModel) startFetch() (tea.Model, tea.Cmd) {
	plan, err := m.app.buildFetchPlan(*m.selectedChat, m.selectedTopic, m.opts)
	if err != nil {
		return m.setMessage("Error", err.Error(), "Press Enter to exit.", stateExit, err), nil
	}
	handle := m.app.startFetchWithProgress(FetchOpts{Ctx: m.ctx, Title: plan.progressTitle}, plan.fetch)
	m.fetchHandle = &handle
	m.displayTitle = plan.displayTitle
	m.progress = tui.NewProgressModel(plan.progressTitle, handle.msgCh)
	m.state = stateProgress
	return m, tea.Batch(m.progress.Init(), waitForFetchResult(handle.resultCh))
}

func (m appModel) handleFetchResult(msg fetchResultMsg) (tea.Model, tea.Cmd) {
	m.cancelFetch()
	if msg.err != nil {
		return m.setMessage("Error", msg.err.Error(), "Press Enter to exit.", stateExit, msg.err), nil
	}
	saved, err := m.app.saveFetchResult(m.ctx, *m.selectedChat, m.selectedTopic, msg.result)
	if err != nil {
		return m.setMessage("Error", err.Error(), "Press Enter to exit.", stateExit, err), nil
	}
	if saved == 0 {
		return m.setMessage("", "No text messages found to save.", "Press Enter to return.", stateLoadingChats, nil), nil
	}

	markResult := m.app.markMessagesAsRead(m.ctx, *m.selectedChat, m.selectedTopic, msg.result, m.opts)
	m.summary = tui.NewSummaryModel(m.displayTitle, m.opts.DBPath, saved, formatMarkReadStatus(markResult))
	m.state = stateSummary
	return m, nil
}

func (m appModel) setMessage(title, body, footer string, next appState, err error) appModel {
	m.message = tui.NewMessageModel(title, body, footer)
	m.messageNext = next
	m.state = stateMessage
	if err != nil {
		m.err = err
	}
	return m
}

func (m appModel) newChatModel(chats []telegram.Chat) tui.Model {
	markReadFunc := func(chat telegram.Chat) error {
		if chat.IsForum {
			return markForumAsRead(m.ctx, m.app.tgClient, chat)
		}
		if chat.TopMessageID == 0 {
			return fmt.Errorf("no top message id found")
		}
		return m.app.tgClient.MarkAsRead(m.ctx, chat, chat.TopMessageID)
	}

	modelOpts := tui.ModelOptions{}
	if m.opts.UseDateRange {
		modelOpts.Mode = tui.ModeDateRange
		modelOpts.Since = m.opts.Since
		modelOpts.Until = m.opts.Until
	}
	return tui.NewModel(chats, markReadFunc, modelOpts)
}

func (m *appModel) applyHistoryMode() {
	mode := m.chat.GetHistoryMode()
	if mode == tui.ModeDateRange {
		since, until, ok := m.chat.GetDateRange()
		if ok {
			m.opts.UseDateRange = true
			m.opts.Since = since
			m.opts.Until = until
		}
		return
	}
	m.opts.UseDateRange = false
}

func (m *appModel) cancelFetch() {
	if m.fetchHandle == nil {
		return
	}
	m.fetchHandle.cancel()
	m.fetchHandle = nil
}

type forumMarkClient interface {
	GetForumTopics(ctx context.Context, chatID int64) ([]telegram.Topic, error)
	MarkTopicAsRead(ctx context.Context, chatID int64, topicID int, maxID int) error
}

func markForumAsRead(ctx context.Context, client forumMarkClient, chat telegram.Chat) error {
	topics, err := client.GetForumTopics(ctx, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to get forum topics: %w", err)
	}
	for _, topic := range topics {
		if topic.UnreadCount == 0 {
			continue
		}
		if topic.TopMessageID == 0 {
			return fmt.Errorf("topic %q (id=%d) has no top message id", topic.Title, topic.ID)
		}
		if err := client.MarkTopicAsRead(ctx, chat.ID, topic.ID, topic.TopMessageID); err != nil {
			return fmt.Errorf("topic %q (id=%d): %w", topic.Title, topic.ID, err)
		}
	}
	return nil
}

func fetchChatsCmd(ctx context.Context, client *telegram.Client) tea.Cmd {
	return func() tea.Msg {
		chats, err := client.GetDialogs(ctx)
		return chatsLoadedMsg{chats: chats, err: err}
	}
}

func fetchTopicsCmd(ctx context.Context, client *telegram.Client, chatID int64) tea.Cmd {
	return func() tea.Msg {
		topics, err := client.GetForumTopics(ctx, chatID)
		return topicsLoadedMsg{topics: topics, err: err}
	}
}

func waitForFetchResult(resultCh <-chan fetchResult) tea.Cmd {
	return func() tea.Msg {
		result := <-resultCh
		return fetchResultMsg(result)
	}
}

func isCtrlC(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	if key.Type == tea.KeyCtrlC {
		return true
	}
	return false
}
