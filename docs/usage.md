# CLI Usage

## Commands

Build the binary:

```bash
mise run build
```

Run commands:

```bash
./bin/tg-arc [history] [flags]
./bin/tg-arc sync [flags]
./bin/tg-arc chats [flags]
./bin/tg-arc export --id <chat-id> [flags]
```

No command means `history`. `history` and `sync` connect to Telegram. `chats` and `export` are offline SQLite commands and do not require Telegram credentials.

From source, use `mise dev -- <args>`:

```bash
mise dev -- history --since 2024-01-01
mise dev -- sync --chat-limit 5
mise dev -- export --id 123456789 > chat.md
```

## History

Interactive history opens the TUI, lets you choose a chat or forum topic, saves incoming text messages, and returns to the chat list after the summary screen.

```bash
./bin/tg-arc
./bin/tg-arc history
```

Unread mode saves unread incoming text messages and marks them as read after a successful DB write. Date range mode saves incoming text messages from the selected range and never marks messages as read.

```bash
# Save from a specific UTC date until now.
./bin/tg-arc history --since 2024-01-01

# Save a specific UTC date range.
./bin/tg-arc history --since 2024-01-01 --until 2024-01-31
```

TUI keys:

- `ctrl+c`: exit.
- `q` or `esc`: exit from chat list; go back from topic list.
- `enter`: select the highlighted chat, topic, or mode.
- `q` during parsing: stop after the current request and save parsed messages. Unread mode marks only a safe parsed prefix as read; date range mode never marks messages as read.
- `m`: switch between unread and date range history modes.
- `ctrl+r`: mark selected chat as read; for forum chats, mark all unread topics.

Interactive mode requires a real terminal. Use `history --id` or `sync` for scripts, cron, pipes, or non-TTY wrappers.

## Non-Interactive History

Use `--id` to skip the TUI. It accepts raw MTProto channel IDs and Bot API style `-100...` IDs.

```bash
# Save unread messages from one chat.
./bin/tg-arc history --id 123456789

# Same command with a Bot API style supergroup/channel ID.
./bin/tg-arc history --id -1001234567890

# Save a range without marking messages as read.
./bin/tg-arc history --id 123456789 --since 2024-01-01 --until 2024-01-31
```

For forum chats, non-interactive `history` requires a topic:

```bash
./bin/tg-arc history --id 123456789 --topic-id 42
./bin/tg-arc history --id 123456789 --topic "Release Notes"
```

Topic title matching is case-insensitive. Exact matches are preferred; partial matches must be unique.

## Sync

`sync` saves unread messages from unread chats and marks saved ranges as read after successful DB writes.

```bash
# Sync every unread chat/topic.
./bin/tg-arc sync

# Limit breadth and messages per chat/topic.
./bin/tg-arc sync --chat-limit 5 --message-limit 50

# Sync one chat; for a forum without topic flags, all unread topics are synced.
./bin/tg-arc sync --id 123456789
./bin/tg-arc sync --id 123456789 --topic-id 42
```

If `--message-limit` truncates unread messages for a chat/topic, mark-as-read is skipped for that item and a warning is stored in run metadata.

## Offline Commands

```bash
# List cached chats and forum topics.
./bin/tg-arc chats --db data/tg-arc.db

# Export all cached messages for a chat.
./bin/tg-arc export --id -1001234567890 --db data/tg-arc.db

# Export one cached forum topic.
./bin/tg-arc export --id -1001234567890 --topic-id 42

# Export a local-date range.
./bin/tg-arc export --id -1001234567890 --since 2026-04-24 --until 2026-05-03

# Export everything up to a local date.
./bin/tg-arc export --id -1001234567890 --until 2026-05-03

# Select an account when one DB contains multiple accounts.
./bin/tg-arc export --account-id 2 --id 123456789
```

If a database contains one account, offline commands select it automatically. With multiple accounts, pass `--account-id`.

## Flags

| Flag | Commands | Description |
| --- | --- | --- |
| `--db <path>` | all | SQLite message cache path. Default: `data/tg-arc.db`. |
| `--session <path>` | `history`, `sync` | Telegram session SQLite path. Overrides `TG_SESSION_PATH`. |
| `--account-id <int>` | `chats`, `export` | Required only when one DB contains multiple accounts and you do not want the automatic single-account selection. |
| `--id <int64>` | `history`, `sync`, `export` | Chat ID. Accepts raw MTProto IDs and Bot API style `-100...` IDs. Required by `export`; makes `history` non-interactive; filters `sync` to one chat. |
| `--topic-id <int>` | `history`, `sync`, `export` | Forum topic ID. |
| `--topic <string>` | `history`, `sync` | Forum topic title. Exact case-insensitive matches win; otherwise a partial match must be unique. |
| `--since YYYY-MM-DD` | `history`, `export` | Start date. `history` interprets dates as UTC; `export` uses local dates. |
| `--until YYYY-MM-DD` | `history`, `export` | End date. `history` requires `--since`; `export` accepts `--until` alone. |
| `--chat-limit <n>` | `sync` | Maximum unread chats/topics to sync; `0` means unlimited. |
| `--message-limit <n>` | `history`, `sync` | Maximum messages per chat/topic; `0` means unlimited. If unread fetches are truncated, mark-as-read is skipped for that item. |

`sync` rejects `--since` and `--until`. `chats` only accepts `--db` and `--account-id`. `--format` is rejected; persistence is SQLite-only and transcript output is produced by `export`.
