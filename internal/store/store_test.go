package store

import (
	"context"
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
