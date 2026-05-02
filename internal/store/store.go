package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cli-tg-chat-summary/internal/telegram"

	_ "github.com/glebarez/go-sqlite"
)

const schemaVersion = 1

type Store struct {
	db *sql.DB
}

type SyncRun struct {
	ID        int64
	Mode      string
	StartedAt time.Time
}

type RunItem struct {
	RunID          int64
	ChatID         int64
	TopicID        int
	SavedMessages  int
	MarkReadStatus string
	Warning        string
	Error          string
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

const DefaultPath = "data/tg-summary.db"

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			is_bot INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chats (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			kind TEXT NOT NULL,
			is_channel INTEGER NOT NULL DEFAULT 0,
			is_forum INTEGER NOT NULL DEFAULT 0,
			is_user INTEGER NOT NULL DEFAULT 0,
			is_bot INTEGER NOT NULL DEFAULT 0,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			top_message_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS topics (
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			top_message_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (chat_id, topic_id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL DEFAULT 0,
			message_id INTEGER NOT NULL,
			date_unix INTEGER NOT NULL,
			sender_id INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL,
			outgoing INTEGER NOT NULL DEFAULT 0,
			edit_date_unix INTEGER NOT NULL DEFAULT 0,
			saved_at TEXT NOT NULL,
			PRIMARY KEY (chat_id, topic_id, message_id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mode TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sync_run_items (
			run_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL DEFAULT 0,
			saved_messages INTEGER NOT NULL DEFAULT 0,
			mark_read_status TEXT NOT NULL DEFAULT '',
			warning TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (run_id) REFERENCES sync_runs(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat_date ON messages(chat_id, topic_id, date_unix)`,
		fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion),
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveUsers(ctx context.Context, users []telegram.User) error {
	if len(users) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save users: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO users (id, username, first_name, last_name, is_bot, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
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
		if _, err := stmt.ExecContext(ctx, user.ID, user.Username, user.FirstName, user.LastName, boolInt(user.IsBot), now); err != nil {
			return fmt.Errorf("save user %d: %w", user.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save users: %w", err)
	}
	return nil
}

func (s *Store) SaveChats(ctx context.Context, chats []telegram.Chat) error {
	if len(chats) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save chats: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO chats
		(id, title, kind, is_channel, is_forum, is_user, is_bot, unread_count, last_read_id, top_message_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
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

func (s *Store) SaveTopics(ctx context.Context, chatID int64, topics []telegram.Topic) error {
	if len(topics) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save topics: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO topics
		(chat_id, topic_id, title, unread_count, last_read_id, top_message_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, topic_id) DO UPDATE SET
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
		if _, err := stmt.ExecContext(ctx, chatID, topic.ID, topic.Title, topic.UnreadCount, topic.LastReadID, topic.TopMessageID, now); err != nil {
			return fmt.Errorf("save topic %d/%d: %w", chatID, topic.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save topics: %w", err)
	}
	return nil
}

func (s *Store) SaveMessages(ctx context.Context, chatID int64, topicID int, messages []telegram.Message) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin save messages: %w", err)
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO messages
		(chat_id, topic_id, message_id, date_unix, sender_id, text, outgoing, edit_date_unix, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, topic_id, message_id) DO UPDATE SET
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

func (s *Store) StartRun(ctx context.Context, mode string) (SyncRun, error) {
	started := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO sync_runs (mode, started_at, status) VALUES (?, ?, ?)`, mode, started.Format(time.RFC3339Nano), "running")
	if err != nil {
		return SyncRun{}, fmt.Errorf("start sync run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SyncRun{}, fmt.Errorf("read sync run id: %w", err)
	}
	return SyncRun{ID: id, Mode: mode, StartedAt: started}, nil
}

func (s *Store) FinishRun(ctx context.Context, runID int64, status string, runErr error) error {
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sync_runs SET finished_at = ?, status = ?, error = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), status, errText, runID); err != nil {
		return fmt.Errorf("finish sync run %d: %w", runID, err)
	}
	return nil
}

func (s *Store) AddRunItem(ctx context.Context, item RunItem) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sync_run_items
		(run_id, chat_id, topic_id, saved_messages, mark_read_status, warning, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.RunID,
		item.ChatID,
		item.TopicID,
		item.SavedMessages,
		item.MarkReadStatus,
		item.Warning,
		item.Error,
	); err != nil {
		return fmt.Errorf("add sync run item: %w", err)
	}
	return nil
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func closeStmt(stmt *sql.Stmt) {
	_ = stmt.Close()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func chatKind(chat telegram.Chat) string {
	switch {
	case chat.IsUser:
		return "user"
	case chat.IsChannel:
		return "channel"
	default:
		return "chat"
	}
}
