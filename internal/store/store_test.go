package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

func TestStoreMigrateAndUpsert(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "tg.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	setTestAccount(t, ctx, s, 111)

	chats := []telegram.Chat{
		{ID: 10, Title: "Old", IsChannel: true, UnreadCount: 1, LastReadID: 5, TopMessageID: 6},
	}
	if err := s.SaveChats(ctx, chats); err != nil {
		t.Fatalf("save chats: %v", err)
	}
	chats[0].Title = "New"
	chats[0].UnreadCount = 2
	if err := s.SaveChats(ctx, chats); err != nil {
		t.Fatalf("save chats again: %v", err)
	}

	var title string
	var unread int
	if err := s.db.QueryRowContext(ctx, `SELECT title, unread_count FROM chats WHERE id = 10`).Scan(&title, &unread); err != nil {
		t.Fatalf("query chat: %v", err)
	}
	if title != "New" || unread != 2 {
		t.Fatalf("unexpected chat row: title=%q unread=%d", title, unread)
	}

	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("query schema version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("schema version = %d, want %d", version, schemaVersion)
	}
}

func TestStoreSaveMessagesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "tg.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	setTestAccount(t, ctx, s, 111)

	msg := telegram.Message{
		ID:       100,
		Date:     time.Unix(123, 0),
		Text:     "hello",
		SenderID: 42,
	}
	if saved, err := s.SaveMessages(ctx, 10, 0, []telegram.Message{msg}); err != nil || saved != 1 {
		t.Fatalf("save messages = %d, %v", saved, err)
	}
	msg.Text = "updated"
	if saved, err := s.SaveMessages(ctx, 10, 0, []telegram.Message{msg}); err != nil || saved != 1 {
		t.Fatalf("save messages again = %d, %v", saved, err)
	}

	var count int
	var text string
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), text FROM messages WHERE chat_id = 10 AND topic_id = 0 AND message_id = 100`).Scan(&count, &text); err != nil {
		t.Fatalf("query messages: %v", err)
	}
	if count != 1 || text != "updated" {
		t.Fatalf("unexpected message row: count=%d text=%q", count, text)
	}
}

func TestStoreRecordsSyncRun(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "tg.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	setTestAccount(t, ctx, s, 111)

	run, err := s.StartRun(ctx, "sync")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := s.AddRunItem(ctx, RunItem{RunID: run.ID, ChatID: 10, SavedMessages: 3, MarkReadStatus: "ok"}); err != nil {
		t.Fatalf("add run item: %v", err)
	}
	if err := s.FinishRun(ctx, run.ID, "ok", nil); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	var status string
	var saved int
	if err := s.db.QueryRowContext(ctx, `SELECT r.status, i.saved_messages
		FROM sync_runs r JOIN sync_run_items i ON i.run_id = r.id WHERE r.id = ?`, run.ID).Scan(&status, &saved); err != nil {
		t.Fatalf("query run: %v", err)
	}
	if status != "ok" || saved != 3 {
		t.Fatalf("unexpected run row: status=%q saved=%d", status, saved)
	}
}

func TestStoreScopesRowsByAccount(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "tg.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	setTestAccount(t, ctx, s, 111)
	if _, err := s.SaveMessages(ctx, 10, 0, []telegram.Message{{ID: 100, Date: time.Unix(123, 0), Text: "first"}}); err != nil {
		t.Fatalf("save first account messages: %v", err)
	}
	setTestAccount(t, ctx, s, 222)
	if _, err := s.SaveMessages(ctx, 10, 0, []telegram.Message{{ID: 100, Date: time.Unix(123, 0), Text: "second"}}); err != nil {
		t.Fatalf("save second account messages: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id = 10 AND topic_id = 0 AND message_id = 100`).Scan(&count); err != nil {
		t.Fatalf("query messages: %v", err)
	}
	if count != 2 {
		t.Fatalf("message rows = %d, want 2", count)
	}
}

func TestStoreMigratesV1RowsToAccountScope(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tg.db")
	createV1Store(t, ctx, path)

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	setTestAccount(t, ctx, s, 333)

	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("query schema version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("schema version = %d, want %d", version, schemaVersion)
	}

	var telegramUserID int64
	if err := s.db.QueryRowContext(ctx, `SELECT telegram_user_id FROM accounts WHERE id = 1`).Scan(&telegramUserID); err != nil {
		t.Fatalf("query adopted account: %v", err)
	}
	if telegramUserID != 333 {
		t.Fatalf("telegram_user_id = %d, want 333", telegramUserID)
	}

	var text string
	if err := s.db.QueryRowContext(ctx, `SELECT text FROM messages WHERE account_id = 1 AND chat_id = 10 AND message_id = 100`).Scan(&text); err != nil {
		t.Fatalf("query migrated message: %v", err)
	}
	if text != "legacy" {
		t.Fatalf("migrated message text = %q, want legacy", text)
	}
}

func setTestAccount(t *testing.T, ctx context.Context, s *SQLiteStore, id int64) {
	t.Helper()
	if err := s.SetAccount(ctx, telegram.Account{
		TelegramUserID: id,
		Username:       "test",
		FirstName:      "Test",
	}); err != nil {
		t.Fatalf("set account: %v", err)
	}
}

func createV1Store(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	statements := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			is_bot INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE chats (
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
		`CREATE TABLE topics (
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			unread_count INTEGER NOT NULL DEFAULT 0,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			top_message_id INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (chat_id, topic_id)
		)`,
		`CREATE TABLE messages (
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
		`CREATE TABLE sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mode TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			error TEXT
		)`,
		`CREATE TABLE sync_run_items (
			run_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			topic_id INTEGER NOT NULL DEFAULT 0,
			saved_messages INTEGER NOT NULL DEFAULT 0,
			mark_read_status TEXT NOT NULL DEFAULT '',
			warning TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (run_id) REFERENCES sync_runs(id)
		)`,
		`CREATE INDEX idx_messages_chat_date ON messages(chat_id, topic_id, date_unix)`,
		`INSERT INTO users (id, username, first_name, last_name, is_bot, updated_at)
			VALUES (42, 'sender', 'Sender', '', 0, '2026-01-01T00:00:00Z')`,
		`INSERT INTO chats (id, title, kind, updated_at)
			VALUES (10, 'Legacy Chat', 'chat', '2026-01-01T00:00:00Z')`,
		`INSERT INTO messages (chat_id, topic_id, message_id, date_unix, sender_id, text, saved_at)
			VALUES (10, 0, 100, 123, 42, 'legacy', '2026-01-01T00:00:00Z')`,
		`INSERT INTO sync_runs (id, mode, started_at, status)
			VALUES (1, 'history', '2026-01-01T00:00:00Z', 'ok')`,
		`INSERT INTO sync_run_items (run_id, chat_id, saved_messages)
			VALUES (1, 10, 1)`,
		`PRAGMA user_version = 1`,
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create v1 store: %v", err)
		}
	}
}
