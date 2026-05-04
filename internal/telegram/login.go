package telegram

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"tg-arc/internal/config"

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
	client := &Client{
		cfg:          cfg,
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
		historyPacer: newHistoryPacer(cfg),
	}
	client.startProtoClient = client.newProtoClient
	return client, nil
}

func (c *Client) Login(ctx context.Context, input io.Reader) error {
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
	authPromptActive := &atomic.Bool{}
	authPromptWake := make(chan struct{}, 1)
	opts := &gotgproto.ClientOpts{
		Context:         clientCtx,
		Session:         sessionMaker.SqlSession(sqlite.Open(sessionPath)),
		AuthConversator: newTerminalAuthConversator(input, os.Stdout, authPromptActive, authPromptWake),
		Middlewares: []gotdtelegram.Middleware{
			newFloodWaitMiddleware(time.Duration(c.cfg.FloodWaitMaxSeconds)*time.Second, c.recordFloodWait),
			ratelimit.New(rate.Every(time.Duration(c.cfg.RateLimitMs)*time.Millisecond), 3),
		},
	}
	if c.cfg.ProxyURL != "" {
		resolver, err := newProxyResolver(c.cfg.ProxyURL)
		if err != nil {
			cancelClient()
			return fmt.Errorf("configure telegram proxy: %w", err)
		}
		opts.Resolver = resolver
	}

	if c.cfg.LogLevel == "debug" {
		opts.Middlewares = append(opts.Middlewares, MiddlewareFunc(func(next tg.Invoker) gotdtelegram.InvokeFunc {
			return gotdtelegram.InvokeFunc(func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
				slog.Debug("TG Request", "method", fmt.Sprintf("%T", input))
				return next.Invoke(ctx, input, output)
			})
		}))
	}

	client, err := c.startClient(opts, cancelClient, authPromptActive, authPromptWake)
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

func (c *Client) newProtoClient(opts *gotgproto.ClientOpts) (*gotgproto.Client, error) {
	return gotgproto.NewClient(
		c.cfg.TelegramAppID,
		c.cfg.TelegramAppHash,
		gotgproto.ClientTypePhone(c.cfg.Phone),
		opts,
	)
}

func (c *Client) startClient(
	opts *gotgproto.ClientOpts,
	cancelClient context.CancelFunc,
	authPromptActive *atomic.Bool,
	authPromptWake <-chan struct{},
) (*gotgproto.Client, error) {
	resultCh := make(chan startClientResult, 1)
	startProtoClient := c.startProtoClient
	if startProtoClient == nil {
		startProtoClient = c.newProtoClient
	}
	go func() {
		client, err := startProtoClient(opts)
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
	timeoutPaused := false

	for {
		select {
		case result := <-resultCh:
			return result.client, result.err
		case <-authPromptWake:
			timeoutPaused = setTimeoutPaused(timeoutTimer, timeout, timeoutPaused, authPromptActive.Load())
		case <-statusTimer.C:
			if !authPromptActive.Load() {
				fmt.Fprintf(os.Stderr, "Still connecting to Telegram after 10s; waiting up to %s before aborting.\n", timeout)
			}
		case <-timeoutTimer.C:
			if authPromptActive.Load() {
				timeoutPaused = true
				continue
			}
			cancelClient()
			return nil, fmt.Errorf("telegram connection timed out after %ds; check network/proxy access to Telegram or increase TG_CONNECT_TIMEOUT_SECONDS", timeoutSeconds)
		}
	}
}

func setTimeoutPaused(timer *time.Timer, timeout time.Duration, paused, promptActive bool) bool {
	if promptActive {
		if !paused {
			stopTimer(timer)
		}
		return true
	}
	if paused {
		timer.Reset(timeout)
	}
	return false
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

type terminalAuthConversator struct {
	reader       *bufio.Reader
	output       io.Writer
	authStatus   gotgproto.AuthStatus
	promptActive *atomic.Bool
	promptWake   chan<- struct{}
}

func newTerminalAuthConversator(
	input io.Reader,
	output io.Writer,
	promptActive *atomic.Bool,
	promptWake chan<- struct{},
) gotgproto.AuthConversator {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	return &terminalAuthConversator{
		reader:       bufio.NewReader(input),
		output:       output,
		promptActive: promptActive,
		promptWake:   promptWake,
	}
}

func (c *terminalAuthConversator) AskPhoneNumber() (string, error) {
	if c.authStatus.Event == gotgproto.AuthStatusPhoneRetrial {
		if err := c.printRetry(
			"The phone number you just entered seems to be incorrect,",
			c.authStatus.AttemptsLeft,
		); err != nil {
			return "", err
		}
	}
	return c.ask("Enter Phone Number: ")
}

func (c *terminalAuthConversator) AskPassword() (string, error) {
	if c.authStatus.Event == gotgproto.AuthStatusPasswordRetrial {
		if err := c.printRetry(
			"The 2FA password you just entered seems to be incorrect,",
			c.authStatus.AttemptsLeft,
		); err != nil {
			return "", err
		}
	}
	return c.ask("Enter 2FA password: ")
}

func (c *terminalAuthConversator) AskCode() (string, error) {
	if c.authStatus.Event == gotgproto.AuthStatusPhoneCodeRetrial {
		if err := c.printRetry(
			"The OTP you just entered seems to be incorrect,",
			c.authStatus.AttemptsLeft,
		); err != nil {
			return "", err
		}
	}
	return c.ask("Enter Code: ")
}

func (c *terminalAuthConversator) AuthStatus(authStatus gotgproto.AuthStatus) {
	c.authStatus = authStatus
}

func (c *terminalAuthConversator) ask(prompt string) (string, error) {
	if _, err := fmt.Fprint(c.output, prompt); err != nil {
		return "", fmt.Errorf("write auth prompt: %w", err)
	}
	c.setPromptActive(true)
	defer c.setPromptActive(false)
	return c.reader.ReadString('\n')
}

func (c *terminalAuthConversator) printRetry(message string, attemptsLeft int) error {
	if _, err := fmt.Fprintln(c.output, message); err != nil {
		return fmt.Errorf("write auth retry message: %w", err)
	}
	if _, err := fmt.Fprintln(c.output, "Attempts Left:", attemptsLeft); err != nil {
		return fmt.Errorf("write auth retry attempts: %w", err)
	}
	if _, err := fmt.Fprintln(c.output, "Please try again...."); err != nil {
		return fmt.Errorf("write auth retry instruction: %w", err)
	}
	return nil
}

func (c *terminalAuthConversator) setPromptActive(active bool) {
	if c.promptActive != nil {
		c.promptActive.Store(active)
	}
	if c.promptWake == nil {
		return
	}
	select {
	case c.promptWake <- struct{}{}:
	default:
	}
}
