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
