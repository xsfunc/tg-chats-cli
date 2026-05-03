package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"
)

func (c *Client) GetUnreadMessages(ctx context.Context, chatID int64, lastReadID int, progress ProgressFunc) ([]Message, error) {
	result, err := c.GetUnreadMessagesWithOptions(ctx, chatID, lastReadID, progress, MessageFetchOptions{})
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

func (c *Client) GetUnreadMessagesWithOptions(ctx context.Context, chatID int64, lastReadID int, progress ProgressFunc, opts MessageFetchOptions) (MessageFetchResult, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return MessageFetchResult{}, fmt.Errorf("peer %d not found in cache or storage", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"unread",
		time.Time{},
		false,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			_ = offsetDate
			req := &tg.MessagesGetHistoryRequest{
				Peer:     inputPeer,
				Limit:    limit,
				OffsetID: offsetID,
			}
			return c.ctx.Raw.MessagesGetHistory(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if msg.ID <= lastReadID {
				return false, true
			}
			if msg.Message == "" || msg.Out {
				return false, false
			}
			return true, false
		},
		opts,
	)
}

func (c *Client) GetTopicMessages(ctx context.Context, chatID int64, topicID int, lastReadID int, progress ProgressFunc) ([]Message, error) {
	result, err := c.GetTopicMessagesWithOptions(ctx, chatID, topicID, lastReadID, progress, MessageFetchOptions{})
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

func (c *Client) GetTopicMessagesWithOptions(ctx context.Context, chatID int64, topicID int, lastReadID int, progress ProgressFunc, opts MessageFetchOptions) (MessageFetchResult, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return MessageFetchResult{}, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"topic-unread",
		time.Time{},
		false,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			_ = offsetDate
			if topicID == 1 {
				req := &tg.MessagesGetHistoryRequest{
					Peer:     inputPeer,
					Limit:    limit,
					OffsetID: offsetID,
				}
				return c.ctx.Raw.MessagesGetHistory(ctx, req)
			}
			req := &tg.MessagesGetRepliesRequest{
				Peer:     inputPeer,
				MsgID:    topicID,
				Limit:    limit,
				OffsetID: offsetID,
			}
			return c.ctx.Raw.MessagesGetReplies(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false
					}
				}
			}
			if msg.ID <= lastReadID {
				return false, true
			}
			if msg.Message == "" || msg.Out {
				return false, false
			}
			return true, false
		},
		opts,
	)
}

func (c *Client) GetMessagesByDate(ctx context.Context, chatID int64, since, until time.Time, progress ProgressFunc) ([]Message, error) {
	result, err := c.GetMessagesByDateWithOptions(ctx, chatID, since, until, progress, MessageFetchOptions{})
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

func (c *Client) GetMessagesByDateWithOptions(ctx context.Context, chatID int64, since, until time.Time, progress ProgressFunc, opts MessageFetchOptions) (MessageFetchResult, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return MessageFetchResult{}, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"date-range",
		until,
		true,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			req := &tg.MessagesGetHistoryRequest{
				Peer:       inputPeer,
				Limit:      limit,
				OffsetID:   offsetID,
				OffsetDate: offsetDate,
			}
			return c.ctx.Raw.MessagesGetHistory(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			msgTime := time.Unix(int64(msg.Date), 0)
			if msgTime.Before(since) {
				return false, true
			}
			if msgTime.After(until) {
				return false, false
			}
			if msg.Message == "" || msg.Out {
				return false, false
			}
			return true, false
		},
		opts,
	)
}

func (c *Client) GetTopicMessagesByDate(ctx context.Context, chatID int64, topicID int, since, until time.Time, progress ProgressFunc) ([]Message, error) {
	result, err := c.GetTopicMessagesByDateWithOptions(ctx, chatID, topicID, since, until, progress, MessageFetchOptions{})
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

func (c *Client) GetTopicMessagesByDateWithOptions(ctx context.Context, chatID int64, topicID int, since, until time.Time, progress ProgressFunc, opts MessageFetchOptions) (MessageFetchResult, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return MessageFetchResult{}, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"topic-date-range",
		until,
		true,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			if topicID == 1 {
				req := &tg.MessagesGetHistoryRequest{
					Peer:       inputPeer,
					Limit:      limit,
					OffsetID:   offsetID,
					OffsetDate: offsetDate,
				}
				return c.ctx.Raw.MessagesGetHistory(ctx, req)
			}
			req := &tg.MessagesGetRepliesRequest{
				Peer:       inputPeer,
				MsgID:      topicID,
				Limit:      limit,
				OffsetID:   offsetID,
				OffsetDate: offsetDate,
			}
			return c.ctx.Raw.MessagesGetReplies(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false
					}
				}
			}
			msgTime := time.Unix(int64(msg.Date), 0)
			if msgTime.Before(since) {
				return false, true
			}
			if msgTime.After(until) {
				return false, false
			}
			if msg.Message == "" || msg.Out {
				return false, false
			}
			return true, false
		},
		opts,
	)
}

func extractMessages(result tg.MessagesMessagesClass) []tg.MessageClass {
	switch r := result.(type) {
	case *tg.MessagesMessages:
		return r.Messages
	case *tg.MessagesMessagesSlice:
		return r.Messages
	case *tg.MessagesChannelMessages:
		return r.Messages
	}
	return nil
}

func extractMessageUsers(result tg.MessagesMessagesClass) []tg.UserClass {
	switch r := result.(type) {
	case *tg.MessagesMessages:
		return r.Users
	case *tg.MessagesMessagesSlice:
		return r.Users
	case *tg.MessagesChannelMessages:
		return r.Users
	}
	return nil
}

func extractMessageChats(result tg.MessagesMessagesClass) []tg.ChatClass {
	switch r := result.(type) {
	case *tg.MessagesMessages:
		return r.Chats
	case *tg.MessagesMessagesSlice:
		return r.Chats
	case *tg.MessagesChannelMessages:
		return r.Chats
	}
	return nil
}

func (c *Client) fetchMessages(
	ctx context.Context,
	progress ProgressFunc,
	phase string,
	until time.Time,
	useOffsetDate bool,
	fetch func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error),
	filter func(msg *tg.Message) (process bool, stop bool),
	opts MessageFetchOptions,
) (MessageFetchResult, error) {
	var allMessages []Message
	var allUsers []User
	var allChats []Chat
	seenUsers := make(map[int64]struct{})
	offsetID := 0
	batchSize := 100
	page := 0
	truncated := false

	for {
		offsetDate := 0
		if useOffsetDate && offsetID == 0 && !until.IsZero() {
			offsetDate = int(until.Unix())
			reportProgress(progress, ProgressUpdate{
				Phase: fmt.Sprintf("jumped to date %s", until.Format("2006-01-02")),
			})
		}

		if page > 0 && c.historyPacer != nil {
			if err := c.historyPacer.Wait(ctx, progress); err != nil {
				return MessageFetchResult{}, fmt.Errorf("history request pause: %w", err)
			}
		}

		result, err := fetch(offsetID, offsetDate, batchSize)
		if err != nil {
			return MessageFetchResult{}, err
		}
		if c.historyPacer != nil {
			c.historyPacer.RecordSuccess()
		}
		page++
		allUsers = appendUsers(allUsers, seenUsers, extractMessageUsers(result))
		c.processPeerEntities(extractMessageChats(result), extractMessageUsers(result))
		allChats = append(allChats, c.chatsFromEntities(extractMessageChats(result), extractMessageUsers(result))...)

		msgs := extractMessages(result)
		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := processMessageBatch(msgs, filter)
		if opts.MessageLimit > 0 && len(allMessages)+len(batchMessages) >= opts.MessageLimit {
			remaining := opts.MessageLimit - len(allMessages)
			if remaining < len(batchMessages) {
				batchMessages = batchMessages[:remaining]
				truncated = true
			}
			allMessages = append(allMessages, batchMessages...)
			reportProgress(progress, ProgressUpdate{
				Phase:   phase,
				Parsed:  len(batchMessages),
				Scanned: len(msgs),
				Batch:   1,
			})
			if len(allMessages) >= opts.MessageLimit {
				truncated = true
				break
			}
		} else {
			allMessages = append(allMessages, batchMessages...)
			reportProgress(progress, ProgressUpdate{
				Phase:   phase,
				Parsed:  len(batchMessages),
				Scanned: len(msgs),
				Batch:   1,
			})
		}

		if stop {
			break
		}
		if lastID == 0 {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return MessageFetchResult{Messages: allMessages, Users: allUsers, Chats: dedupeChats(allChats), Truncated: truncated}, nil
}

func processMessageBatch(msgs []tg.MessageClass,
	filter func(msg *tg.Message) (process bool, stop bool)) ([]Message, int, bool) {

	var results []Message
	var lastID int
	var stopLoop bool

	for _, m := range msgs {
		lastID = m.GetID()

		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}

		process, stop := filter(msg)
		if stop {
			stopLoop = true
			break
		}
		if !process {
			continue
		}

		senderID := resolveSenderID(msg.FromID)
		editDate := time.Time{}
		if msg.EditDate != 0 {
			editDate = time.Unix(int64(msg.EditDate), 0)
		}
		results = append(results, Message{
			ID:       msg.ID,
			Date:     time.Unix(int64(msg.Date), 0),
			Text:     msg.Message,
			SenderID: senderID,
			Outgoing: msg.Out,
			EditDate: editDate,
		})
	}
	return results, lastID, stopLoop
}
