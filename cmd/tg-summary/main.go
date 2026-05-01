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

	"cli-tg-chat-summary/internal/app"
	"cli-tg-chat-summary/internal/config"
	"cli-tg-chat-summary/internal/telegram"
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

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Setup context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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

func parseRunOptions(args []string, now func() time.Time) (app.RunOptions, error) {
	var sinceStr, untilStr string
	var formatName string
	var chatIDRaw int64
	var topicID int
	var topicTitle string

	fs := flag.NewFlagSet("tg-summary", flag.ContinueOnError)
	fs.StringVar(&sinceStr, "since", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&untilStr, "until", "", "End date (YYYY-MM-DD)")
	fs.StringVar(&formatName, "format", "text", "Export format (text, xml, xml-compact)")
	fs.Int64Var(&chatIDRaw, "id", 0, "Chat ID (raw or -100... format) to export without TUI")
	fs.IntVar(&topicID, "topic-id", 0, "Forum topic ID (required for forum chats in non-interactive mode)")
	fs.StringVar(&topicTitle, "topic", "", "Forum topic title (alternative to --topic-id)")
	if err := fs.Parse(args); err != nil {
		return app.RunOptions{}, err
	}

	var opts app.RunOptions
	var err error
	opts.ExportFormat = formatName

	if chatIDRaw != 0 {
		opts.NonInteractive = true
		opts.ChatIDRaw = chatIDRaw
		// Telegram Bot API uses -100... IDs for channels/supergroups.
		// Dialogs return raw ChannelID, so normalize by stripping the -100 prefix.
		opts.ChatID = normalizeChatID(chatIDRaw)
		opts.TopicID = topicID
		opts.TopicTitle = topicTitle
	}

	if (topicID != 0 || topicTitle != "") && chatIDRaw == 0 {
		return app.RunOptions{}, fmt.Errorf("--topic-id/--topic requires --id")
	}

	if untilStr != "" && sinceStr == "" {
		return app.RunOptions{}, fmt.Errorf("--until requires --since")
	}

	if sinceStr != "" {
		opts.UseDateRange = true
		opts.Since, err = time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return app.RunOptions{}, fmt.Errorf("invalid date format for --since %q; use YYYY-MM-DD (e.g., 2024-01-20)", sinceStr)
		}
		if untilStr != "" {
			opts.Until, err = time.Parse("2006-01-02", untilStr)
			if err != nil {
				return app.RunOptions{}, fmt.Errorf("invalid date format for --until %q; use YYYY-MM-DD (e.g., 2024-01-20)", untilStr)
			}
			if opts.Until.Before(opts.Since) {
				return app.RunOptions{}, fmt.Errorf("--until cannot be before --since")
			}
			// set until to end of that day
			opts.Until = opts.Until.Add(24 * time.Hour).Add(-time.Nanosecond)
		} else {
			opts.Until = now()
		}
	}

	return opts, nil
}

func normalizeChatID(id int64) int64 {
	if id <= -1000000000000 {
		return -id - 1000000000000
	}
	return id
}
