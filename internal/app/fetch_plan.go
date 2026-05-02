package app

import (
	"context"
	"fmt"

	"cli-tg-chat-summary/internal/telegram"
)

type fetchPlan struct {
	progressTitle string
	exportTitle   string
	fetch         func(context.Context, telegram.ProgressFunc) (telegram.MessageFetchResult, error)
}

func (a *App) buildFetchPlan(selectedChat telegram.Chat, selectedTopic *telegram.Topic, opts RunOptions) (fetchPlan, error) {
	if selectedChat.IsForum {
		if selectedTopic == nil {
			return fetchPlan{}, fmt.Errorf("forum chat requires --topic-id or --topic")
		}
		if opts.UseDateRange {
			progressTitle := fmt.Sprintf("%s / %s (%s to %s)", selectedChat.Title, selectedTopic.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
			return fetchPlan{
				progressTitle: progressTitle,
				exportTitle:   selectedChat.Title + " - " + selectedTopic.Title,
				fetch: func(ctx context.Context, progress telegram.ProgressFunc) (telegram.MessageFetchResult, error) {
					return a.tgClient.GetTopicMessagesByDateWithOptions(ctx, selectedChat.ID, selectedTopic.ID, opts.Since, opts.Until, progress, telegram.MessageFetchOptions{
						MessageLimit: opts.MessageLimit,
					})
				},
			}, nil
		}
		progressTitle := fmt.Sprintf("%s / %s (unread)", selectedChat.Title, selectedTopic.Title)
		return fetchPlan{
			progressTitle: progressTitle,
			exportTitle:   selectedChat.Title + " - " + selectedTopic.Title,
			fetch: func(ctx context.Context, progress telegram.ProgressFunc) (telegram.MessageFetchResult, error) {
				return a.tgClient.GetTopicMessagesWithOptions(ctx, selectedChat.ID, selectedTopic.ID, selectedTopic.LastReadID, progress, telegram.MessageFetchOptions{
					MessageLimit: opts.MessageLimit,
				})
			},
		}, nil
	}

	if opts.UseDateRange {
		progressTitle := fmt.Sprintf("%s (%s to %s)", selectedChat.Title, opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
		return fetchPlan{
			progressTitle: progressTitle,
			exportTitle:   selectedChat.Title,
			fetch: func(ctx context.Context, progress telegram.ProgressFunc) (telegram.MessageFetchResult, error) {
				return a.tgClient.GetMessagesByDateWithOptions(ctx, selectedChat.ID, opts.Since, opts.Until, progress, telegram.MessageFetchOptions{
					MessageLimit: opts.MessageLimit,
				})
			},
		}, nil
	}
	progressTitle := fmt.Sprintf("%s (unread)", selectedChat.Title)
	return fetchPlan{
		progressTitle: progressTitle,
		exportTitle:   selectedChat.Title,
		fetch: func(ctx context.Context, progress telegram.ProgressFunc) (telegram.MessageFetchResult, error) {
			return a.tgClient.GetUnreadMessagesWithOptions(ctx, selectedChat.ID, selectedChat.LastReadID, progress, telegram.MessageFetchOptions{
				MessageLimit: opts.MessageLimit,
			})
		},
	}, nil
}
