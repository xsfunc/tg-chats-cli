package telegram

import (
	"context"
	"fmt"
	"sort"

	"github.com/gotd/td/tg"
)

func (c *Client) GetDialogs(ctx context.Context) ([]Chat, error) {
	result, err := c.GetDialogsWithEntities(ctx)
	if err != nil {
		return nil, err
	}
	return result.Chats, nil
}

func (c *Client) GetDialogsWithEntities(ctx context.Context) (DialogsResult, error) {
	if c.ctx == nil {
		c.ctx = c.proto.CreateContext()
	}

	const limit = 100
	var parsedDialogs []Chat
	var users []User
	seen := make(map[int64]struct{})
	seenUsers := make(map[int64]struct{})

	offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
	offsetID := 0
	offsetDate := 0

	for {
		if c.historyPacer != nil {
			if err := c.historyPacer.Wait(ctx, nil); err != nil {
				return DialogsResult{}, fmt.Errorf("dialog request pause: %w", err)
			}
		}

		dialogs, err := c.ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      limit,
			OffsetPeer: offsetPeer,
			OffsetID:   offsetID,
			OffsetDate: offsetDate,
		})
		if err != nil {
			return DialogsResult{}, fmt.Errorf("failed to get dialogs: %w", err)
		}
		if c.historyPacer != nil {
			c.historyPacer.RecordSuccess()
		}

		var batch []Chat
		var lastDialog *tg.Dialog
		var lastMessages []tg.MessageClass

		switch d := dialogs.(type) {
		case *tg.MessagesDialogsSlice:
			batch = c.processDialogs(d.Dialogs, d.Chats, d.Users)
			users = appendUsers(users, seenUsers, d.Users)
			lastMessages = d.Messages
			if len(d.Dialogs) > 0 {
				if dlg, ok := d.Dialogs[len(d.Dialogs)-1].(*tg.Dialog); ok {
					lastDialog = dlg
				}
			}
		case *tg.MessagesDialogs:
			batch = c.processDialogs(d.Dialogs, d.Chats, d.Users)
			users = appendUsers(users, seenUsers, d.Users)
			lastMessages = d.Messages
			if len(d.Dialogs) > 0 {
				if dlg, ok := d.Dialogs[len(d.Dialogs)-1].(*tg.Dialog); ok {
					lastDialog = dlg
				}
			}
		}

		for _, chat := range batch {
			if _, ok := seen[chat.ID]; ok {
				continue
			}
			seen[chat.ID] = struct{}{}
			parsedDialogs = append(parsedDialogs, chat)
		}

		if len(batch) < limit || lastDialog == nil {
			break
		}

		peerID := resolveSenderID(lastDialog.Peer)
		nextPeer, ok := c.peerCache[peerID]
		if !ok {
			nextPeer = c.ctx.PeerStorage.GetInputPeerById(peerID)
		}
		if nextPeer == nil {
			break
		}
		nextOffsetDate := messageDateByID(lastMessages, lastDialog.TopMessage)
		if nextOffsetDate == 0 {
			// Cannot determine the pagination date; stop to avoid an incorrect offset.
			break
		}
		offsetPeer = nextPeer
		offsetID = lastDialog.TopMessage
		offsetDate = nextOffsetDate
	}

	sort.Slice(parsedDialogs, func(i, j int) bool {
		return parsedDialogs[i].UnreadCount > parsedDialogs[j].UnreadCount
	})

	return DialogsResult{Chats: parsedDialogs, Users: users}, nil
}

func (c *Client) processDialogs(dialogs []tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass) []Chat {
	c.processPeerEntities(chats, users)
	chatMap := make(map[int64]tg.ChatClass)
	for _, ch := range chats {
		chatMap[ch.GetID()] = ch
	}
	userMap := make(map[int64]tg.UserClass)
	for _, u := range users {
		userMap[u.GetID()] = u
	}

	var results []Chat

	for _, d := range dialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}

		var title string
		var peerID int64
		var isChannel bool
		var isForum bool
		var isUser bool
		var isBot bool

		switch p := dlg.Peer.(type) {
		case *tg.PeerUser:
			peerID = p.UserID
			isUser = true
			if u, ok := userMap[peerID]; ok {
				if user, ok := u.(*tg.User); ok {
					title = user.FirstName + " " + user.LastName
					if user.Username != "" {
						title += " (@" + user.Username + ")"
					}
					isBot = user.Bot
				}
			}
		case *tg.PeerChat:
			peerID = p.ChatID
			if ch, ok := chatMap[peerID]; ok {
				if chat, ok := ch.(*tg.Chat); ok {
					title = chat.Title
				}
			}
		case *tg.PeerChannel:
			peerID = p.ChannelID
			isChannel = true
			if ch, ok := chatMap[peerID]; ok {
				if channel, ok := ch.(*tg.Channel); ok {
					title = channel.Title
					isForum = channel.Forum
				}
			}
		}

		if title == "" {
			title = fmt.Sprintf("Unknown Peer %d", peerID)
		}

		results = append(results, Chat{
			ID:           peerID,
			Title:        title,
			UnreadCount:  dlg.UnreadCount,
			IsChannel:    isChannel,
			IsForum:      isForum,
			IsUser:       isUser,
			IsBot:        isBot,
			LastReadID:   dlg.ReadInboxMaxID,
			TopMessageID: dlg.TopMessage,
		})
	}
	return results
}
