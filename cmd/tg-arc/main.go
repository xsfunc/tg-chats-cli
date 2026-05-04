package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"tg-arc/internal/app"
	"tg-arc/internal/config"
	"tg-arc/internal/exporter"
	"tg-arc/internal/store"
	"tg-arc/internal/telegram"
)

func main() {
	opts, err := parseRunOptions(os.Args[1:], time.Now)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if opts.Command == "chats" || opts.Command == "export" {
		if err := runOfflineCommand(ctx, opts); err != nil {
			log.Fatalf("Application error: %v", err)
		}
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize Telegram client
	tgClient, err := telegram.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create telegram client: %v", err)
	}

	// Initialize function app
	application := app.New(cfg, tgClient)

	// Run application
	if err := application.Run(ctx, opts); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func runOfflineCommand(ctx context.Context, opts app.RunOptions) error {
	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = store.DefaultPath
	}
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite store: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	exportOpts := exporter.Options{
		AccountID:  opts.AccountID,
		ChatID:     opts.ChatID,
		TopicID:    opts.TopicID,
		TopicIDSet: opts.TopicIDSet,
		Since:      opts.Since,
		Until:      opts.Until,
		UseSince:   opts.UseDateRange,
		UseUntil:   opts.UseUntil,
		Location:   time.Local,
	}
	if opts.Command == "chats" {
		return exporter.ListChats(ctx, db, os.Stdout, exportOpts)
	}
	return exporter.ExportMarkdown(ctx, db, os.Stdout, exportOpts)
}

func parseRunOptions(args []string, now func() time.Time) (app.RunOptions, error) {
	var sinceStr, untilStr string
	var formatName string
	var chatIDRaw int64
	var topicID int
	var topicTitle string
	var dbPath string
	var sessionPath string
	var accountID int64
	var chatLimit int
	var messageLimit int
	command := "history"

	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		switch args[0] {
		case "history", "sync", "chats", "export":
			command = args[0]
			args = args[1:]
		default:
			return app.RunOptions{}, fmt.Errorf("unknown command %q (available: history, sync, chats, export)", args[0])
		}
	}

	fs := flag.NewFlagSet("tg-arc", flag.ContinueOnError)
	fs.StringVar(&sinceStr, "since", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&untilStr, "until", "", "End date (YYYY-MM-DD)")
	fs.StringVar(&formatName, "format", "", "Deprecated: file export format is not supported in DB modes")
	fs.Int64Var(&chatIDRaw, "id", 0, "Chat ID (raw or -100... format) to save without TUI")
	fs.IntVar(&topicID, "topic-id", 0, "Forum topic ID (required for forum chats in non-interactive mode)")
	fs.StringVar(&topicTitle, "topic", "", "Forum topic title (alternative to --topic-id)")
	fs.StringVar(&dbPath, "db", "", "SQLite database path")
	fs.StringVar(&sessionPath, "session", "", "Telegram session SQLite path")
	fs.Int64Var(&accountID, "account-id", 0, "SQLite account_id to use when the database contains multiple accounts")
	fs.IntVar(&chatLimit, "chat-limit", 0, "Maximum unread chats to sync (0 means unlimited)")
	fs.IntVar(&messageLimit, "message-limit", 0, "Maximum messages per chat/topic (0 means unlimited)")
	if err := fs.Parse(args); err != nil {
		return app.RunOptions{}, err
	}
	visited := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	if visited["format"] {
		return app.RunOptions{}, fmt.Errorf("--format is not supported in DB modes")
	}
	if chatLimit < 0 {
		return app.RunOptions{}, fmt.Errorf("--chat-limit cannot be negative")
	}
	if messageLimit < 0 {
		return app.RunOptions{}, fmt.Errorf("--message-limit cannot be negative")
	}
	if accountID < 0 {
		return app.RunOptions{}, fmt.Errorf("--account-id cannot be negative")
	}
	if command == "sync" && (sinceStr != "" || untilStr != "") {
		return app.RunOptions{}, fmt.Errorf("sync does not support --since/--until; it only saves unread messages")
	}
	if command == "history" && chatLimit != 0 {
		return app.RunOptions{}, fmt.Errorf("--chat-limit is only supported by sync")
	}
	if (command == "chats" || command == "export") && sessionPath != "" {
		return app.RunOptions{}, fmt.Errorf("--session is only supported by history and sync")
	}
	if (command == "chats" || command == "export") && (chatLimit != 0 || messageLimit != 0) {
		return app.RunOptions{}, fmt.Errorf("--chat-limit/--message-limit are only supported by history and sync")
	}
	if command == "chats" && (chatIDRaw != 0 || topicID != 0 || topicTitle != "" || sinceStr != "" || untilStr != "") {
		return app.RunOptions{}, fmt.Errorf("chats only supports --db and --account-id")
	}
	if command == "export" && chatIDRaw == 0 {
		return app.RunOptions{}, fmt.Errorf("export requires --id")
	}
	if command == "export" && topicTitle != "" {
		return app.RunOptions{}, fmt.Errorf("export supports --topic-id, not --topic")
	}

	var opts app.RunOptions
	var err error
	opts.Command = command
	opts.DBPath = dbPath
	opts.SessionPath = sessionPath
	opts.AccountID = accountID
	opts.ChatLimit = chatLimit
	opts.MessageLimit = messageLimit
	if command == "sync" || command == "export" || command == "chats" {
		opts.NonInteractive = true
	}

	if chatIDRaw != 0 {
		opts.NonInteractive = true
		opts.ChatIDRaw = chatIDRaw
		// Telegram Bot API uses -100... IDs for channels/supergroups.
		// Dialogs return raw ChannelID, so normalize by stripping the -100 prefix.
		opts.ChatID = normalizeChatID(chatIDRaw)
		opts.TopicID = topicID
		opts.TopicIDSet = visited["topic-id"]
		opts.TopicTitle = topicTitle
	}

	if (topicID != 0 || topicTitle != "") && chatIDRaw == 0 {
		return app.RunOptions{}, fmt.Errorf("--topic-id/--topic requires --id")
	}

	if command != "export" && untilStr != "" && sinceStr == "" {
		return app.RunOptions{}, fmt.Errorf("--until requires --since")
	}

	if sinceStr != "" {
		opts.UseDateRange = true
		opts.Since, err = parseDate(command, sinceStr)
		if err != nil {
			return app.RunOptions{}, fmt.Errorf("invalid date format for --since %q; use YYYY-MM-DD (e.g., 2024-01-20)", sinceStr)
		}
	}
	if untilStr != "" {
		opts.UseUntil = true
		opts.Until, err = parseDate(command, untilStr)
		if err != nil {
			return app.RunOptions{}, fmt.Errorf("invalid date format for --until %q; use YYYY-MM-DD (e.g., 2024-01-20)", untilStr)
		}
		if opts.UseDateRange && opts.Until.Before(opts.Since) {
			return app.RunOptions{}, fmt.Errorf("--until cannot be before --since")
		}
		opts.Until = opts.Until.Add(24 * time.Hour).Add(-time.Nanosecond)
	} else if command != "export" && sinceStr != "" {
		opts.UseUntil = true
		opts.Until = now()
	}
	return opts, nil
}

func parseDate(command string, value string) (time.Time, error) {
	if command == "export" {
		return time.ParseInLocation("2006-01-02", value, time.Local)
	}
	return time.Parse("2006-01-02", value)
}

func normalizeChatID(id int64) int64 {
	if id <= -1000000000000 {
		return -id - 1000000000000
	}
	return id
}
