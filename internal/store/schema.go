package store

import (
	"context"
	"fmt"
)

const schemaVersion = 2

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
