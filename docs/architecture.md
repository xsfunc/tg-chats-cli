# Architecture

## Package Map

```text
cmd/tg-arc/       CLI entry point and flag parsing
internal/app/     Login orchestration, TUI flow, history, sync, mark-as-read
internal/config/  Environment configuration loader
internal/exporter Offline SQLite chat listing and Markdown transcript rendering
internal/store/   Store interface, SQLite schema/migrations, account scope, upserts, run metadata
internal/telegram Telegram client wrapper: login, dialogs, history, topics, mark-as-read, pacing
internal/tui/     Bubble Tea models for chat/topic selection, loading, progress, summary
```

## Runtime Flow

1. `cmd/tg-arc` parses command and flags into `app.RunOptions`.
2. `chats` and `export` open SQLite directly and call `internal/exporter`.
3. Online commands load env config, create `internal/telegram.Client`, log in, and identify the Telegram account.
4. `internal/app` opens SQLite, sets account scope, and routes to sync, non-interactive history, or interactive TUI.
5. Telegram dialogs, users, topics, messages, and run metadata are saved through `internal/store`.
6. Unread history and sync mark messages as read only after the DB write succeeds.

## Behavior Invariants

- `history` with `--since`/`--until` is date range mode and does not mark messages as read.
- `sync` saves unread messages only and rejects date range flags.
- Forum `history --id` requires `--topic-id` or `--topic`.
- Forum `sync --id` without topic flags syncs all unread topics.
- `--id` accepts raw MTProto IDs and Bot API style `-100...` IDs.
- `--message-limit` truncation prevents mark-as-read for that item.
- Offline commands do not load Telegram config, session files, or network clients.

## Telegram Pacing

Telegram dialog, forum topic, history, and mark-as-read calls covered by the history pacer wait before each request, including the first request. The base wait is a random delay between `HISTORY_DELAY_MIN_MS` and `HISTORY_DELAY_MAX_MS`.

If Telegram returns `FLOOD_WAIT`, the client logs the wait, increases the pacer backoff, and retries only when the requested wait is no longer than `FLOOD_WAIT_MAX_SECONDS`. Longer waits stop the run with an error.

These safeguards reduce request pressure but do not guarantee Telegram will not limit an account. Avoid large full-history saves, repeated concurrent runs, and unnecessary mark-as-read operations.
