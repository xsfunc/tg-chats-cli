package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

func (c *Client) MarkAsRead(ctx context.Context, chat Chat, maxID int) error {
	inputPeer := c.resolvePeer(chat.ID)
	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chat.ID)
	}

	if chat.IsChannel {
		inputChannel, ok := inputPeer.(*tg.InputPeerChannel)
		if !ok {
			return fmt.Errorf("peer is marked as channel but input peer is %T", inputPeer)
		}

		_, err := c.ctx.Raw.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{
				ChannelID:  inputChannel.ChannelID,
				AccessHash: inputChannel.AccessHash,
			},
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark channel as read: %w", err)
		}
	} else {
		_, err := c.ctx.Raw.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
			Peer:  inputPeer,
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark chat as read: %w", err)
		}
	}

	return nil
}

func (c *Client) MarkTopicAsRead(ctx context.Context, chatID int64, topicID int, maxID int) error {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chatID)
	}

	_, err := c.ctx.Raw.MessagesReadDiscussion(ctx, &tg.MessagesReadDiscussionRequest{
		Peer:      inputPeer,
		MsgID:     topicID,
		ReadMaxID: maxID,
	})
	if err != nil {
		return fmt.Errorf("failed to mark topic as read: %w", err)
	}

	return nil
}
