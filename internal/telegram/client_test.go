package telegram

import (
	"context"
	"testing"
	"time"

	"cli-tg-chat-summary/internal/config"

	"github.com/gotd/td/tg"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		TelegramAppID:   12345,
		TelegramAppHash: "testhash",
		SessionPath:     "session/session.db",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.peerCache == nil {
		t.Error("peerCache not initialized")
	}
	if client.channelCache == nil {
		t.Error("channelCache not initialized")
	}
}

func TestAccountRequiresLogin(t *testing.T) {
	client := &Client{}

	_, err := client.Account()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessDialogs_UserDialogs(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:           &tg.PeerUser{UserID: 123},
			UnreadCount:    5,
			ReadInboxMaxID: 100,
		},
	}
	users := []tg.UserClass{
		&tg.User{
			ID:         123,
			FirstName:  "John",
			LastName:   "Doe",
			Username:   "johndoe",
			AccessHash: 456,
		},
	}

	result := client.processDialogs(dialogs, nil, users)

	if len(result) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(result))
	}

	chat := result[0]
	if chat.ID != 123 {
		t.Errorf("expected ID 123, got %d", chat.ID)
	}
	if chat.Title != "John Doe (@johndoe)" {
		t.Errorf("expected title 'John Doe (@johndoe)', got '%s'", chat.Title)
	}
	if chat.UnreadCount != 5 {
		t.Errorf("expected UnreadCount 5, got %d", chat.UnreadCount)
	}
	if chat.IsChannel {
		t.Error("expected IsChannel false")
	}
	if chat.LastReadID != 100 {
		t.Errorf("expected LastReadID 100, got %d", chat.LastReadID)
	}

	// Check peer cache was populated
	if _, ok := client.peerCache[123]; !ok {
		t.Error("user peer not cached")
	}
}

func TestProcessDialogs_ChatDialogs(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:        &tg.PeerChat{ChatID: 789},
			UnreadCount: 10,
		},
	}
	chats := []tg.ChatClass{
		&tg.Chat{
			ID:    789,
			Title: "Test Group",
		},
	}

	result := client.processDialogs(dialogs, chats, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(result))
	}

	chat := result[0]
	if chat.Title != "Test Group" {
		t.Errorf("expected title 'Test Group', got '%s'", chat.Title)
	}
	if chat.IsChannel {
		t.Error("expected IsChannel false for basic chat")
	}
}

func TestProcessDialogs_ChannelDialogs(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:        &tg.PeerChannel{ChannelID: 111},
			UnreadCount: 3,
		},
	}
	chats := []tg.ChatClass{
		&tg.Channel{
			ID:         111,
			Title:      "News Channel",
			AccessHash: 999,
			Forum:      false,
		},
	}

	result := client.processDialogs(dialogs, chats, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(result))
	}

	chat := result[0]
	if chat.Title != "News Channel" {
		t.Errorf("expected title 'News Channel', got '%s'", chat.Title)
	}
	if !chat.IsChannel {
		t.Error("expected IsChannel true")
	}
	if chat.IsForum {
		t.Error("expected IsForum false")
	}

	// Check caches
	if _, ok := client.peerCache[111]; !ok {
		t.Error("channel peer not cached")
	}
	if _, ok := client.channelCache[111]; !ok {
		t.Error("channel not cached in channelCache")
	}
}

func TestProcessDialogs_ForumChannel(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:        &tg.PeerChannel{ChannelID: 222},
			UnreadCount: 15,
		},
	}
	chats := []tg.ChatClass{
		&tg.Channel{
			ID:         222,
			Title:      "Discussion Forum",
			AccessHash: 888,
			Forum:      true,
		},
	}

	result := client.processDialogs(dialogs, chats, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(result))
	}

	chat := result[0]
	if !chat.IsForum {
		t.Error("expected IsForum true for forum channel")
	}
	if !chat.IsChannel {
		t.Error("expected IsChannel true for forum channel")
	}
}

func TestProcessDialogs_UnknownPeer(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer:        &tg.PeerUser{UserID: 999},
			UnreadCount: 1,
		},
	}
	// No matching user in users list

	result := client.processDialogs(dialogs, nil, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dialog, got %d", len(result))
	}

	if result[0].Title != "Unknown Peer 999" {
		t.Errorf("expected 'Unknown Peer 999', got '%s'", result[0].Title)
	}
}

func TestProcessDialogs_UserWithoutUsername(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{
			Peer: &tg.PeerUser{UserID: 123},
		},
	}
	users := []tg.UserClass{
		&tg.User{
			ID:        123,
			FirstName: "Jane",
			LastName:  "Smith",
			Username:  "", // no username
		},
	}

	result := client.processDialogs(dialogs, nil, users)

	if result[0].Title != "Jane Smith" {
		t.Errorf("expected 'Jane Smith', got '%s'", result[0].Title)
	}
}

func TestProcessDialogs_MultipleDialogs(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}, UnreadCount: 5},
		&tg.Dialog{Peer: &tg.PeerChat{ChatID: 2}, UnreadCount: 10},
		&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 3}, UnreadCount: 3},
	}
	users := []tg.UserClass{
		&tg.User{ID: 1, FirstName: "User1", AccessHash: 100},
	}
	chats := []tg.ChatClass{
		&tg.Chat{ID: 2, Title: "Chat2"},
		&tg.Channel{ID: 3, Title: "Channel3", AccessHash: 300},
	}

	result := client.processDialogs(dialogs, chats, users)

	if len(result) != 3 {
		t.Fatalf("expected 3 dialogs, got %d", len(result))
	}

	// Check all peer types are in cache
	if len(client.peerCache) != 3 {
		t.Errorf("expected 3 entries in peerCache, got %d", len(client.peerCache))
	}
}

// Message struct tests
func TestMessageStruct(t *testing.T) {
	now := time.Now()
	msg := Message{
		ID:       42,
		Date:     now,
		Text:     "Hello, World!",
		SenderID: 123,
	}

	if msg.ID != 42 {
		t.Errorf("expected ID 42, got %d", msg.ID)
	}
	if msg.Text != "Hello, World!" {
		t.Errorf("unexpected text: %s", msg.Text)
	}
	if msg.SenderID != 123 {
		t.Errorf("unexpected sender id: %d", msg.SenderID)
	}
}

// Chat struct tests
func TestChatStruct(t *testing.T) {
	chat := Chat{
		ID:          123,
		Title:       "Test Chat",
		UnreadCount: 10,
		IsChannel:   true,
		IsForum:     true,
		IsUser:      false,
		IsBot:       false,
		LastReadID:  50,
	}

	if chat.ID != 123 {
		t.Errorf("unexpected ID: %d", chat.ID)
	}
	if !chat.IsChannel {
		t.Error("expected IsChannel true")
	}
	if !chat.IsForum {
		t.Error("expected IsForum true")
	}
	if chat.IsUser {
		t.Error("expected IsUser false")
	}
	if chat.IsBot {
		t.Error("expected IsBot false")
	}
}

// Topic struct tests
func TestTopicStruct(t *testing.T) {
	topic := Topic{
		ID:          1,
		Title:       "General",
		UnreadCount: 5,
		LastReadID:  100,
	}

	if topic.ID != 1 {
		t.Errorf("unexpected ID: %d", topic.ID)
	}
	if topic.Title != "General" {
		t.Errorf("unexpected Title: %s", topic.Title)
	}
}

func TestFetchMessages_PaginatesAndReportsProgress(t *testing.T) {
	client := &Client{}

	callCount := 0
	var seenOffsets []int
	var seenOffsetDates []int
	var progressBatches []ProgressUpdate

	fetch := func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
		callCount++
		seenOffsets = append(seenOffsets, offsetID)
		seenOffsetDates = append(seenOffsetDates, offsetDate)

		switch callCount {
		case 1:
			msgs := make([]tg.MessageClass, 0, limit)
			for i := 0; i < limit; i++ {
				msgs = append(msgs, &tg.Message{ID: 200 - i, Date: 1000 - i, Message: "a"})
			}
			return &tg.MessagesMessages{Messages: msgs}, nil
		case 2:
			return &tg.MessagesMessages{
				Messages: []tg.MessageClass{
					&tg.Message{ID: 99, Date: 500, Message: "b"},
					&tg.Message{ID: 98, Date: 400, Message: "c"},
				},
			}, nil
		default:
			return &tg.MessagesMessages{Messages: nil}, nil
		}
	}

	filter := func(msg *tg.Message) (bool, bool) {
		return true, false
	}

	progress := func(update ProgressUpdate) {
		progressBatches = append(progressBatches, update)
	}

	got, err := client.fetchMessages(
		context.Background(),
		progress,
		"test-phase",
		time.Time{},
		false,
		fetch,
		filter,
		MessageFetchOptions{},
	)
	if err != nil {
		t.Fatalf("fetchMessages error: %v", err)
	}
	if len(got.Messages) != 102 {
		t.Fatalf("expected 102 messages, got %d", len(got.Messages))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 fetch calls, got %d", callCount)
	}
	if len(seenOffsets) < 2 || seenOffsets[0] != 0 || seenOffsets[1] != 101 {
		t.Fatalf("unexpected offsets: %v", seenOffsets)
	}
	for _, od := range seenOffsetDates {
		if od != 0 {
			t.Fatalf("unexpected offsetDate when not using it: %v", seenOffsetDates)
		}
	}
	if len(progressBatches) < 2 {
		t.Fatalf("expected progress updates per batch, got %d", len(progressBatches))
	}
	if progressBatches[0].Phase != "test-phase" {
		t.Fatalf("unexpected progress phase: %s", progressBatches[0].Phase)
	}
}

func TestFetchMessages_PacesBetweenHistoryPages(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 2000,
	})
	var pauses int
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		pauses++
		if d != 2*time.Second {
			t.Fatalf("unexpected pause: got %v want %v", d, 2*time.Second)
		}
		return nil
	}

	client := &Client{historyPacer: pacer}
	callCount := 0
	var phases []string

	fetch := func(_ int, _ int, limit int) (tg.MessagesMessagesClass, error) {
		callCount++
		switch callCount {
		case 1:
			msgs := make([]tg.MessageClass, 0, limit)
			for i := 0; i < limit; i++ {
				msgs = append(msgs, &tg.Message{ID: 200 - i, Date: 1000 - i, Message: "a"})
			}
			return &tg.MessagesMessages{Messages: msgs}, nil
		case 2:
			return &tg.MessagesMessages{
				Messages: []tg.MessageClass{
					&tg.Message{ID: 99, Date: 500, Message: "b"},
				},
			}, nil
		default:
			return &tg.MessagesMessages{Messages: nil}, nil
		}
	}

	_, err := client.fetchMessages(
		context.Background(),
		func(update ProgressUpdate) {
			phases = append(phases, update.Phase)
		},
		"test-phase",
		time.Time{},
		false,
		fetch,
		func(*tg.Message) (bool, bool) { return true, false },
		MessageFetchOptions{},
	)
	if err != nil {
		t.Fatalf("fetchMessages error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 fetch calls, got %d", callCount)
	}
	if pauses != 1 {
		t.Fatalf("expected 1 pause between pages, got %d", pauses)
	}
	foundPause := false
	for _, phase := range phases {
		if phase == "pausing 2.0s before next history request" {
			foundPause = true
			break
		}
	}
	if !foundPause {
		t.Fatalf("expected pause progress phase, got %v", phases)
	}
}

func TestFetchMessages_UsesOffsetDateOnFirstPage(t *testing.T) {
	client := &Client{}

	var seenOffsetDates []int
	until := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	fetch := func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
		seenOffsetDates = append(seenOffsetDates, offsetDate)
		return &tg.MessagesMessages{Messages: nil}, nil
	}

	filter := func(msg *tg.Message) (bool, bool) {
		return true, false
	}

	_, err := client.fetchMessages(
		context.Background(),
		nil,
		"phase",
		until,
		true,
		fetch,
		filter,
		MessageFetchOptions{},
	)
	if err != nil {
		t.Fatalf("fetchMessages error: %v", err)
	}
	if len(seenOffsetDates) != 1 {
		t.Fatalf("expected 1 fetch call, got %d", len(seenOffsetDates))
	}
	if seenOffsetDates[0] != int(until.Unix()) {
		t.Fatalf("unexpected offsetDate: got %d want %d", seenOffsetDates[0], int(until.Unix()))
	}
}

func TestFetchMessages_RespectsMessageLimitAndCollectsUsers(t *testing.T) {
	client := &Client{
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}

	fetch := func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
		return &tg.MessagesMessages{
			Messages: []tg.MessageClass{
				&tg.Message{ID: 3, Date: 300, Message: "c", FromID: &tg.PeerUser{UserID: 7}},
				&tg.Message{ID: 2, Date: 200, Message: "b", FromID: &tg.PeerUser{UserID: 7}},
				&tg.Message{ID: 1, Date: 100, Message: "a", FromID: &tg.PeerUser{UserID: 8}},
			},
			Users: []tg.UserClass{
				&tg.User{ID: 7, FirstName: "Ada", Username: "ada", AccessHash: 77},
			},
		}, nil
	}

	got, err := client.fetchMessages(
		context.Background(),
		nil,
		"phase",
		time.Time{},
		false,
		fetch,
		func(*tg.Message) (bool, bool) { return true, false },
		MessageFetchOptions{MessageLimit: 2},
	)
	if err != nil {
		t.Fatalf("fetchMessages error: %v", err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got.Messages))
	}
	if !got.Truncated {
		t.Fatal("expected truncated result")
	}
	if len(got.Users) != 1 || got.Users[0].ID != 7 || got.Users[0].Username != "ada" {
		t.Fatalf("unexpected users: %+v", got.Users)
	}
	if _, ok := client.peerCache[7]; !ok {
		t.Fatal("expected user peer cache entry")
	}
}

func TestProcessMessageBatch_AdvancesCursorForServiceMessages(t *testing.T) {
	msgs := []tg.MessageClass{
		&tg.Message{
			ID:      200,
			Date:    1000,
			Message: "exported",
			FromID:  &tg.PeerUser{UserID: 42},
		},
		&tg.MessageService{
			ID:   199,
			Date: 999,
		},
	}

	got, lastID, stop := processMessageBatch(msgs, func(*tg.Message) (bool, bool) {
		return true, false
	})
	if stop {
		t.Fatal("unexpected stop")
	}
	if lastID != 199 {
		t.Fatalf("expected last cursor id 199, got %d", lastID)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 exported message, got %d", len(got))
	}
	if got[0].ID != 200 || got[0].SenderID != 42 {
		t.Fatalf("unexpected exported message: %+v", got[0])
	}
}

func TestNextForumTopicOffset_UsesTopMessageDate(t *testing.T) {
	page := &tg.MessagesForumTopics{
		Topics: []tg.ForumTopicClass{
			&tg.ForumTopic{ID: 7, Date: 100, TopMessage: 50},
		},
		Messages: []tg.MessageClass{
			&tg.MessageService{ID: 50, Date: 1234},
		},
	}

	offset, ok := nextForumTopicOffset(page)
	if !ok {
		t.Fatal("expected offset")
	}
	if offset.topic != 7 || offset.id != 50 || offset.date != 1234 {
		t.Fatalf("unexpected offset: %+v", offset)
	}
}

func TestNextForumTopicOffset_UsesTopicDateWhenOrderedByCreateDate(t *testing.T) {
	page := &tg.MessagesForumTopics{
		OrderByCreateDate: true,
		Topics: []tg.ForumTopicClass{
			&tg.ForumTopic{ID: 7, Date: 100, TopMessage: 50},
		},
		Messages: []tg.MessageClass{
			&tg.Message{ID: 50, Date: 1234},
		},
	}

	offset, ok := nextForumTopicOffset(page)
	if !ok {
		t.Fatal("expected offset")
	}
	if offset.topic != 7 || offset.id != 50 || offset.date != 100 {
		t.Fatalf("unexpected offset: %+v", offset)
	}
}
