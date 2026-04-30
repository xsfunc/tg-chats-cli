package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
)

type App struct {
	cfg      *config.Config
	tgClient *telegram.Client
	exporter Exporter
}

func New(cfg *config.Config, tgClient *telegram.Client) *App {
	return NewWithExporter(cfg, tgClient, NewDefaultExporter())
}

func NewWithExporter(cfg *config.Config, tgClient *telegram.Client, exporter Exporter) *App {
	if exporter == nil {
		exporter = NewDefaultExporter()
	}
	return &App{
		cfg:      cfg,
		tgClient: tgClient,
		exporter: exporter,
	}
}

type RunOptions struct {
	Since          time.Time
	Until          time.Time
	UseDateRange   bool
	ExportFormat   string
	ChatID         int64
	ChatIDRaw      int64
	TopicID        int
	TopicTitle     string
	NonInteractive bool
}

func (a *App) Run(ctx context.Context, opts RunOptions) error {
	if !opts.NonInteractive && !stdioIsTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; run from an interactive shell or pass --id for non-interactive export")
	}

	// Login
	fmt.Fprintln(os.Stderr, "Connecting to Telegram...")
	if err := a.tgClient.Login(ctx, os.Stdin); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Connected. Loading chats...")

	if opts.NonInteractive {
		return a.runNonInteractive(ctx, opts)
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

func (a *App) runNonInteractive(ctx context.Context, opts RunOptions) error {
	chats, err := a.tgClient.GetDialogs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dialogs: %w", err)
	}

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
		selectedTopic, err = selectForumTopic(topics, opts.TopicID, opts.TopicTitle)
		if err != nil {
			return err
		}
		if selectedTopic == nil {
			return fmt.Errorf("forum chat requires --topic-id or --topic")
		}
	}

	plan, err := a.buildFetchPlan(*selectedChat, selectedTopic, opts)
	if err != nil {
		return err
	}
	messages, err := plan.fetch(ctx, nil)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		fmt.Fprintln(os.Stderr, "No text messages found to export.")
		return nil
	}

	filename, err := a.exportMessages(plan.exportTitle, messages, opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Successfully exported %d messages to %s\n", len(messages), filename)
	markResult := a.markMessagesAsRead(ctx, *selectedChat, selectedTopic, messages, opts)
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

func (a *App) exportMessages(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error) {
	// Sort messages by date (oldest first)
	// fetched messages are usually newest first from history?
	// `GetUnreadMessages` implementation appended them as they came.
	// If we used `MessagesGetHistory` without offset loop, we got newest first.
	// Let's reverse to have chronological order for reading.
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	// Export to file
	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	// date range format: ChatName_YYYY-MM-DD_to_YYYY-MM-DD.txt
	filename, err := a.exporter.Export(exportTitle, messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to export: %w", err)
	}

	return filename, nil
}

type markReadResult struct {
	Attempted bool
	Err       error
}

func (a *App) markMessagesAsRead(ctx context.Context, selectedChat telegram.Chat, selectedTopic *telegram.Topic, messages []telegram.Message, opts RunOptions) markReadResult {
	// Mark as read
	maxID := 0
	for _, msg := range messages {
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

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	res := name
	for _, char := range invalid {
		res = strings.ReplaceAll(res, char, "_")
	}
	return strings.TrimSpace(res)
}

func formatMarkReadStatus(result markReadResult) string {
	if !result.Attempted {
		return ""
	}
	if result.Err != nil {
		return fmt.Sprintf("Warning: failed to mark messages as read: %v", result.Err)
	}
	return "Messages marked as read."
}

func printMarkReadStatus(result markReadResult) {
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
