package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"cli-tg-chat-summary/internal/telegram"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal filename",
			input:    "My Chat Name",
			expected: "My Chat Name",
		},
		{
			name:     "with slash",
			input:    "Chat/Name",
			expected: "Chat_Name",
		},
		{
			name:     "with backslash",
			input:    "Chat\\Name",
			expected: "Chat_Name",
		},
		{
			name:     "with colon",
			input:    "Chat:Name",
			expected: "Chat_Name",
		},
		{
			name:     "with asterisk",
			input:    "Chat*Name",
			expected: "Chat_Name",
		},
		{
			name:     "with question mark",
			input:    "Chat?Name",
			expected: "Chat_Name",
		},
		{
			name:     "with quotes",
			input:    "Chat\"Name",
			expected: "Chat_Name",
		},
		{
			name:     "with angle brackets",
			input:    "Chat<Name>",
			expected: "Chat_Name_",
		},
		{
			name:     "with pipe",
			input:    "Chat|Name",
			expected: "Chat_Name",
		},
		{
			name:     "multiple invalid chars",
			input:    "Chat/Name:Test*File",
			expected: "Chat_Name_Test_File",
		},
		{
			name:     "with spaces",
			input:    "  Chat Name  ",
			expected: "Chat Name",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only invalid chars",
			input:    "/\\:*?\"<>|",
			expected: "_________",
		},
		{
			name:     "forum topic with dash",
			input:    "My Forum - General Topic",
			expected: "My Forum - General Topic",
		},
		{
			name:     "cyrillic text",
			input:    "Мой чат",
			expected: "Мой чат",
		},
		{
			name:     "emoji in name",
			input:    "Chat 🚀 Name",
			expected: "Chat 🚀 Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeFilename_EdgeCases(t *testing.T) {
	// Test that result is safe for filesystem
	dangerous := "../../etc/passwd"
	result := sanitizeFilename(dangerous)

	// Should not contain path traversal
	if result == dangerous {
		t.Errorf("sanitizeFilename should have modified dangerous path: %s", result)
	}
}

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
