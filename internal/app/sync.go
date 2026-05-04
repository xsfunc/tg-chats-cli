package app

import (
	"context"
	"fmt"
	"os"

	"tg-arc/internal/store"
	"tg-arc/internal/telegram"
)

func (a *App) runSync(ctx context.Context, opts RunOptions) error {
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

	chats, err := syncChats(dialogs.Chats, opts)
	if err != nil {
		return err
	}
	if len(chats) == 0 {
		fmt.Fprintln(os.Stderr, "No unread chats found.")
		return nil
	}

	run, err := a.store.StartRun(ctx, "sync")
	if err != nil {
		return err
	}

	status := "ok"
	var firstErr error
	totalSaved := 0
	for _, chat := range chats {
		saved, err := a.syncChat(ctx, run.ID, chat, opts)
		totalSaved += saved
		if err != nil {
			status = "partial"
			if firstErr == nil {
				firstErr = err
			}
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
	if firstErr != nil && totalSaved == 0 {
		status = "error"
	}
	if err := a.store.FinishRun(ctx, run.ID, status, firstErr); err != nil {
		return err
	}
	if firstErr != nil && totalSaved == 0 {
		return firstErr
	}
	fmt.Fprintf(os.Stderr, "Sync saved %d messages to %s\n", totalSaved, opts.DBPath)
	return nil
}

func syncChats(chats []telegram.Chat, opts RunOptions) ([]telegram.Chat, error) {
	if opts.ChatID != 0 {
		chat := findChatByID(chats, opts.ChatID)
		if chat == nil {
			chatID := opts.ChatID
			if opts.ChatIDRaw != 0 {
				chatID = opts.ChatIDRaw
			}
			return nil, fmt.Errorf("chat with id %d not found; accepts raw ID or -100... format", chatID)
		}
		if chat.UnreadCount == 0 {
			return nil, nil
		}
		return []telegram.Chat{*chat}, nil
	}

	var result []telegram.Chat
	for _, chat := range chats {
		if chat.UnreadCount == 0 {
			continue
		}
		result = append(result, chat)
		if opts.ChatLimit > 0 && len(result) >= opts.ChatLimit {
			break
		}
	}
	return result, nil
}

func (a *App) syncChat(ctx context.Context, runID int64, chat telegram.Chat, opts RunOptions) (int, error) {
	if chat.IsForum {
		return a.syncForumChat(ctx, runID, chat, opts)
	}

	result, err := a.tgClient.GetUnreadMessagesWithOptions(ctx, chat.ID, chat.LastReadID, nil, telegram.MessageFetchOptions{
		MessageLimit: opts.MessageLimit,
	})
	if err != nil {
		_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, Error: err.Error()})
		return 0, fmt.Errorf("sync chat %q: %w", chat.Title, err)
	}
	saved, err := a.saveFetchResult(ctx, chat, nil, result)
	if err != nil {
		_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, Error: err.Error()})
		return 0, fmt.Errorf("save chat %q: %w", chat.Title, err)
	}
	markResult := a.markMessagesAsRead(ctx, chat, nil, result, opts)
	if err := a.store.AddRunItem(ctx, store.RunItem{
		RunID:          runID,
		ChatID:         chat.ID,
		SavedMessages:  saved,
		MarkReadStatus: formatMarkReadStatus(markResult),
		Warning:        markResult.Warning,
	}); err != nil {
		return saved, err
	}
	printMarkReadStatus(markResult)
	return saved, nil
}

func (a *App) syncForumChat(ctx context.Context, runID int64, chat telegram.Chat, opts RunOptions) (int, error) {
	topics, err := a.tgClient.GetForumTopics(ctx, chat.ID)
	if err != nil {
		_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, Error: err.Error()})
		return 0, fmt.Errorf("get forum topics for %q: %w", chat.Title, err)
	}
	if err := a.store.SaveTopics(ctx, chat.ID, topics); err != nil {
		return 0, err
	}

	selectedTopics, err := syncTopics(topics, opts)
	if err != nil {
		_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, Error: err.Error()})
		return 0, err
	}

	totalSaved := 0
	var firstErr error
	for _, topic := range selectedTopics {
		result, err := a.tgClient.GetTopicMessagesWithOptions(ctx, chat.ID, topic.ID, topic.LastReadID, nil, telegram.MessageFetchOptions{
			MessageLimit: opts.MessageLimit,
		})
		if err != nil {
			_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, TopicID: topic.ID, Error: err.Error()})
			if firstErr == nil {
				firstErr = fmt.Errorf("sync topic %q/%q: %w", chat.Title, topic.Title, err)
			}
			continue
		}
		saved, err := a.saveFetchResult(ctx, chat, &topic, result)
		totalSaved += saved
		if err != nil {
			_ = a.store.AddRunItem(ctx, store.RunItem{RunID: runID, ChatID: chat.ID, TopicID: topic.ID, Error: err.Error()})
			if firstErr == nil {
				firstErr = fmt.Errorf("save topic %q/%q: %w", chat.Title, topic.Title, err)
			}
			continue
		}
		markResult := a.markMessagesAsRead(ctx, chat, &topic, result, opts)
		if err := a.store.AddRunItem(ctx, store.RunItem{
			RunID:          runID,
			ChatID:         chat.ID,
			TopicID:        topic.ID,
			SavedMessages:  saved,
			MarkReadStatus: formatMarkReadStatus(markResult),
			Warning:        markResult.Warning,
		}); err != nil && firstErr == nil {
			firstErr = err
		}
		printMarkReadStatus(markResult)
	}
	return totalSaved, firstErr
}

func syncTopics(topics []telegram.Topic, opts RunOptions) ([]telegram.Topic, error) {
	if opts.TopicID != 0 || opts.TopicTitle != "" {
		topic, err := selectForumTopic(topics, opts.TopicID, opts.TopicTitle)
		if err != nil {
			return nil, err
		}
		if topic == nil || topic.UnreadCount == 0 {
			return nil, nil
		}
		return []telegram.Topic{*topic}, nil
	}

	var result []telegram.Topic
	for _, topic := range topics {
		if topic.UnreadCount == 0 {
			continue
		}
		result = append(result, topic)
	}
	return result, nil
}
