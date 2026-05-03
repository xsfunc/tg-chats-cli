package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"cli-tg-chat-summary/internal/config"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/bin"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"
)

func NewClient(cfg *config.Config) (*Client, error) {
	return &Client{
		cfg:          cfg,
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
		historyPacer: newHistoryPacer(cfg),
	}, nil
}

func (c *Client) Login(ctx context.Context, input io.Reader) error {
	_ = input

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

	sessionPath := c.cfg.SessionPath
	if sessionPath == "" {
		sessionPath = "session/session.db"
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	clientCtx, cancelClient := context.WithCancel(ctx)
	opts := &gotgproto.ClientOpts{
		Context:         clientCtx,
		Session:         sessionMaker.SqlSession(sqlite.Open(sessionPath)),
		AuthConversator: gotgproto.BasicConversator(),
		Middlewares: []gotdtelegram.Middleware{
			newFloodWaitMiddleware(time.Duration(c.cfg.FloodWaitMaxSeconds)*time.Second, c.recordFloodWait),
			ratelimit.New(rate.Every(time.Duration(c.cfg.RateLimitMs)*time.Millisecond), 3),
		},
	}

	if c.cfg.LogLevel == "debug" {
		opts.Middlewares = append(opts.Middlewares, MiddlewareFunc(func(next tg.Invoker) gotdtelegram.InvokeFunc {
			return gotdtelegram.InvokeFunc(func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
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

func (c *Client) Account() (Account, error) {
	if c.proto == nil || c.proto.Self == nil {
		return Account{}, fmt.Errorf("telegram client is not logged in")
	}
	self := c.proto.Self
	return Account{
		TelegramUserID: self.ID,
		Username:       self.Username,
		FirstName:      self.FirstName,
		LastName:       self.LastName,
		Phone:          self.Phone,
		IsBot:          self.Bot,
	}, nil
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
