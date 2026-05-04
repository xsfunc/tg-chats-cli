package exporter

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"tg-arc/internal/store"
)

type Options struct {
	AccountID  int64
	ChatID     int64
	TopicID    int
	TopicIDSet bool
	Since      time.Time
	Until      time.Time
	UseSince   bool
	UseUntil   bool
	Location   *time.Location
}

func ListChats(ctx context.Context, db *store.SQLiteStore, w io.Writer, opts Options) error {
	location := opts.Location
	if location == nil {
		location = time.Local
	}
	accountID, err := db.ResolveAccountID(ctx, opts.AccountID)
	if err != nil {
		return err
	}
	account, err := db.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	chats, err := db.ListChatSummaries(ctx, accountID)
	if err != nil {
		return err
	}

	if err := writef(w, "Account: %s\n\n", FormatAccount(account)); err != nil {
		return fmt.Errorf("write account header: %w", err)
	}
	if len(chats) == 0 {
		if err := writeln(w, "No chats found."); err != nil {
			return fmt.Errorf("write empty chat list: %w", err)
		}
		return nil
	}
	if err := writeln(w, "Chats:"); err != nil {
		return fmt.Errorf("write chat list header: %w", err)
	}
	if err := writeln(w, "chat_id\tmessages\trange\ttitle"); err != nil {
		return fmt.Errorf("write chat list columns: %w", err)
	}
	for _, chat := range chats {
		if err := writef(w, "%d\t%d\t%s\t%s\n", chat.ID, chat.MessageCount, formatRange(chat.FirstMessage, chat.LastMessage, location), chat.Title); err != nil {
			return fmt.Errorf("write chat %d: %w", chat.ID, err)
		}
		topics, err := db.ListTopicSummaries(ctx, accountID, chat.ID)
		if err != nil {
			return err
		}
		if len(topics) == 0 {
			continue
		}
		if err := writef(w, "  Topics for %d:\n", chat.ID); err != nil {
			return fmt.Errorf("write topic header for chat %d: %w", chat.ID, err)
		}
		if err := writeln(w, "  topic_id\tmessages\trange\ttitle"); err != nil {
			return fmt.Errorf("write topic columns for chat %d: %w", chat.ID, err)
		}
		for _, topic := range topics {
			if err := writef(w, "  %d\t%d\t%s\t%s\n", topic.ID, topic.MessageCount, formatRange(topic.FirstMessage, topic.LastMessage, location), topic.Title); err != nil {
				return fmt.Errorf("write topic %d/%d: %w", chat.ID, topic.ID, err)
			}
		}
	}
	return nil
}

func ExportMarkdown(ctx context.Context, db *store.SQLiteStore, w io.Writer, opts Options) error {
	location := opts.Location
	if location == nil {
		location = time.Local
	}
	accountID, err := db.ResolveAccountID(ctx, opts.AccountID)
	if err != nil {
		return err
	}
	data, err := db.ExportMessages(ctx, store.MessageQuery{
		AccountID:  accountID,
		ChatID:     opts.ChatID,
		TopicID:    opts.TopicID,
		TopicIDSet: opts.TopicIDSet,
		Since:      opts.Since,
		Until:      opts.Until,
		UseSince:   opts.UseSince,
		UseUntil:   opts.UseUntil,
	})
	if err != nil {
		return err
	}
	if err := RenderMarkdown(w, data, location); err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}
	return nil
}

func RenderMarkdown(w io.Writer, data store.ExportData, location *time.Location) error {
	if location == nil {
		location = time.Local
	}
	if err := writeln(w, "# Telegram Chat Export"); err != nil {
		return err
	}
	if err := writeln(w); err != nil {
		return err
	}
	if err := writef(w, "Account: %s\n", FormatAccount(data.Account)); err != nil {
		return err
	}
	if err := writef(w, "Chat: %s, chat_id=%d\n", data.Chat.Title, data.Chat.ID); err != nil {
		return err
	}
	if data.HasTopic {
		if err := writef(w, "Topic: %s, topic_id=%d\n", formatTopicTitle(data.Topic), data.Topic.ID); err != nil {
			return err
		}
	}
	if err := writef(w, "Range: %s\n", messageRange(data.Messages, location)); err != nil {
		return err
	}
	if err := writef(w, "Messages: %d\n", len(data.Messages)); err != nil {
		return err
	}
	if err := writeln(w); err != nil {
		return err
	}
	if err := writeln(w, "## Transcript"); err != nil {
		return err
	}
	for _, msg := range data.Messages {
		if err := writeln(w); err != nil {
			return err
		}
		if err := writef(w, "[%s] %s:\n", formatTime(msg.Date, location), FormatSender(msg)); err != nil {
			return err
		}
		if err := writeln(w, msg.Text); err != nil {
			return err
		}
	}
	return nil
}

func FormatAccount(account store.AccountSummary) string {
	name := strings.TrimSpace(strings.Join(nonEmpty(account.FirstName, account.LastName), " "))
	username := formatUsername(account.Username)
	switch {
	case name != "" && username != "":
		return fmt.Sprintf("%s (%s), account_id=%d", name, username, account.ID)
	case name != "":
		return fmt.Sprintf("%s, account_id=%d", name, account.ID)
	case username != "":
		return fmt.Sprintf("%s, account_id=%d", username, account.ID)
	case account.HasTelegramUserID:
		return fmt.Sprintf("telegram_user_id=%d, account_id=%d", account.TelegramUserID, account.ID)
	default:
		return fmt.Sprintf("account_id=%d", account.ID)
	}
}

func FormatSender(msg store.ExportMessage) string {
	if msg.Outgoing {
		return "Me"
	}
	name := strings.TrimSpace(strings.Join(nonEmpty(msg.SenderFirstName, msg.SenderLastName), " "))
	username := formatUsername(msg.SenderUsername)
	switch {
	case name != "" && username != "":
		return fmt.Sprintf("%s (%s)", name, username)
	case name != "":
		return name
	case username != "":
		return username
	case msg.SenderID != 0:
		return fmt.Sprintf("sender_id=%d", msg.SenderID)
	default:
		return "Unknown"
	}
}

func nonEmpty(values ...string) []string {
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func formatUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	if strings.HasPrefix(username, "@") {
		return username
	}
	return "@" + username
}

func formatTopicTitle(topic store.TopicSummary) string {
	title := strings.TrimSpace(topic.Title)
	if title == "" {
		return fmt.Sprintf("topic_id=%d", topic.ID)
	}
	return title
}

func messageRange(messages []store.ExportMessage, location *time.Location) string {
	if len(messages) == 0 {
		return "(no messages)"
	}
	return formatRange(messages[0].Date, messages[len(messages)-1].Date, location)
}

func formatRange(first time.Time, last time.Time, location *time.Location) string {
	if first.IsZero() || last.IsZero() {
		return "(no messages)"
	}
	return fmt.Sprintf("%s .. %s", formatTime(first, location), formatTime(last, location))
}

func formatTime(value time.Time, location *time.Location) string {
	return value.In(location).Format("2006-01-02 15:04 -07:00")
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeln(w io.Writer, args ...any) error {
	_, err := fmt.Fprintln(w, args...)
	return err
}
