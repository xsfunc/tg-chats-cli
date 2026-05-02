# CLI Telegram Chat Summary

CLI tool that authenticates with Telegram, lets you pick a chat (or forum topic) via TUI, and saves Telegram history into SQLite.
It supports one-shot history capture and unread synchronization with limits. This repository currently does not call an LLM; it prepares a local SQLite cache that can be summarized elsewhere.

## AI Agent Guide

Use this section to orient quickly and save context window.

First read: `README.md` (this file) and `AGENTS.md` for repo-specific rules.

High-level flow:
- `cmd/tg-summary` parses flags and launches the app.
- `internal/app` orchestrates login, TUI flow, DB history, unread sync, and mark-as-read.
- `internal/telegram` wraps the Telegram client and data fetch.
- `internal/store` owns the storage interface plus SQLite schema, migrations, and upserts.
- `internal/tui` contains Bubble Tea models for chat and topic selection.
- `internal/config` loads config from process environment variables.
- Cached messages go to `data/tg-summary.db` by default and sessions to `session/session.db`.
- Saved rows are scoped by the logged-in Telegram account so multiple accounts can share one message database.

Key behaviors to preserve:
- Unread history mode saves unread messages to SQLite and marks them as read after a successful DB write.
- Sync mode saves only unread messages from selected chats/topics.
- Date range mode saves a specific range and does not mark as read.
- Storage rows are keyed by `account_id`; each Telegram session should use its own session file.
- `--id` skips TUI and works with `--since` and `--until`.
- Forum chats require `--topic-id` or `--topic` in non-interactive mode.

Where to look for common tasks:
- CLI flags or new options: `cmd/tg-summary/`.
- SQLite persistence or schema changes: `internal/store/`.
- Telegram API changes or fetch logic: `internal/telegram/`.
- TUI changes: `internal/tui/`.
- Config or env updates: `internal/config/`.

Testing and lint:
- Run `mise run lint` before commits.
- Run `mise run test` for unit tests.
- Go formatting is required via `gofmt`.
- In restricted Codex sandboxes, run `MISE_CACHE_DIR=/tmp/mise-cache mise run lint` if `mise` itself warns about read-only `~/.cache/mise`.

Common pitfalls:
- Do not change public APIs without explicit request.
- Avoid new dependencies unless strictly needed.
- Be explicit about context cancellation and goroutine lifetimes.

Pre-commit checklist:
1. `gofmt` on changed Go files.
2. `mise run lint`.
3. `mise run test` (or the relevant subset).
4. Update `README.md` if behavior or structure changed.
5. Check `git status -sb`.

Definition of Done (for tasks):
1. At least one relevant test or verification step was run.
2. Documentation reflects any behavior/structure changes.
3. Diffs are understood and `git status -sb` is clean.

## Prerequisites

- [mise](https://mise.jdx.dev/)
- Go 1.26.2+
- Telegram API credentials from [my.telegram.org](https://my.telegram.org)

## Quick Start

```bash
cp mise.local.toml.example mise.local.toml
$EDITOR mise.local.toml
mise run run
```

## Setup

1. Install dependencies:
   ```bash
   mise install
   go mod download
   ```

2. Set environment variables:
   ```bash
   export TG_APP_ID=your_app_id
   export TG_APP_HASH=your_app_hash
   export TG_PHONE=+1234567890  # optional
   ```

   Or use a local mise config file:
   ```bash
   cp mise.local.toml.example mise.local.toml
   $EDITOR mise.local.toml
   ```

## Usage

```bash
mise run build
mise run exec
```

Or run directly:
```bash
mise run run
```

The default command is `history`. It opens the TUI, lets you choose a chat or forum topic, and saves messages to `data/tg-summary.db`.

```bash
# Same as ./bin/tg-summary
./bin/tg-summary history

# Save unread messages from one chat without the TUI
./bin/tg-summary history --id 123456789

# Save unread messages from all unread chats
./bin/tg-summary sync

# Save unread messages from up to 5 chats and 50 messages per chat/topic
./bin/tg-summary sync --chat-limit 5 --message-limit 50

# Use a custom SQLite database path
./bin/tg-summary sync --db /tmp/tg-summary.db

# Use a separate Telegram session while sharing the same message database
./bin/tg-summary sync --session session/account-a.db --db data/tg-summary.db
```

## Install From GitHub Releases With mise

Release tags named `v*` publish Linux `x64` and `arm64` archives that the mise GitHub backend can autodetect.

Create a release by pushing a version tag:
```bash
git tag v0.1.0
git push origin v0.1.0
```

Then install the latest release on a VPS:
```bash
mise use -g github:username/cli-tg-chat-summary
tg-summary
```

Replace `username` with the GitHub account or organization that owns the repository.

Note: Interactive history mode runs entirely inside the TUI (single alt-screen session) and does not print status messages to stdout.
It must be started from an interactive terminal. If stdin is piped or the command is run by a non-TTY wrapper, the app exits with a clear error; use `history --id` or `sync` for non-interactive runs.

TUI quick keys:
- `ctrl+c` to exit at any time.
- `q`/`esc` to exit from the chat list (in the topic list, `q`/`esc` goes back).
- `m` to switch history mode, `ctrl+r` to mark a chat as read (forum chats mark all topics).


## Date Range History

You can save messages from a specific date range instead of just unread messages.
When using date range mode, messages will NOT be marked as read.

In the TUI, press `m` to choose the history mode. Select `Date range` to enter `since` and `until` interactively.

```bash
# Save from a specific date until now
./bin/tg-summary --since 2024-01-01

# Save messages within a specific range
./bin/tg-summary --since 2024-01-01 --until 2024-01-31
```

## Non-Interactive History By Chat ID

Use `--id` to skip the TUI and save a specific chat in one shot. This works with date ranges too.

```bash
# Save unread messages from a chat
./bin/tg-summary --id 123456789

# Save with date range
./bin/tg-summary --id 123456789 --since 2024-01-01 --until 2024-01-31
```

### Forum Topics

For forum chats, you must provide a topic via `--topic-id` or `--topic`:

```bash
./bin/tg-summary --id 123456789 --topic-id 42
./bin/tg-summary --id 123456789 --topic "Release Notes"
```

For `sync --id <forum>`, topic flags are optional. Without a topic filter, sync saves all unread topics in that forum.

### `-100...` IDs

`--id` accepts both raw MTProto `ChannelID` values and Bot API style `-100...` IDs (which are normalized automatically).
To find chat IDs, use a Bot API-based tool or client that exposes chat IDs; channels/supergroups are often shown with the `-100...` prefix.

## How It Works

1. Authenticate with Telegram using `gotgproto`.
2. Fetch dialogs and show them in a TUI list (Bubble Tea, alternate screen).
3. If the selected chat is a forum, show a second TUI to select a topic.
4. Save chats, users, topics, messages, and run metadata into SQLite.
5. In unread history and sync modes, mark messages as read up to the max saved ID after the DB write succeeds.
6. If `--message-limit` truncates unread messages for a chat/topic, skip mark-as-read for that item and record a warning.
7. Show a save summary screen and return to the chat list on Enter.

## Configuration

The CLI is configured via process environment variables. It does not load `.env` files directly.
Use your shell, mise, direnv, Docker, CI secrets, systemd, or another environment manager to provide the variables.

Required:
- `TG_APP_ID` integer app ID from Telegram.
- `TG_APP_HASH` app hash from Telegram.

Optional:
- `TG_PHONE` phone number for login.
- `TG_SESSION_PATH` Telegram session SQLite path (default `session/session.db`).
- `LOG_LEVEL` `debug|info|warn|error` (default `info`).
- `RATE_LIMIT_MS` positive request interval in milliseconds (default `350`; non-positive values use the default).
- `TG_CONNECT_TIMEOUT_SECONDS` maximum time to wait for Telegram client startup before aborting (default `60`, set `0` to disable).
- `HISTORY_DELAY_MIN_MS` minimum pause between Telegram history pages (default `2000`).
- `HISTORY_DELAY_MAX_MS` maximum pause between Telegram history pages (default `4000`).
- `FLOOD_WAIT_MAX_SECONDS` maximum Telegram flood-wait delay to handle automatically (default `900`).

Example with shell:
```bash
export TG_APP_ID=your_app_id
export TG_APP_HASH=your_api_hash
export TG_PHONE=+1234567890
tg-summary
```

Example with local mise config:
```toml
# mise.local.toml
[env]
TG_APP_ID = "your_app_id"
TG_APP_HASH = "your_api_hash"
TG_PHONE = "+1234567890"
```

`mise.local.toml` is ignored by git and is suitable for local secrets. The committed `mise.toml` is for shared tools, non-secret environment settings, and tasks.

The Telegram session file is stored at `session/session.db` by default and can be changed with `TG_SESSION_PATH` or `--session`.
The message cache defaults to `data/tg-summary.db` and can be changed with `--db`.
One message database can contain multiple Telegram accounts because chats, topics, messages, users, and run metadata are scoped by `account_id`.
Use a separate session path per Telegram account; reusing one session path means reusing that Telegram login.

## Troubleshooting

If startup stops after the GoTGProto banner, the client is still trying to connect or authorize with Telegram. By default the app aborts after `TG_CONNECT_TIMEOUT_SECONDS=60` with a diagnostic error. Check network or proxy access to Telegram, try `LOG_LEVEL=debug`, or increase/disable the startup timeout with `TG_CONNECT_TIMEOUT_SECONDS=0` if you expect a long authorization step.

## Telegram Safety Pauses

History and sync use an additional pacer for `messages.getHistory` and forum topic history calls. The first history page is requested immediately, then later pages wait for a random delay between `HISTORY_DELAY_MIN_MS` and `HISTORY_DELAY_MAX_MS`. This jitter avoids fixed metronome-like request timing.

If Telegram returns a `FLOOD_WAIT`, the client logs the wait, slows future history requests with adaptive backoff, and retries only when the requested wait is no longer than `FLOOD_WAIT_MAX_SECONDS`. Longer waits stop the run with an error instead of leaving the process paused for a long time.

These settings reduce risk but do not guarantee that Telegram will not limit or ban an account. Avoid large full-history saves, repeated runs, multiple concurrent sessions, and unnecessary mark-as-read actions.

## CLI Flags

- Commands: `history` and `sync`; no command means `history`.
- `--db <path>` SQLite database path (default `data/tg-summary.db`).
- `--session <path>` Telegram session SQLite path (default `session/session.db` or `TG_SESSION_PATH`).
- `--since YYYY-MM-DD` start date for history mode (enables date range mode).
- `--until YYYY-MM-DD` end date for history mode (requires `--since`; defaults to now when omitted).
- `--id <int64>` chat ID (raw or `-100...`) to run without TUI.
- `--topic-id <int>` forum topic ID for non-interactive mode.
- `--topic <string>` forum topic title for non-interactive mode.
- `--chat-limit <n>` maximum unread chats to sync; `0` means unlimited (`sync` only).
- `--message-limit <n>` maximum messages per chat/topic; `0` means unlimited.

`--format` is no longer supported because normal CLI output is SQLite-only.

## SQLite Schema

The database is migrated automatically with `PRAGMA user_version`.

- `users`: Telegram users seen in dialog/history responses.
- `accounts`: logged-in Telegram accounts; existing v1 SQLite rows are migrated to a legacy account and adopted by the next logged-in account.
- `users`: Telegram users seen in dialog/history responses, keyed by `(account_id, id)`.
- `chats`: dialogs and chat metadata, including unread counters and read cursors, keyed by `(account_id, id)`.
- `topics`: forum topics keyed by `(account_id, chat_id, topic_id)`.
- `messages`: text messages keyed by `(account_id, chat_id, topic_id, message_id)`.
- `sync_runs` and `sync_run_items`: history/sync run status, saved counts, mark-read status, warnings, and errors.

## Project Structure

```
cmd/tg-summary/     - CLI entry point and flag parsing
internal/app/       - Orchestrates login, TUI flow, history, sync, mark-as-read
internal/config/    - Env config loader
internal/store/     - Storage interface plus SQLite schema, migrations, and persistence
internal/telegram/  - Telegram client wrapper (gotd + gotgproto)
internal/tui/       - Bubble Tea TUI models for chat/topic selection
```

## Development

| Command | Description |
|---------|-------------|
| `mise run` | Show help with all available commands |
| `mise run all` | Full cycle: tidy, lint, test, build |
| `mise run ci` | CI pipeline: clean build from scratch |
| `mise run build` | Compile binary to `bin/` directory |
| `mise run install` | Install to `$GOPATH/bin` |
| `mise run run` | Run via `go run` (example: `mise run run -- history --since 2024-01-01`) |
| `mise run exec` | Build and run binary |
| `mise run clean` | Clean build artifacts |
| `mise run tidy` | Update dependencies (`go mod tidy`) |
| `mise run lint` | Run golangci-lint |
| `mise run test` | Run unit tests |
| `mise run test-nocache` | Run tests without cache |
| `mise run test-cover` | Run tests with coverage report |
| `mise run setup-hooks` | Install git hooks |
