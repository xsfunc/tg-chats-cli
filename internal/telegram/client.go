package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/gotd/td/bin"

	"cli-tg-chat-summary/internal/config"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"
)

type Client struct {
	cfg          *config.Config
	proto        *gotgproto.Client
	ctx          *ext.Context
	peerCache    map[int64]tg.InputPeerClass
	channelCache map[int64]*tg.Channel // For forum operations
	historyPacer *historyPacer
}

type Chat struct {
	ID           int64
	Title        string
	UnreadCount  int
	IsChannel    bool
	IsForum      bool
	IsUser       bool
	IsBot        bool
	LastReadID   int
	TopMessageID int
}

type Topic struct {
	ID           int
	Title        string
	UnreadCount  int
	LastReadID   int
	TopMessageID int
}

type Message struct {
	ID       int
	Date     time.Time
	Text     string
	SenderID int64
}

type ProgressUpdate struct {
	Phase   string
	Parsed  int
	Scanned int
	Batch   int
}

type ProgressFunc func(ProgressUpdate)

func reportProgress(progress ProgressFunc, update ProgressUpdate) {
	if progress != nil {
		progress(update)
	}
}

func (c *Client) recordFloodWait(time.Duration) {
	if c.historyPacer != nil {
		c.historyPacer.RecordFloodWait()
	}
}

func NewClient(cfg *config.Config) (*Client, error) {
	return &Client{
		cfg:          cfg,
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
		historyPacer: newHistoryPacer(cfg),
	}, nil
}

func (c *Client) Login(ctx context.Context, input io.Reader) error {
	// Configure logger
	var level slog.Level
	switch c.cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	// Ensure session directory exists
	if err := os.MkdirAll("session", 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	clientCtx, cancelClient := context.WithCancel(ctx)
	opts := &gotgproto.ClientOpts{
		Context:         clientCtx,
		Session:         sessionMaker.SqlSession(sqlite.Open("session/session.db")),
		AuthConversator: gotgproto.BasicConversator(),
		Middlewares: []telegram.Middleware{
			newFloodWaitMiddleware(time.Duration(c.cfg.FloodWaitMaxSeconds)*time.Second, c.recordFloodWait),
			ratelimit.New(rate.Every(time.Duration(c.cfg.RateLimitMs)*time.Millisecond), 3),
		},
	}

	if c.cfg.LogLevel == "debug" {
		opts.Middlewares = append(opts.Middlewares, MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
			return telegram.InvokeFunc(func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
				slog.Debug("TG Request", "method", fmt.Sprintf("%T", input))
				return next.Invoke(ctx, input, output)
			})
		}))
	}

	client, err := c.startClient(opts, cancelClient)
	if err != nil {
		cancelClient()
		return fmt.Errorf("failed to create client: %w", err)
	}

	c.proto = client
	c.ctx = client.CreateContext()

	return nil
}

type startClientResult struct {
	client *gotgproto.Client
	err    error
}

func (c *Client) startClient(opts *gotgproto.ClientOpts, cancelClient context.CancelFunc) (*gotgproto.Client, error) {
	resultCh := make(chan startClientResult, 1)
	go func() {
		client, err := gotgproto.NewClient(
			c.cfg.TelegramAppID,
			c.cfg.TelegramAppHash,
			gotgproto.ClientTypePhone(c.cfg.Phone),
			opts,
		)
		resultCh <- startClientResult{client: client, err: err}
	}()

	timeoutSeconds := c.cfg.TelegramConnectTimeoutSeconds
	if timeoutSeconds == 0 {
		result := <-resultCh
		return result.client, result.err
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	statusTimer := time.NewTimer(10 * time.Second)
	timeoutTimer := time.NewTimer(timeout)
	defer statusTimer.Stop()
	defer timeoutTimer.Stop()

	for {
		select {
		case result := <-resultCh:
			return result.client, result.err
		case <-statusTimer.C:
			fmt.Fprintf(os.Stderr, "Still connecting to Telegram after 10s; waiting up to %s before aborting.\n", timeout)
		case <-timeoutTimer.C:
			cancelClient()
			return nil, fmt.Errorf("telegram connection timed out after %ds; check network/proxy access to Telegram or increase TG_CONNECT_TIMEOUT_SECONDS", timeoutSeconds)
		}
	}
}

func (c *Client) GetDialogs(ctx context.Context) ([]Chat, error) {
	if c.ctx == nil {
		c.ctx = c.proto.CreateContext()
	}

	const limit = 100
	var parsedDialogs []Chat
	seen := make(map[int64]struct{})

	offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
	offsetID := 0
	offsetDate := 0

	for {
		dialogs, err := c.ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      limit,
			OffsetPeer: offsetPeer,
			OffsetID:   offsetID,
			OffsetDate: offsetDate,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get dialogs: %w", err)
		}

		var batch []Chat
		var lastDialog *tg.Dialog

		switch d := dialogs.(type) {
		case *tg.MessagesDialogsSlice:
			batch = c.processDialogs(d.Dialogs, d.Chats, d.Users)
			if len(d.Dialogs) > 0 {
				if dlg, ok := d.Dialogs[len(d.Dialogs)-1].(*tg.Dialog); ok {
					lastDialog = dlg
				}
			}
		case *tg.MessagesDialogs:
			batch = c.processDialogs(d.Dialogs, d.Chats, d.Users)
			if len(d.Dialogs) > 0 {
				if dlg, ok := d.Dialogs[len(d.Dialogs)-1].(*tg.Dialog); ok {
					lastDialog = dlg
				}
			}
		}

		for _, chat := range batch {
			if _, ok := seen[chat.ID]; ok {
				continue
			}
			seen[chat.ID] = struct{}{}
			parsedDialogs = append(parsedDialogs, chat)
		}

		if len(batch) < limit || lastDialog == nil {
			break
		}

		peerID := resolveSenderID(lastDialog.Peer)
		nextPeer, ok := c.peerCache[peerID]
		if !ok {
			nextPeer = c.ctx.PeerStorage.GetInputPeerById(peerID)
		}
		if nextPeer == nil {
			break
		}
		offsetPeer = nextPeer
		offsetID = lastDialog.TopMessage
		offsetDate = 0
	}

	// Sort by unread count desc
	sort.Slice(parsedDialogs, func(i, j int) bool {
		return parsedDialogs[i].UnreadCount > parsedDialogs[j].UnreadCount
	})

	return parsedDialogs, nil
}

func (c *Client) processDialogs(dialogs []tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass) []Chat {
	chatMap := make(map[int64]tg.ChatClass)
	for _, ch := range chats {
		chatMap[ch.GetID()] = ch
		switch item := ch.(type) {
		case *tg.Chat:
			c.peerCache[item.ID] = &tg.InputPeerChat{ChatID: item.ID}
		case *tg.Channel:
			c.peerCache[item.ID] = &tg.InputPeerChannel{ChannelID: item.ID, AccessHash: item.AccessHash}
			c.channelCache[item.ID] = item // Cache for forum operations
		}
	}
	userMap := make(map[int64]tg.UserClass)
	for _, u := range users {
		userMap[u.GetID()] = u
		switch item := u.(type) {
		case *tg.User:
			c.peerCache[item.ID] = &tg.InputPeerUser{UserID: item.ID, AccessHash: item.AccessHash}
		}
	}

	var results []Chat

	for _, d := range dialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}

		var title string
		var peerID int64
		var isChannel bool
		var isForum bool
		var isUser bool
		var isBot bool

		switch p := dlg.Peer.(type) {
		case *tg.PeerUser:
			peerID = p.UserID
			isUser = true
			if u, ok := userMap[peerID]; ok {
				switch user := u.(type) {
				case *tg.User:
					title = user.FirstName + " " + user.LastName
					if user.Username != "" {
						title += " (@" + user.Username + ")"
					}
					isBot = user.Bot
				}
			}
		case *tg.PeerChat:
			peerID = p.ChatID
			if ch, ok := chatMap[peerID]; ok {
				switch chat := ch.(type) {
				case *tg.Chat:
					title = chat.Title
				}
			}
		case *tg.PeerChannel:
			peerID = p.ChannelID
			isChannel = true
			if ch, ok := chatMap[peerID]; ok {
				switch channel := ch.(type) {
				case *tg.Channel:
					title = channel.Title
					isForum = channel.Forum
				}
			}
		}

		if title == "" {
			title = fmt.Sprintf("Unknown Peer %d", peerID)
		}

		results = append(results, Chat{
			ID:           peerID,
			Title:        title,
			UnreadCount:  dlg.UnreadCount,
			IsChannel:    isChannel,
			IsForum:      isForum,
			IsUser:       isUser,
			IsBot:        isBot,
			LastReadID:   dlg.ReadInboxMaxID,
			TopMessageID: dlg.TopMessage,
		})
	}
	return results
}

func (c *Client) GetUnreadMessages(ctx context.Context, chatID int64, lastReadID int, progress ProgressFunc) ([]Message, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found in cache or storage", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"unread",
		time.Time{},
		false,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			req := &tg.MessagesGetHistoryRequest{
				Peer:     inputPeer,
				Limit:    limit,
				OffsetID: offsetID,
			}
			return c.ctx.Raw.MessagesGetHistory(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if msg.ID <= lastReadID {
				return false, true // Stop
			}
			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		},
	)
}

func (c *Client) MarkAsRead(ctx context.Context, chat Chat, maxID int) error {
	inputPeer := c.resolvePeer(chat.ID)
	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chat.ID)
	}

	if chat.IsChannel {
		inputChannel, ok := inputPeer.(*tg.InputPeerChannel)
		if !ok {
			// Try to cast or reconstruct if possible, but peerCache should have correct type
			return fmt.Errorf("peer is marked as channel but input peer is %T", inputPeer)
		}

		// For channels/supergroups
		_, err := c.ctx.Raw.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{
				ChannelID:  inputChannel.ChannelID,
				AccessHash: inputChannel.AccessHash,
			},
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark channel as read: %w", err)
		}
	} else {
		// For users and basic groups
		_, err := c.ctx.Raw.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
			Peer:  inputPeer,
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark chat as read: %w", err)
		}
	}

	return nil
}

// GetForumTopics fetches all topics from a forum with their unread counts.
func (c *Client) GetForumTopics(ctx context.Context, chatID int64) ([]Topic, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	const limit = 100
	var result []Topic
	seen := make(map[int]struct{})
	offset := forumTopicOffset{}

	for {
		topics, err := c.ctx.Raw.MessagesGetForumTopics(ctx, &tg.MessagesGetForumTopicsRequest{
			Peer:        inputPeer,
			OffsetDate:  offset.date,
			OffsetID:    offset.id,
			OffsetTopic: offset.topic,
			Limit:       limit,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get forum topics: %w", err)
		}

		result = appendForumTopics(result, seen, topics.Topics)
		nextOffset, ok := nextForumTopicOffset(topics)
		if !ok || len(topics.Topics) < limit || nextOffset == offset {
			break
		}
		offset = nextOffset
	}

	return result, nil
}

// GetTopicMessages fetches unread messages from a specific topic.
func (c *Client) GetTopicMessages(ctx context.Context, chatID int64, topicID int, lastReadID int, progress ProgressFunc) ([]Message, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"topic-unread",
		time.Time{},
		false,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			if topicID == 1 {
				req := &tg.MessagesGetHistoryRequest{
					Peer:     inputPeer,
					Limit:    limit,
					OffsetID: offsetID,
				}
				return c.ctx.Raw.MessagesGetHistory(ctx, req)
			}
			req := &tg.MessagesGetRepliesRequest{
				Peer:     inputPeer,
				MsgID:    topicID,
				Limit:    limit,
				OffsetID: offsetID,
			}
			return c.ctx.Raw.MessagesGetReplies(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false // Skip message belonging to another topic
					}
				}
			}
			if msg.ID <= lastReadID {
				return false, true // Stop
			}
			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		},
	)
}

// MarkTopicAsRead marks a specific topic as read up to the given message ID.
func (c *Client) MarkTopicAsRead(ctx context.Context, chatID int64, topicID int, maxID int) error {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chatID)
	}

	_, err := c.ctx.Raw.MessagesReadDiscussion(ctx, &tg.MessagesReadDiscussionRequest{
		Peer:      inputPeer,
		MsgID:     topicID,
		ReadMaxID: maxID,
	})
	if err != nil {
		return fmt.Errorf("failed to mark topic as read: %w", err)
	}

	return nil
}

// GetMessagesByDate fetches messages within a specific date range.
func (c *Client) GetMessagesByDate(ctx context.Context, chatID int64, since, until time.Time, progress ProgressFunc) ([]Message, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"date-range",
		until,
		true,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			req := &tg.MessagesGetHistoryRequest{
				Peer:       inputPeer,
				Limit:      limit,
				OffsetID:   offsetID,
				OffsetDate: offsetDate,
			}
			return c.ctx.Raw.MessagesGetHistory(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			msgTime := time.Unix(int64(msg.Date), 0)
			if msgTime.Before(since) {
				return false, true // Stop (tooOld)
			}
			if msgTime.After(until) {
				return false, false // Skip (tooNew)
			}
			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		},
	)
}

// GetTopicMessagesByDate fetches topic messages within a specific date range.
func (c *Client) GetTopicMessagesByDate(ctx context.Context, chatID int64, topicID int, since, until time.Time, progress ProgressFunc) ([]Message, error) {
	inputPeer := c.resolvePeer(chatID)
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	return c.fetchMessages(
		ctx,
		progress,
		"topic-date-range",
		until,
		true,
		func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error) {
			if topicID == 1 {
				req := &tg.MessagesGetHistoryRequest{
					Peer:       inputPeer,
					Limit:      limit,
					OffsetID:   offsetID,
					OffsetDate: offsetDate,
				}
				return c.ctx.Raw.MessagesGetHistory(ctx, req)
			}
			req := &tg.MessagesGetRepliesRequest{
				Peer:       inputPeer,
				MsgID:      topicID,
				Limit:      limit,
				OffsetID:   offsetID,
				OffsetDate: offsetDate,
			}
			return c.ctx.Raw.MessagesGetReplies(ctx, req)
		},
		func(msg *tg.Message) (bool, bool) {
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false
					}
				}
			}
			msgTime := time.Unix(int64(msg.Date), 0)
			if msgTime.Before(since) {
				return false, true // Stop (tooOld)
			}
			if msgTime.After(until) {
				return false, false // Skip (tooNew)
			}
			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		},
	)
}

func (c *Client) resolvePeer(chatID int64) tg.InputPeerClass {
	if inputPeer, ok := c.peerCache[chatID]; ok {
		return inputPeer
	}
	if c.ctx == nil || c.ctx.PeerStorage == nil {
		return nil
	}
	return c.ctx.PeerStorage.GetInputPeerById(chatID)
}

func resolveSenderID(fromID tg.PeerClass) int64 {
	if fromID == nil {
		return 0
	}
	switch p := fromID.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChannel:
		return p.ChannelID
	case *tg.PeerChat:
		return p.ChatID
	default:
		return 0
	}
}

type forumTopicOffset struct {
	date  int
	id    int
	topic int
}

func appendForumTopics(result []Topic, seen map[int]struct{}, topics []tg.ForumTopicClass) []Topic {
	for _, t := range topics {
		topic, ok := t.(*tg.ForumTopic)
		if !ok {
			continue
		}
		if _, ok := seen[topic.ID]; ok {
			continue
		}
		seen[topic.ID] = struct{}{}
		result = append(result, Topic{
			ID:           topic.ID,
			Title:        topic.Title,
			UnreadCount:  topic.UnreadCount,
			LastReadID:   topic.ReadInboxMaxID,
			TopMessageID: topic.TopMessage,
		})
	}
	return result
}

func nextForumTopicOffset(topics *tg.MessagesForumTopics) (forumTopicOffset, bool) {
	for i := len(topics.Topics) - 1; i >= 0; i-- {
		topic, ok := topics.Topics[i].(*tg.ForumTopic)
		if !ok {
			continue
		}
		offsetDate := topic.Date
		if !topics.OrderByCreateDate {
			if date := messageDateByID(topics.Messages, topic.TopMessage); date != 0 {
				offsetDate = date
			}
		}
		return forumTopicOffset{
			date:  offsetDate,
			id:    topic.TopMessage,
			topic: topic.ID,
		}, true
	}
	return forumTopicOffset{}, false
}

func messageDateByID(messages []tg.MessageClass, id int) int {
	for _, m := range messages {
		if m.GetID() != id {
			continue
		}
		switch msg := m.(type) {
		case *tg.Message:
			return msg.Date
		case *tg.MessageService:
			return msg.Date
		}
	}
	return 0
}

func extractMessages(result tg.MessagesMessagesClass) []tg.MessageClass {
	switch r := result.(type) {
	case *tg.MessagesMessages:
		return r.Messages
	case *tg.MessagesMessagesSlice:
		return r.Messages
	case *tg.MessagesChannelMessages:
		return r.Messages
	}
	return nil
}

func (c *Client) fetchMessages(
	ctx context.Context,
	progress ProgressFunc,
	phase string,
	until time.Time,
	useOffsetDate bool,
	fetch func(offsetID, offsetDate, limit int) (tg.MessagesMessagesClass, error),
	filter func(msg *tg.Message) (process bool, stop bool),
) ([]Message, error) {
	var allMessages []Message
	offsetID := 0
	batchSize := 100
	page := 0

	for {
		offsetDate := 0
		if useOffsetDate && offsetID == 0 && !until.IsZero() {
			// Jump close to the end of the requested range, then page backwards.
			offsetDate = int(until.Unix())
			reportProgress(progress, ProgressUpdate{
				Phase: fmt.Sprintf("jumped to date %s", until.Format("2006-01-02")),
			})
		}

		if page > 0 && c.historyPacer != nil {
			if err := c.historyPacer.Wait(ctx, progress); err != nil {
				return nil, fmt.Errorf("history request pause: %w", err)
			}
		}

		result, err := fetch(offsetID, offsetDate, batchSize)
		if err != nil {
			return nil, err
		}
		if c.historyPacer != nil {
			c.historyPacer.RecordSuccess()
		}
		page++

		msgs := extractMessages(result)
		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := processMessageBatch(msgs, filter)
		allMessages = append(allMessages, batchMessages...)
		reportProgress(progress, ProgressUpdate{
			Phase:   phase,
			Parsed:  len(batchMessages),
			Scanned: len(msgs),
			Batch:   1,
		})

		if stop {
			break
		}
		if lastID == 0 {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

func processMessageBatch(msgs []tg.MessageClass,
	filter func(msg *tg.Message) (process bool, stop bool)) ([]Message, int, bool) {

	var results []Message
	var lastID int
	var stopLoop bool

	for _, m := range msgs {
		lastID = m.GetID()

		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}

		process, stop := filter(msg)
		if stop {
			stopLoop = true
			break
		}
		if !process {
			continue
		}

		senderID := resolveSenderID(msg.FromID)
		results = append(results, Message{
			ID:       msg.ID,
			Date:     time.Unix(int64(msg.Date), 0),
			Text:     msg.Message,
			SenderID: senderID,
		})
	}
	return results, lastID, stopLoop
}

// Helpers for middleware

type MiddlewareFunc func(next tg.Invoker) telegram.InvokeFunc

func (m MiddlewareFunc) Handle(next tg.Invoker) telegram.InvokeFunc {
	return m(next)
}
