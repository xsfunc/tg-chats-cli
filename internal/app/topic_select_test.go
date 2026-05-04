package app

import (
	"strings"
	"testing"

	"tg-arc/internal/telegram"
)

func TestSelectForumTopic(t *testing.T) {
	topics := []telegram.Topic{
		{ID: 1, Title: "General"},
		{ID: 2, Title: "Release Notes"},
		{ID: 3, Title: "General Updates"},
	}

	tests := []struct {
		name        string
		topicID     int
		topicTitle  string
		wantID      int
		wantErrText string
	}{
		{
			name:    "by id",
			topicID: 2,
			wantID:  2,
		},
		{
			name:        "by id not found",
			topicID:     9,
			wantErrText: "forum topic id 9 not found",
		},
		{
			name:       "by title exact case insensitive",
			topicTitle: "general",
			wantID:     1,
		},
		{
			name:       "by title contains",
			topicTitle: "release",
			wantID:     2,
		},
		{
			name:        "by title multiple matches",
			topicTitle:  "gen",
			wantErrText: "matched multiple topics",
		},
		{
			name:        "by title none",
			topicTitle:  "random",
			wantErrText: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, err := selectForumTopic(topics, tt.topicID, tt.topicTitle)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrText)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if topic == nil {
				t.Fatalf("expected topic, got nil")
			}
			if topic.ID != tt.wantID {
				t.Fatalf("expected topic id %d, got %d", tt.wantID, topic.ID)
			}
		})
	}
}
