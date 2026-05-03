package store

import (
	"context"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

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
