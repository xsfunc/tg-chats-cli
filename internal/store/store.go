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

const schemaVersion = 2

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

type Store interface {
	Close() error
	SetAccount(ctx context.Context, account telegram.Account) error
	SaveUsers(ctx context.Context, users []telegram.User) error
	SaveChats(ctx context.Context, chats []telegram.Chat) error
	SaveTopics(ctx context.Context, chatID int64, topics []telegram.Topic) error
	SaveMessages(ctx context.Context, chatID int64, topicID int, messages []telegram.Message) (int, error)
	StartRun(ctx context.Context, mode string) (SyncRun, error)
	FinishRun(ctx context.Context, runID int64, status string, runErr error) error
	AddRunItem(ctx context.Context, item RunItem) error
}

type SQLiteStore struct {
	db        *sql.DB
	accountID int64
}

var _ Store = (*SQLiteStore)(nil)

func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	return OpenSQLite(ctx, path)
}

func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
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
	s := &SQLiteStore{db: db}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

const DefaultPath = "data/tg-summary.db"

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) SetAccount(ctx context.Context, account telegram.Account) error {
	if account.TelegramUserID == 0 {
		return fmt.Errorf("telegram account id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE telegram_user_id = ?`, account.TelegramUserID).Scan(&id)
	if err == nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE accounts
			SET username = ?, first_name = ?, last_name = ?, phone = ?, is_bot = ?, updated_at = ?
			WHERE id = ?`,
			account.Username,
			account.FirstName,
			account.LastName,
			account.Phone,
			boolInt(account.IsBot),
			now,
			id,
		); err != nil {
			return fmt.Errorf("update account %d: %w", account.TelegramUserID, err)
		}
		s.accountID = id
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("find account %d: %w", account.TelegramUserID, err)
	}

	adoptedID, err := s.adoptLegacyAccount(ctx, account, now)
	if err != nil {
		return err
	}
	if adoptedID != 0 {
		s.accountID = adoptedID
		return nil
	}

	res, err := s.db.ExecContext(ctx, `INSERT INTO accounts
		(telegram_user_id, username, first_name, last_name, phone, is_bot, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		account.TelegramUserID,
		account.Username,
		account.FirstName,
		account.LastName,
		account.Phone,
		boolInt(account.IsBot),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("create account %d: %w", account.TelegramUserID, err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read account id: %w", err)
	}
	s.accountID = id
	return nil
}

func (s *SQLiteStore) adoptLegacyAccount(ctx context.Context, account telegram.Account, now string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE telegram_user_id IS NULL ORDER BY id LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find legacy account: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts
		SET telegram_user_id = ?, username = ?, first_name = ?, last_name = ?, phone = ?, is_bot = ?, updated_at = ?
		WHERE id = ? AND telegram_user_id IS NULL`,
		account.TelegramUserID,
		account.Username,
		account.FirstName,
		account.LastName,
		account.Phone,
		boolInt(account.IsBot),
		now,
		id,
	); err != nil {
		return 0, fmt.Errorf("adopt legacy account %d: %w", id, err)
	}
	return id, nil
}

func (s *SQLiteStore) requireAccount() (int64, error) {
	if s.accountID == 0 {
		return 0, fmt.Errorf("storage account is not set")
	}
	return s.accountID, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	version, err := s.schemaVersion(ctx)
	if err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("sqlite schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version < 2 {
		if err := s.migrateToV2(ctx, version); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) schemaVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read sqlite schema version: %w", err)
	}
	return version, nil
}

func (s *SQLiteStore) migrateToV2(ctx context.Context, version int) error {
	hasV1Users, err := s.tableExists(ctx, "users")
	if err != nil {
		return err
	}
	if version == 0 && !hasV1Users {
		return s.createSchemaV2(ctx)
	}
	return s.migrateV1ToV2(ctx)
}

func (s *SQLiteStore) tableExists(ctx context.Context, name string) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("check sqlite table %q: %w", name, err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) createSchemaV2(ctx context.Context) error {
	statements := sqliteSchemaV2Statements()
	statements = append(statements, fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion))
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) migrateV1ToV2(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for sqlite migration: %w", err)
	}
	defer func() {
		_, _ = s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	}()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite v2 migration: %w", err)
	}
	defer rollback(tx)

	statements := []string{
		`ALTER TABLE users RENAME TO users_v1`,
		`ALTER TABLE chats RENAME TO chats_v1`,
		`ALTER TABLE topics RENAME TO topics_v1`,
		`ALTER TABLE messages RENAME TO messages_v1`,
		`ALTER TABLE sync_runs RENAME TO sync_runs_v1`,
		`ALTER TABLE sync_run_items RENAME TO sync_run_items_v1`,
	}
	statements = append(statements, sqliteSchemaV2Statements()...)
	statements = append(statements,
		`INSERT INTO accounts (id, telegram_user_id, username, first_name, last_name, phone, is_bot, created_at, updated_at)
			VALUES (1, NULL, '', 'Legacy account', '', '', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO users (account_id, id, username, first_name, last_name, is_bot, updated_at)
			SELECT 1, id, username, first_name, last_name, is_bot, updated_at FROM users_v1`,
		`INSERT INTO chats (account_id, id, title, kind, is_channel, is_forum, is_user, is_bot, unread_count, last_read_id, top_message_id, updated_at)
			SELECT 1, id, title, kind, is_channel, is_forum, is_user, is_bot, unread_count, last_read_id, top_message_id, updated_at FROM chats_v1`,
		`INSERT INTO topics (account_id, chat_id, topic_id, title, unread_count, last_read_id, top_message_id, updated_at)
			SELECT 1, chat_id, topic_id, title, unread_count, last_read_id, top_message_id, updated_at FROM topics_v1`,
		`INSERT INTO messages (account_id, chat_id, topic_id, message_id, date_unix, sender_id, text, outgoing, edit_date_unix, saved_at)
			SELECT 1, chat_id, topic_id, message_id, date_unix, sender_id, text, outgoing, edit_date_unix, saved_at FROM messages_v1`,
		`INSERT INTO sync_runs (id, account_id, mode, started_at, finished_at, status, error)
			SELECT id, 1, mode, started_at, finished_at, status, error FROM sync_runs_v1`,
		`INSERT INTO sync_run_items (account_id, run_id, chat_id, topic_id, saved_messages, mark_read_status, warning, error)
			SELECT 1, run_id, chat_id, topic_id, saved_messages, mark_read_status, warning, error FROM sync_run_items_v1`,
		`DROP TABLE users_v1`,
		`DROP TABLE chats_v1`,
		`DROP TABLE topics_v1`,
		`DROP TABLE messages_v1`,
		`DROP TABLE sync_runs_v1`,
		`DROP TABLE sync_run_items_v1`,
		fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion),
	)

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate sqlite schema to v2: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite v2 migration: %w", err)
	}
	return nil
}

func sqliteSchemaV2Statements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_user_id INTEGER UNIQUE,
			username TEXT NOT NULL DEFAULT '',
			first_name TEXT NOT NULL DEFAULT '',
			last_name TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			is_bot INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			account_id INTEGER NOT NULL,
			id INTEGER NOT NULL,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			is_bot INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (account_id, id),
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS chats (
			account_id INTEGER NOT NULL,
			id INTEGER NOT NULL,
			title TEXT NOT NULL,
			kind TEXT NOT NULL,
			is_channel INTEGER NOT NULL DEFAULT 0,
			is_forum INTEGER NOT NULL DEFAULT 0,
			is_user INTEGER NOT NULL DEFAULT 0,
			is_bot INTEGER NOT NULL DEFAULT 0,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			top_message_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (account_id, id),
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS topics (
			account_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			top_message_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (account_id, chat_id, topic_id),
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			account_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL DEFAULT 0,
			message_id INTEGER NOT NULL,
			date_unix INTEGER NOT NULL,
			sender_id INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL,
			outgoing INTEGER NOT NULL DEFAULT 0,
			edit_date_unix INTEGER NOT NULL DEFAULT 0,
			saved_at TEXT NOT NULL,
			PRIMARY KEY (account_id, chat_id, topic_id, message_id),
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL,
			mode TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			error TEXT,
			FOREIGN KEY (account_id) REFERENCES accounts(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_run_items (
			account_id INTEGER NOT NULL,
			run_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL DEFAULT 0,
			saved_messages INTEGER NOT NULL DEFAULT 0,
			mark_read_status TEXT NOT NULL DEFAULT '',
			warning TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (account_id) REFERENCES accounts(id),
			FOREIGN KEY (run_id) REFERENCES sync_runs(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_account_chat_date ON messages(account_id, chat_id, topic_id, date_unix)`,
	}
}

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

func (s *SQLiteStore) StartRun(ctx context.Context, mode string) (SyncRun, error) {
	accountID, err := s.requireAccount()
	if err != nil {
		return SyncRun{}, err
	}
	started := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO sync_runs (account_id, mode, started_at, status) VALUES (?, ?, ?, ?)`, accountID, mode, started.Format(time.RFC3339Nano), "running")
	if err != nil {
		return SyncRun{}, fmt.Errorf("start sync run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SyncRun{}, fmt.Errorf("read sync run id: %w", err)
	}
	return SyncRun{ID: id, Mode: mode, StartedAt: started}, nil
}

func (s *SQLiteStore) FinishRun(ctx context.Context, runID int64, status string, runErr error) error {
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sync_runs SET finished_at = ?, status = ?, error = ? WHERE id = ? AND account_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), status, errText, runID, accountID); err != nil {
		return fmt.Errorf("finish sync run %d: %w", runID, err)
	}
	return nil
}

func (s *SQLiteStore) AddRunItem(ctx context.Context, item RunItem) error {
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sync_run_items
		(account_id, run_id, chat_id, topic_id, saved_messages, mark_read_status, warning, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID,
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
