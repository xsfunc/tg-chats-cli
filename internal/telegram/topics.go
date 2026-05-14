package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

func (c *Client) GetForumTopics(ctx context.Context, chatID int64) ([]Topic, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	const limit = 100
	var result []Topic
	seen := make(map[int]struct{})
	offset := forumTopicOffset{}

	for {
		if c.historyPacer != nil {
			if err := c.historyPacer.Wait(ctx, nil); err != nil {
				return nil, fmt.Errorf("forum topics request pause: %w", err)
			}
		}

		topics, err := c.ctx.Raw.MessagesGetForumTopics(ctx, &tg.MessagesGetForumTopicsRequest{
			Peer:        inputPeer,
			OffsetDate:  offset.date,
			OffsetID:    offset.id,
			OffsetTopic: offset.topic,
			Limit:       limit,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get forum topics: %w", err)
		}
		if c.historyPacer != nil {
			c.historyPacer.RecordSuccess()
		}

		result = appendForumTopics(result, seen, topics.Topics)
		nextOffset, ok := nextForumTopicOffset(topics)
		if !ok || len(topics.Topics) < limit || nextOffset == offset {
			break
		}
		offset = nextOffset
	}

	return result, nil
}

type forumTopicOffset struct {
	date  int
	id    int
	topic int
}

func appendForumTopics(result []Topic, seen map[int]struct{}, topics []tg.ForumTopicClass) []Topic {
	for _, t := range topics {
		topic, ok := t.(*tg.ForumTopic)
		if !ok {
			continue
		}
		if _, ok := seen[topic.ID]; ok {
			continue
		}
		seen[topic.ID] = struct{}{}
		result = append(result, Topic{
			ID:           topic.ID,
			Title:        topic.Title,
			UnreadCount:  topic.UnreadCount,
			LastReadID:   topic.ReadInboxMaxID,
			TopMessageID: topic.TopMessage,
		})
	}
	return result
}

func nextForumTopicOffset(topics *tg.MessagesForumTopics) (forumTopicOffset, bool) {
	for i := len(topics.Topics) - 1; i >= 0; i-- {
		topic, ok := topics.Topics[i].(*tg.ForumTopic)
		if !ok {
			continue
		}
		offsetDate := topic.Date
		if !topics.OrderByCreateDate {
			if date := messageDateByID(topics.Messages, topic.TopMessage); date != 0 {
				offsetDate = date
			}
		}
		return forumTopicOffset{
			date:  offsetDate,
			id:    topic.TopMessage,
			topic: topic.ID,
		}, true
	}
	return forumTopicOffset{}, false
}

func messageDateByID(messages []tg.MessageClass, id int) int {
	for _, m := range messages {
		if m.GetID() != id {
			continue
		}
		switch msg := m.(type) {
		case *tg.Message:
			return msg.Date
		case *tg.MessageService:
			return msg.Date
		}
	}
	return 0
}
