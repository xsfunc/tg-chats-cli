package main

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeChatID(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{
			name:     "raw channel id",
			input:    123456789,
			expected: 123456789,
		},
		{
			name:     "bot api -100 id",
			input:    -1001234567890,
			expected: 1234567890,
		},
		{
			name:     "negative non -100 id",
			input:    -123,
			expected: -123,
		},
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeChatID(tt.input); got != tt.expected {
				t.Fatalf("normalizeChatID(%d) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseRunOptions_DateRange(t *testing.T) {
	opts, err := parseRunOptions([]string{
		"history",
		"--id", "-1001234567890",
		"--since", "2024-01-01",
		"--until", "2024-01-31",
	}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if !opts.NonInteractive {
		t.Fatal("expected non-interactive mode")
	}
	if opts.ChatID != 1234567890 {
		t.Fatalf("unexpected normalized chat id: %d", opts.ChatID)
	}
	if !opts.UseDateRange {
		t.Fatal("expected date range mode")
	}
	wantSince := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2024, 1, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !opts.Since.Equal(wantSince) {
		t.Fatalf("unexpected since: got %v want %v", opts.Since, wantSince)
	}
	if !opts.Until.Equal(wantUntil) {
		t.Fatalf("unexpected until: got %v want %v", opts.Until, wantUntil)
	}
}

func TestParseRunOptions_Sync(t *testing.T) {
	opts, err := parseRunOptions([]string{
		"sync",
		"--id", "-1001234567890",
		"--chat-limit", "5",
		"--message-limit", "10",
		"--db", "tmp/tg.db",
		"--session", "session/account-a.db",
	}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if opts.Command != "sync" {
		t.Fatalf("command = %q, want sync", opts.Command)
	}
	if !opts.NonInteractive {
		t.Fatal("expected sync to be non-interactive")
	}
	if opts.ChatID != 1234567890 || opts.ChatIDRaw != -1001234567890 {
		t.Fatalf("unexpected chat ids: raw=%d normalized=%d", opts.ChatIDRaw, opts.ChatID)
	}
	if opts.ChatLimit != 5 || opts.MessageLimit != 10 || opts.DBPath != "tmp/tg.db" || opts.SessionPath != "session/account-a.db" {
		t.Fatalf("unexpected limits/db/session: %+v", opts)
	}
}

func TestParseRunOptions_Chats(t *testing.T) {
	opts, err := parseRunOptions([]string{
		"chats",
		"--db", "tmp/tg.db",
		"--account-id", "2",
	}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if opts.Command != "chats" {
		t.Fatalf("command = %q, want chats", opts.Command)
	}
	if !opts.NonInteractive {
		t.Fatal("expected chats to be non-interactive")
	}
	if opts.DBPath != "tmp/tg.db" || opts.AccountID != 2 {
		t.Fatalf("unexpected db/account: %+v", opts)
	}
}

func TestParseRunOptions_Export(t *testing.T) {
	opts, err := parseRunOptions([]string{
		"export",
		"--id", "-1001234567890",
		"--topic-id", "42",
		"--since", "2024-01-01",
		"--until", "2024-01-31",
		"--db", "tmp/tg.db",
		"--account-id", "2",
	}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if opts.Command != "export" {
		t.Fatalf("command = %q, want export", opts.Command)
	}
	if opts.ChatID != 1234567890 || opts.ChatIDRaw != -1001234567890 {
		t.Fatalf("unexpected chat ids: raw=%d normalized=%d", opts.ChatIDRaw, opts.ChatID)
	}
	if opts.TopicID != 42 || !opts.TopicIDSet {
		t.Fatalf("unexpected topic id state: id=%d set=%v", opts.TopicID, opts.TopicIDSet)
	}
	if !opts.UseDateRange || !opts.UseUntil {
		t.Fatalf("expected since and until filters: %+v", opts)
	}
	if opts.DBPath != "tmp/tg.db" || opts.AccountID != 2 {
		t.Fatalf("unexpected db/account: %+v", opts)
	}
}

func TestParseRunOptions_ExportAllowsUntilOnly(t *testing.T) {
	opts, err := parseRunOptions([]string{"export", "--id", "123", "--until", "2024-01-31"}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if opts.UseDateRange {
		t.Fatal("did not expect since filter")
	}
	if !opts.UseUntil {
		t.Fatal("expected until filter")
	}
}

func TestParseRunOptions_ExportRequiresID(t *testing.T) {
	_, err := parseRunOptions([]string{"export", "--db", "tmp/tg.db"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "export requires --id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunOptions_RejectsFormat(t *testing.T) {
	_, err := parseRunOptions([]string{"history", "--format", "xml"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--format is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunOptions_SyncRejectsDateRange(t *testing.T) {
	_, err := parseRunOptions([]string{"sync", "--since", "2024-01-01"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sync does not support --since/--until") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunOptions_SinceDefaultsUntilToNow(t *testing.T) {
	opts, err := parseRunOptions([]string{"--since", "2024-01-01"}, fixedNow)
	if err != nil {
		t.Fatalf("parseRunOptions error: %v", err)
	}
	if !opts.UseDateRange {
		t.Fatal("expected date range mode")
	}
	if !opts.Until.Equal(fixedNow()) {
		t.Fatalf("unexpected until: got %v want %v", opts.Until, fixedNow())
	}
}

func TestParseRunOptions_UntilRequiresSince(t *testing.T) {
	_, err := parseRunOptions([]string{"--id", "123", "--until", "2024-01-31"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--until requires --since") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunOptions_RejectsReversedDateRange(t *testing.T) {
	_, err := parseRunOptions([]string{"--since", "2024-02-01", "--until", "2024-01-31"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--until cannot be before --since") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunOptions_TopicRequiresID(t *testing.T) {
	_, err := parseRunOptions([]string{"--topic", "General"}, fixedNow)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--topic-id/--topic requires --id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func fixedNow() time.Time {
	return time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
}
