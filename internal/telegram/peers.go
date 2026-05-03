package telegram

import (
	"fmt"

	"github.com/gotd/td/tg"
)

func (c *Client) processPeerEntities(chats []tg.ChatClass, users []tg.UserClass) {
	for _, ch := range chats {
		switch item := ch.(type) {
		case *tg.Chat:
			c.peerCache[item.ID] = &tg.InputPeerChat{ChatID: item.ID}
		case *tg.Channel:
			c.peerCache[item.ID] = &tg.InputPeerChannel{ChannelID: item.ID, AccessHash: item.AccessHash}
			c.channelCache[item.ID] = item
		}
	}
	for _, u := range users {
		if item, ok := u.(*tg.User); ok {
			c.peerCache[item.ID] = &tg.InputPeerUser{UserID: item.ID, AccessHash: item.AccessHash}
		}
	}
}

func (c *Client) chatsFromEntities(chats []tg.ChatClass, users []tg.UserClass) []Chat {
	var result []Chat
	for _, ch := range chats {
		switch item := ch.(type) {
		case *tg.Chat:
			result = append(result, Chat{
				ID:    item.ID,
				Title: item.Title,
			})
		case *tg.Channel:
			result = append(result, Chat{
				ID:        item.ID,
				Title:     item.Title,
				IsChannel: true,
				IsForum:   item.Forum,
			})
		}
	}
	for _, u := range users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		title := user.FirstName + " " + user.LastName
		if user.Username != "" {
			title += " (@" + user.Username + ")"
		}
		if title == " " {
			title = fmt.Sprintf("Unknown Peer %d", user.ID)
		}
		result = append(result, Chat{
			ID:     user.ID,
			Title:  title,
			IsUser: true,
			IsBot:  user.Bot,
		})
	}
	return result
}

func dedupeChats(chats []Chat) []Chat {
	result := make([]Chat, 0, len(chats))
	seen := make(map[int64]struct{})
	for _, chat := range chats {
		if chat.ID == 0 {
			continue
		}
		if _, ok := seen[chat.ID]; ok {
			continue
		}
		seen[chat.ID] = struct{}{}
		result = append(result, chat)
	}
	return result
}

func (c *Client) resolvePeer(chatID int64) tg.InputPeerClass {
	if inputPeer, ok := c.peerCache[chatID]; ok {
		return inputPeer
	}
	if c.ctx == nil || c.ctx.PeerStorage == nil {
		return nil
	}
	return c.ctx.PeerStorage.GetInputPeerById(chatID)
}

func resolveSenderID(fromID tg.PeerClass) int64 {
	if fromID == nil {
		return 0
	}
	switch p := fromID.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChannel:
		return p.ChannelID
	case *tg.PeerChat:
		return p.ChatID
	default:
		return 0
	}
}

func appendUsers(result []User, seen map[int64]struct{}, users []tg.UserClass) []User {
	for _, item := range users {
		user, ok := item.(*tg.User)
		if !ok {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		result = append(result, User{
			ID:        user.ID,
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			IsBot:     user.Bot,
		})
	}
	return result
}
