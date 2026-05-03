package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"cli-tg-chat-summary/internal/telegram"
)

func TestFileModeIsTerminal(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		want bool
	}{
		{
			name: "char device",
			mode: os.ModeCharDevice,
			want: true,
		},
		{
			name: "regular file",
			mode: 0644,
			want: false,
		},
		{
			name: "pipe",
			mode: os.ModeNamedPipe,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileModeIsTerminal(tt.mode); got != tt.want {
				t.Fatalf("fileModeIsTerminal(%v) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestSyncChatsFiltersUnreadAndAppliesLimit(t *testing.T) {
	chats := []telegram.Chat{
		{ID: 1, Title: "read", UnreadCount: 0},
		{ID: 2, Title: "unread-a", UnreadCount: 3},
		{ID: 3, Title: "unread-b", UnreadCount: 1},
	}

	got, err := syncChats(chats, RunOptions{ChatLimit: 1})
	if err != nil {
		t.Fatalf("syncChats error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("unexpected chats: %+v", got)
	}
}

func TestSyncChatsFindsSpecificChat(t *testing.T) {
	chats := []telegram.Chat{{ID: 2, Title: "unread", UnreadCount: 3}}

	got, err := syncChats(chats, RunOptions{ChatID: 2})
	if err != nil {
		t.Fatalf("syncChats error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("unexpected chats: %+v", got)
	}

	_, err = syncChats(chats, RunOptions{ChatID: 9})
	if err == nil || !strings.Contains(err.Error(), "chat with id 9 not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkMessagesAsReadSkipsTruncatedResults(t *testing.T) {
	app := &App{}
	result := telegram.MessageFetchResult{
		Messages:  []telegram.Message{{ID: 10}},
		Truncated: true,
	}

	got := app.markMessagesAsRead(context.Background(), telegram.Chat{ID: 1}, nil, result, RunOptions{})
	if got.Attempted {
		t.Fatal("did not expect mark-as-read attempt")
	}
	if !strings.Contains(got.Warning, "message limit truncated") {
		t.Fatalf("unexpected warning: %q", got.Warning)
	}
}
