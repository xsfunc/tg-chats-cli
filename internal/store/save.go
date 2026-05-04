package store

import (
	"context"
	"fmt"
	"time"

	"tg-arc/internal/telegram"
)

func (s *SQLiteStore) SaveUsers(ctx context.Context, users []telegram.User) error {
	if len(users) == 0 {
		return nil
	}
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save users: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO users (account_id, id, username, first_name, last_name, is_bot, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, id) DO UPDATE SET
			username=excluded.username,
			first_name=excluded.first_name,
			last_name=excluded.last_name,
			is_bot=excluded.is_bot,
			updated_at=excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare save users: %w", err)
	}
	defer closeStmt(stmt)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, user := range users {
		if user.ID == 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, accountID, user.ID, user.Username, user.FirstName, user.LastName, boolInt(user.IsBot), now); err != nil {
			return fmt.Errorf("save user %d: %w", user.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save users: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveChats(ctx context.Context, chats []telegram.Chat) error {
	if len(chats) == 0 {
		return nil
	}
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save chats: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO chats
		(account_id, id, title, kind, is_channel, is_forum, is_user, is_bot, unread_count, last_read_id, top_message_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, id) DO UPDATE SET
			title=excluded.title,
			kind=excluded.kind,
			is_channel=excluded.is_channel,
			is_forum=excluded.is_forum,
			is_user=excluded.is_user,
			is_bot=excluded.is_bot,
			unread_count=excluded.unread_count,
			last_read_id=excluded.last_read_id,
			top_message_id=excluded.top_message_id,
			updated_at=excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare save chats: %w", err)
	}
	defer closeStmt(stmt)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, chat := range chats {
		if chat.ID == 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx,
			accountID,
			chat.ID,
			chat.Title,
			chatKind(chat),
			boolInt(chat.IsChannel),
			boolInt(chat.IsForum),
			boolInt(chat.IsUser),
			boolInt(chat.IsBot),
			chat.UnreadCount,
			chat.LastReadID,
			chat.TopMessageID,
			now,
		); err != nil {
			return fmt.Errorf("save chat %d: %w", chat.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save chats: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveTopics(ctx context.Context, chatID int64, topics []telegram.Topic) error {
	if len(topics) == 0 {
		return nil
	}
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save topics: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO topics
		(account_id, chat_id, topic_id, title, unread_count, last_read_id, top_message_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, chat_id, topic_id) DO UPDATE SET
			title=excluded.title,
			unread_count=excluded.unread_count,
			last_read_id=excluded.last_read_id,
			top_message_id=excluded.top_message_id,
			updated_at=excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare save topics: %w", err)
	}
	defer closeStmt(stmt)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, topic := range topics {
		if topic.ID == 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, accountID, chatID, topic.ID, topic.Title, topic.UnreadCount, topic.LastReadID, topic.TopMessageID, now); err != nil {
			return fmt.Errorf("save topic %d/%d: %w", chatID, topic.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save topics: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveMessages(ctx context.Context, chatID int64, topicID int, messages []telegram.Message) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	accountID, err := s.requireAccount()
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin save messages: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO messages
		(account_id, chat_id, topic_id, message_id, date_unix, sender_id, text, outgoing, edit_date_unix, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, chat_id, topic_id, message_id) DO UPDATE SET
			date_unix=excluded.date_unix,
			sender_id=excluded.sender_id,
			text=excluded.text,
			outgoing=excluded.outgoing,
			edit_date_unix=excluded.edit_date_unix,
			saved_at=excluded.saved_at`)
	if err != nil {
		return 0, fmt.Errorf("prepare save messages: %w", err)
	}
	defer closeStmt(stmt)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, msg := range messages {
		if msg.ID == 0 {
			continue
		}
		editUnix := int64(0)
		if !msg.EditDate.IsZero() {
			editUnix = msg.EditDate.Unix()
		}
		if _, err := stmt.ExecContext(ctx,
			accountID,
			chatID,
			topicID,
			msg.ID,
			msg.Date.Unix(),
			msg.SenderID,
			msg.Text,
			boolInt(msg.Outgoing),
			editUnix,
			now,
		); err != nil {
			return 0, fmt.Errorf("save message %d/%d/%d: %w", chatID, topicID, msg.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit save messages: %w", err)
	}
	return len(messages), nil
}
