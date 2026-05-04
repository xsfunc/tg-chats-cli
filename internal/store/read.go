package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AccountSummary struct {
	ID                int64
	TelegramUserID    int64
	HasTelegramUserID bool
	Username          string
	FirstName         string
	LastName          string
	Phone             string
	IsBot             bool
}

type ChatSummary struct {
	AccountID    int64
	ID           int64
	Title        string
	Kind         string
	IsForum      bool
	MessageCount int
	FirstMessage time.Time
	LastMessage  time.Time
}

type TopicSummary struct {
	AccountID    int64
	ChatID       int64
	ID           int
	Title        string
	MessageCount int
	FirstMessage time.Time
	LastMessage  time.Time
}

type MessageQuery struct {
	AccountID  int64
	ChatID     int64
	TopicID    int
	TopicIDSet bool
	Since      time.Time
	Until      time.Time
	UseSince   bool
	UseUntil   bool
}

type ExportData struct {
	Account  AccountSummary
	Chat     ChatSummary
	Topic    TopicSummary
	HasTopic bool
	Messages []ExportMessage
}

type ExportMessage struct {
	ID              int
	Date            time.Time
	Text            string
	SenderID        int64
	Outgoing        bool
	SenderUsername  string
	SenderFirstName string
	SenderLastName  string
}

func (s *SQLiteStore) ListAccounts(ctx context.Context) ([]AccountSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, telegram_user_id, username, first_name, last_name, phone, is_bot
		FROM accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer closeRows(rows)

	var accounts []AccountSummary
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts rows: %w", err)
	}
	return accounts, nil
}

func (s *SQLiteStore) ResolveAccountID(ctx context.Context, requestedID int64) (int64, error) {
	if requestedID < 0 {
		return 0, fmt.Errorf("--account-id cannot be negative")
	}
	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		return 0, err
	}
	if requestedID != 0 {
		for _, account := range accounts {
			if account.ID == requestedID {
				return requestedID, nil
			}
		}
		return 0, fmt.Errorf("account_id %d not found", requestedID)
	}
	if len(accounts) == 0 {
		return 0, fmt.Errorf("no accounts found in SQLite database")
	}
	if len(accounts) > 1 {
		var ids []string
		for _, account := range accounts {
			ids = append(ids, fmt.Sprintf("%d", account.ID))
		}
		return 0, fmt.Errorf("multiple accounts found; pass --account-id (available: %s)", strings.Join(ids, ", "))
	}
	return accounts[0].ID, nil
}

func (s *SQLiteStore) GetAccount(ctx context.Context, accountID int64) (AccountSummary, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, telegram_user_id, username, first_name, last_name, phone, is_bot
		FROM accounts WHERE id = ?`, accountID)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountSummary{}, fmt.Errorf("account_id %d not found", accountID)
	}
	if err != nil {
		return AccountSummary{}, err
	}
	return account, nil
}

func (s *SQLiteStore) ListChatSummaries(ctx context.Context, accountID int64) ([]ChatSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.account_id, c.id, c.title, c.kind, c.is_forum,
			COUNT(m.message_id), MIN(m.date_unix), MAX(m.date_unix)
		FROM chats c
		LEFT JOIN messages m ON m.account_id = c.account_id AND m.chat_id = c.id
		WHERE c.account_id = ?
		GROUP BY c.account_id, c.id, c.title, c.kind, c.is_forum
		ORDER BY COALESCE(MAX(m.date_unix), 0) DESC, c.title COLLATE NOCASE`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer closeRows(rows)

	var chats []ChatSummary
	for rows.Next() {
		chat, err := scanChatSummary(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list chats rows: %w", err)
	}
	return chats, nil
}

func (s *SQLiteStore) ListTopicSummaries(ctx context.Context, accountID int64, chatID int64) ([]TopicSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.account_id, t.chat_id, t.topic_id, t.title,
			COUNT(m.message_id), MIN(m.date_unix), MAX(m.date_unix)
		FROM topics t
		LEFT JOIN messages m ON m.account_id = t.account_id AND m.chat_id = t.chat_id AND m.topic_id = t.topic_id
		WHERE t.account_id = ? AND t.chat_id = ?
		GROUP BY t.account_id, t.chat_id, t.topic_id, t.title
		ORDER BY COALESCE(MAX(m.date_unix), 0) DESC, t.title COLLATE NOCASE`, accountID, chatID)
	if err != nil {
		return nil, fmt.Errorf("list topics for chat %d: %w", chatID, err)
	}
	defer closeRows(rows)

	var topics []TopicSummary
	for rows.Next() {
		topic, err := scanTopicSummary(rows)
		if err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list topics rows for chat %d: %w", chatID, err)
	}
	return topics, nil
}

func (s *SQLiteStore) ExportMessages(ctx context.Context, query MessageQuery) (ExportData, error) {
	account, err := s.GetAccount(ctx, query.AccountID)
	if err != nil {
		return ExportData{}, err
	}
	chat, err := s.getChatSummary(ctx, query.AccountID, query.ChatID)
	if err != nil {
		return ExportData{}, err
	}
	data := ExportData{
		Account: account,
		Chat:    chat,
	}
	if query.TopicIDSet {
		topic, found, err := s.getTopicSummary(ctx, query.AccountID, query.ChatID, query.TopicID)
		if err != nil {
			return ExportData{}, err
		}
		if found {
			data.Topic = topic
		} else {
			data.Topic = TopicSummary{
				AccountID: query.AccountID,
				ChatID:    query.ChatID,
				ID:        query.TopicID,
			}
		}
		data.HasTopic = true
	}

	messages, err := s.selectMessages(ctx, query)
	if err != nil {
		return ExportData{}, err
	}
	data.Messages = messages
	return data, nil
}

func (s *SQLiteStore) getChatSummary(ctx context.Context, accountID int64, chatID int64) (ChatSummary, error) {
	row := s.db.QueryRowContext(ctx, `SELECT c.account_id, c.id, c.title, c.kind, c.is_forum,
			COUNT(m.message_id), MIN(m.date_unix), MAX(m.date_unix)
		FROM chats c
		LEFT JOIN messages m ON m.account_id = c.account_id AND m.chat_id = c.id
		WHERE c.account_id = ? AND c.id = ?
	GROUP BY c.account_id, c.id, c.title, c.kind, c.is_forum`, accountID, chatID)
	chat, err := scanChatSummary(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatSummary{}, fmt.Errorf("chat_id %d not found for account_id %d", chatID, accountID)
	}
	if err != nil {
		return ChatSummary{}, err
	}
	return chat, nil
}

func (s *SQLiteStore) getTopicSummary(ctx context.Context, accountID int64, chatID int64, topicID int) (TopicSummary, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT t.account_id, t.chat_id, t.topic_id, t.title,
			COUNT(m.message_id), MIN(m.date_unix), MAX(m.date_unix)
		FROM topics t
		LEFT JOIN messages m ON m.account_id = t.account_id AND m.chat_id = t.chat_id AND m.topic_id = t.topic_id
		WHERE t.account_id = ? AND t.chat_id = ? AND t.topic_id = ?
		GROUP BY t.account_id, t.chat_id, t.topic_id, t.title`, accountID, chatID, topicID)
	topic, err := scanTopicSummary(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TopicSummary{}, false, nil
	}
	if err != nil {
		return TopicSummary{}, false, err
	}
	return topic, true, nil
}

func (s *SQLiteStore) selectMessages(ctx context.Context, query MessageQuery) ([]ExportMessage, error) {
	conditions := []string{"m.account_id = ?", "m.chat_id = ?"}
	args := []any{query.AccountID, query.ChatID}
	if query.TopicIDSet {
		conditions = append(conditions, "m.topic_id = ?")
		args = append(args, query.TopicID)
	}
	if query.UseSince {
		conditions = append(conditions, "m.date_unix >= ?")
		args = append(args, query.Since.Unix())
	}
	if query.UseUntil {
		conditions = append(conditions, "m.date_unix <= ?")
		args = append(args, query.Until.Unix())
	}
	sqlText := `SELECT m.message_id, m.date_unix, m.sender_id, m.text, m.outgoing,
			COALESCE(u.username, ''), COALESCE(u.first_name, ''), COALESCE(u.last_name, '')
		FROM messages m
		LEFT JOIN users u ON u.account_id = m.account_id AND u.id = m.sender_id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY m.date_unix ASC, m.message_id ASC`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("select export messages: %w", err)
	}
	defer closeRows(rows)

	var messages []ExportMessage
	for rows.Next() {
		var msg ExportMessage
		var unix int64
		var outgoing int
		if err := rows.Scan(&msg.ID, &unix, &msg.SenderID, &msg.Text, &outgoing, &msg.SenderUsername, &msg.SenderFirstName, &msg.SenderLastName); err != nil {
			return nil, fmt.Errorf("scan export message: %w", err)
		}
		msg.Date = time.Unix(unix, 0).UTC()
		msg.Outgoing = outgoing != 0
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select export messages rows: %w", err)
	}
	return messages, nil
}

type accountScanner interface {
	Scan(dest ...any) error
}

func scanAccount(scanner accountScanner) (AccountSummary, error) {
	var account AccountSummary
	var telegramUserID sql.NullInt64
	var isBot int
	if err := scanner.Scan(&account.ID, &telegramUserID, &account.Username, &account.FirstName, &account.LastName, &account.Phone, &isBot); err != nil {
		return AccountSummary{}, fmt.Errorf("scan account: %w", err)
	}
	account.TelegramUserID = telegramUserID.Int64
	account.HasTelegramUserID = telegramUserID.Valid
	account.IsBot = isBot != 0
	return account, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanChatSummary(scanner rowScanner) (ChatSummary, error) {
	var chat ChatSummary
	var isForum int
	var firstUnix sql.NullInt64
	var lastUnix sql.NullInt64
	if err := scanner.Scan(&chat.AccountID, &chat.ID, &chat.Title, &chat.Kind, &isForum, &chat.MessageCount, &firstUnix, &lastUnix); err != nil {
		return ChatSummary{}, fmt.Errorf("scan chat summary: %w", err)
	}
	chat.IsForum = isForum != 0
	chat.FirstMessage = unixOrZero(firstUnix)
	chat.LastMessage = unixOrZero(lastUnix)
	return chat, nil
}

func scanTopicSummary(scanner rowScanner) (TopicSummary, error) {
	var topic TopicSummary
	var firstUnix sql.NullInt64
	var lastUnix sql.NullInt64
	if err := scanner.Scan(&topic.AccountID, &topic.ChatID, &topic.ID, &topic.Title, &topic.MessageCount, &firstUnix, &lastUnix); err != nil {
		return TopicSummary{}, fmt.Errorf("scan topic summary: %w", err)
	}
	topic.FirstMessage = unixOrZero(firstUnix)
	topic.LastMessage = unixOrZero(lastUnix)
	return topic, nil
}

func unixOrZero(value sql.NullInt64) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return time.Unix(value.Int64, 0).UTC()
}
