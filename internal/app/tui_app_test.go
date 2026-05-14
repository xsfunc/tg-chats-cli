package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tg-arc/internal/telegram"
	"tg-arc/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppModel_TopicCancelReturnsToChatList(t *testing.T) {
	model := appModel{
		state: stateTopicList,
		topic: tui.NewTopicModel([]telegram.Topic{{ID: 1, Title: "General"}}),
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("expected no command on cancel")
	}

	m := updated.(appModel)
	if m.state != stateChatList {
		t.Errorf("expected stateChatList, got %v", m.state)
	}
}

func TestAppModel_ProgressQStopsFetchWithoutQuitting(t *testing.T) {
	stopped := false
	model := appModel{
		state: stateProgress,
		fetchHandle: &fetchHandle{
			stop: func() {
				stopped = true
			},
		},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Error("expected no command for graceful stop")
	}
	if !stopped {
		t.Fatal("expected fetch stop to be requested")
	}

	m := updated.(appModel)
	if m.state != stateProgress {
		t.Fatalf("expected stateProgress, got %v", m.state)
	}
	if !strings.Contains(m.progress.View(), "stopping after current request") {
		t.Fatalf("expected stopping status, got %q", m.progress.View())
	}
}

type forumMarkClientStub struct {
	topics []telegram.Topic
	err    error
	calls  []markCall
}

type markCall struct {
	chatID  int64
	topicID int
	maxID   int
}

func (f *forumMarkClientStub) GetForumTopics(_ context.Context, _ int64) ([]telegram.Topic, error) {
	return f.topics, f.err
}

func (f *forumMarkClientStub) MarkTopicAsRead(_ context.Context, chatID int64, topicID int, maxID int) error {
	f.calls = append(f.calls, markCall{chatID: chatID, topicID: topicID, maxID: maxID})
	if maxID == 13 {
		return errors.New("boom")
	}
	return nil
}

func TestMarkForumAsRead(t *testing.T) {
	client := &forumMarkClientStub{
		topics: []telegram.Topic{
			{ID: 1, Title: "General", UnreadCount: 0, TopMessageID: 11},
			{ID: 2, Title: "Updates", UnreadCount: 3, TopMessageID: 12},
			{ID: 3, Title: "Bugs", UnreadCount: 2, TopMessageID: 13},
		},
	}

	chat := telegram.Chat{ID: 42, Title: "Forum", IsForum: true}
	err := markForumAsRead(context.Background(), client, chat)
	if err == nil {
		t.Fatal("expected error from failed topic mark")
	}

	if len(client.calls) != 2 {
		t.Fatalf("expected 2 mark calls, got %d", len(client.calls))
	}
	if client.calls[0].topicID != 2 || client.calls[0].maxID != 12 {
		t.Errorf("unexpected first call: %+v", client.calls[0])
	}
	if client.calls[1].topicID != 3 || client.calls[1].maxID != 13 {
		t.Errorf("unexpected second call: %+v", client.calls[1])
	}
}
