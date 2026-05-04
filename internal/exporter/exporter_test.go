package exporter

import (
	"strings"
	"testing"
	"time"

	"tg-arc/internal/store"
)

func TestRenderMarkdown(t *testing.T) {
	location := time.FixedZone("MSK", 3*60*60)
	var b strings.Builder
	if err := RenderMarkdown(&b, store.ExportData{
		Account: store.AccountSummary{
			ID:             1,
			Username:       "me",
			FirstName:      "Test",
			TelegramUserID: 111,
		},
		Chat: store.ChatSummary{
			ID:    10,
			Title: "Project",
		},
		Topic: store.TopicSummary{
			ID:    42,
			Title: "Release",
		},
		HasTopic: true,
		Messages: []store.ExportMessage{
			{
				ID:              1,
				Date:            time.Date(2026, 5, 3, 18, 20, 0, 0, time.UTC),
				Text:            "hello\nworld",
				SenderID:        42,
				SenderUsername:  "alice",
				SenderFirstName: "Alice",
			},
			{
				ID:       2,
				Date:     time.Date(2026, 5, 3, 18, 21, 0, 0, time.UTC),
				Text:     "reply",
				SenderID: 111,
				Outgoing: true,
			},
			{
				ID:       3,
				Date:     time.Date(2026, 5, 3, 18, 22, 0, 0, time.UTC),
				Text:     "fallback",
				SenderID: 99,
			},
		},
	}, location); err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	got := b.String()
	wants := []string{
		"# Telegram Chat Export",
		"Account: Test (@me), account_id=1",
		"Chat: Project, chat_id=10",
		"Topic: Release, topic_id=42",
		"Range: 2026-05-03 21:20 +03:00 .. 2026-05-03 21:22 +03:00",
		"Messages: 3",
		"[2026-05-03 21:20 +03:00] Alice (@alice):\nhello\nworld",
		"[2026-05-03 21:21 +03:00] Me:\nreply",
		"[2026-05-03 21:22 +03:00] sender_id=99:\nfallback",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered markdown missing %q:\n%s", want, got)
		}
	}
}

func TestRenderMarkdownEmpty(t *testing.T) {
	var b strings.Builder
	if err := RenderMarkdown(&b, store.ExportData{
		Account: store.AccountSummary{ID: 1},
		Chat:    store.ChatSummary{ID: 10, Title: "Empty"},
	}, time.UTC); err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	got := b.String()
	if !strings.Contains(got, "Range: (no messages)") || !strings.Contains(got, "Messages: 0") {
		t.Fatalf("unexpected empty render:\n%s", got)
	}
}
