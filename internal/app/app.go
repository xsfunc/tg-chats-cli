package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/store"
	"cli-tg-chat-summary/internal/telegram"
)

type App struct {
	cfg      *config.Config
	tgClient *telegram.Client
	store    store.Store
}

func New(cfg *config.Config, tgClient *telegram.Client) *App {
	return &App{
		cfg:      cfg,
		tgClient: tgClient,
	}
}

type RunOptions struct {
	Since          time.Time
	Until          time.Time
	UseDateRange   bool
	Command        string
	DBPath         string
	SessionPath    string
	ChatLimit      int
	MessageLimit   int
	ChatID         int64
	ChatIDRaw      int64
	TopicID        int
	TopicTitle     string
	NonInteractive bool
}

func (a *App) Run(ctx context.Context, opts RunOptions) error {
	if opts.Command == "" {
		opts.Command = "history"
	}
	if opts.DBPath == "" {
		opts.DBPath = store.DefaultPath
	}
	if opts.SessionPath != "" {
		a.cfg.SessionPath = opts.SessionPath
	}
	if opts.Command != "sync" && !opts.NonInteractive && !stdioIsTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; run from an interactive shell or pass --id for non-interactive history")
	}

	// Login
	fmt.Fprintln(os.Stderr, "Connecting to Telegram...")
	if err := a.tgClient.Login(ctx, os.Stdin); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	account, err := a.tgClient.Account()
	if err != nil {
		return fmt.Errorf("read telegram account: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Connected. Loading chats...")

	db, err := store.Open(ctx, opts.DBPath)
	if err != nil {
		return fmt.Errorf("open sqlite store: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()
	a.store = db
	if err := a.store.SetAccount(ctx, account); err != nil {
		return fmt.Errorf("set storage account: %w", err)
	}

	if opts.Command == "sync" {
		return a.runSync(ctx, opts)
	}
	if opts.NonInteractive {
		return a.runNonInteractiveHistory(ctx, opts)
	}
	return a.runInteractiveTUI(ctx, opts)
}

func stdioIsTerminal() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fileModeIsTerminal(stdinInfo.Mode()) && fileModeIsTerminal(stdoutInfo.Mode())
}

func fileModeIsTerminal(mode os.FileMode) bool {
	return mode&os.ModeCharDevice != 0
}

func (a *App) runNonInteractiveHistory(ctx context.Context, opts RunOptions) error {
	dialogs, err := a.tgClient.GetDialogsWithEntities(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dialogs: %w", err)
	}
	if err := a.store.SaveUsers(ctx, dialogs.Users); err != nil {
		return err
	}
	if err := a.store.SaveChats(ctx, dialogs.Chats); err != nil {
		return err
	}
	chats := dialogs.Chats

	if len(chats) == 0 {
		fmt.Fprintln(os.Stderr, "No chats found.")
		return nil
	}

	selectedChat := findChatByID(chats, opts.ChatID)
	if selectedChat == nil {
		chatID := opts.ChatID
		if opts.ChatIDRaw != 0 {
			chatID = opts.ChatIDRaw
		}
		return fmt.Errorf("chat with id %d not found; accepts raw ID or -100... format", chatID)
	}

	var selectedTopic *telegram.Topic
	if selectedChat.IsForum {
		if opts.TopicID == 0 && opts.TopicTitle == "" {
			return fmt.Errorf("forum chat requires --topic-id or --topic")
		}
		topics, err := a.tgClient.GetForumTopics(ctx, selectedChat.ID)
		if err != nil {
			return fmt.Errorf("failed to get forum topics: %w", err)
		}
		if err := a.store.SaveTopics(ctx, selectedChat.ID, topics); err != nil {
			return err
		}
		selectedTopic, err = selectForumTopic(topics, opts.TopicID, opts.TopicTitle)
		if err != nil {
			return err
		}
		if selectedTopic == nil {
			return fmt.Errorf("forum chat requires --topic-id or --topic")
		}
	}

	run, err := a.store.StartRun(ctx, "history")
	if err != nil {
		return err
	}
	status := "ok"
	var runErr error
	defer func() {
		_ = a.store.FinishRun(ctx, run.ID, status, runErr)
	}()

	plan, err := a.buildFetchPlan(*selectedChat, selectedTopic, opts)
	if err != nil {
		status = "error"
		runErr = err
		return err
	}
	result, err := plan.fetch(ctx, nil)
	if err != nil {
		status = "error"
		runErr = err
		return err
	}

	saved, err := a.saveFetchResult(ctx, *selectedChat, selectedTopic, result)
	if err != nil {
		status = "error"
		runErr = err
		return err
	}
	markResult := a.markMessagesAsRead(ctx, *selectedChat, selectedTopic, result, opts)
	if err := a.store.AddRunItem(ctx, store.RunItem{
		RunID:          run.ID,
		ChatID:         selectedChat.ID,
		TopicID:        topicID(selectedTopic),
		SavedMessages:  saved,
		MarkReadStatus: formatMarkReadStatus(markResult),
		Warning:        markResult.Warning,
	}); err != nil {
		status = "error"
		runErr = err
		return err
	}

	if saved == 0 {
		fmt.Fprintln(os.Stderr, "No text messages found to save.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Successfully saved %d messages to %s\n", saved, opts.DBPath)
	printMarkReadStatus(markResult)
	return nil
}

func findChatByID(chats []telegram.Chat, chatID int64) *telegram.Chat {
	for i := range chats {
		if chats[i].ID == chatID {
			return &chats[i]
		}
	}
	return nil
}

func selectForumTopic(topics []telegram.Topic, topicID int, topicTitle string) (*telegram.Topic, error) {
	if topicID != 0 {
		for i := range topics {
			if topics[i].ID == topicID {
				return &topics[i], nil
			}
		}
		return nil, fmt.Errorf("forum topic id %d not found", topicID)
	}

	title := strings.TrimSpace(topicTitle)
	if title == "" {
		return nil, fmt.Errorf("forum chat requires --topic-id or --topic")
	}

	lowerTitle := strings.ToLower(title)
	var exactMatches []telegram.Topic
	for _, topic := range topics {
		if strings.EqualFold(topic.Title, title) {
			exactMatches = append(exactMatches, topic)
		}
	}
	if len(exactMatches) == 1 {
		return &exactMatches[0], nil
	}
	if len(exactMatches) > 1 {
		return nil, fmt.Errorf("forum topic title %q matched multiple topics: %s", title, formatTopicCandidates(exactMatches))
	}

	var containsMatches []telegram.Topic
	for _, topic := range topics {
		if strings.Contains(strings.ToLower(topic.Title), lowerTitle) {
			containsMatches = append(containsMatches, topic)
		}
	}
	if len(containsMatches) == 1 {
		return &containsMatches[0], nil
	}
	if len(containsMatches) > 1 {
		return nil, fmt.Errorf("forum topic title %q matched multiple topics: %s", title, formatTopicCandidates(containsMatches))
	}
	return nil, fmt.Errorf("forum topic title %q not found", title)
}

func formatTopicCandidates(topics []telegram.Topic) string {
	var parts []string
	for _, topic := range topics {
		parts = append(parts, fmt.Sprintf("id=%d title=%q", topic.ID, topic.Title))
	}
	return strings.Join(parts, ", ")
}

type markReadResult struct {
	Attempted bool
	Err       error
	Warning   string
}

func (a *App) saveFetchResult(ctx context.Context, selectedChat telegram.Chat, selectedTopic *telegram.Topic, result telegram.MessageFetchResult) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("sqlite store is not initialized")
	}
	if err := a.store.SaveUsers(ctx, result.Users); err != nil {
		return 0, err
	}
	chats := append([]telegram.Chat{selectedChat}, result.Chats...)
	if err := a.store.SaveChats(ctx, chats); err != nil {
		return 0, err
	}
	if selectedTopic != nil {
		if err := a.store.SaveTopics(ctx, selectedChat.ID, []telegram.Topic{*selectedTopic}); err != nil {
			return 0, err
		}
	}
	return a.store.SaveMessages(ctx, selectedChat.ID, topicID(selectedTopic), result.Messages)
}

func topicID(topic *telegram.Topic) int {
	if topic == nil {
		return 0
	}
	return topic.ID
}

func (a *App) markMessagesAsRead(ctx context.Context, selectedChat telegram.Chat, selectedTopic *telegram.Topic, result telegram.MessageFetchResult, opts RunOptions) markReadResult {
	if opts.UseDateRange {
		return markReadResult{}
	}
	if result.Truncated {
		return markReadResult{Warning: "skipped mark-as-read because message limit truncated unread messages"}
	}
	// Mark as read
	maxID := 0
	for _, msg := range result.Messages {
		if msg.ID > maxID {
			maxID = msg.ID
		}
	}

	if maxID > 0 && !opts.UseDateRange {
		var err error
		if selectedTopic != nil {
			err = a.tgClient.MarkTopicAsRead(ctx, selectedChat.ID, selectedTopic.ID, maxID)
		} else {
			err = a.tgClient.MarkAsRead(ctx, selectedChat, maxID)
		}
		return markReadResult{Attempted: true, Err: err}
	}
	return markReadResult{}
}

func formatMarkReadStatus(result markReadResult) string {
	if result.Warning != "" {
		return "Warning: " + result.Warning
	}
	if !result.Attempted {
		return ""
	}
	if result.Err != nil {
		return fmt.Sprintf("Warning: failed to mark messages as read: %v", result.Err)
	}
	return "Messages marked as read."
}

func printMarkReadStatus(result markReadResult) {
	if result.Warning != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", result.Warning)
		return
	}
	if !result.Attempted {
		return
	}
	fmt.Fprintln(os.Stderr, "Marking messages as read...")
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to mark messages as read: %v\n", result.Err)
	} else {
		fmt.Fprintln(os.Stderr, "Messages marked as read.")
	}
}
