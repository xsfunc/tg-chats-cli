package telegram

import (
	"time"

	"tg-arc/internal/config"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

type Client struct {
	cfg              *config.Config
	proto            *gotgproto.Client
	ctx              *ext.Context
	peerCache        map[int64]tg.InputPeerClass
	channelCache     map[int64]*tg.Channel
	historyPacer     *historyPacer
	startProtoClient func(*gotgproto.ClientOpts) (*gotgproto.Client, error)
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
	Outgoing bool
	EditDate time.Time
}

type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	IsBot     bool
}

type Account struct {
	TelegramUserID int64
	Username       string
	FirstName      string
	LastName       string
	Phone          string
	IsBot          bool
}

type DialogsResult struct {
	Chats []Chat
	Users []User
}

type MessageFetchOptions struct {
	MessageLimit int
}

type MessageFetchResult struct {
	Messages  []Message
	Users     []User
	Chats     []Chat
	Truncated bool
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
