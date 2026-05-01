# CLI Telegram Chat Summary

CLI tool that authenticates with Telegram, lets you pick a chat (or forum topic) via TUI, exports unread messages or a date range to a text file, and optionally marks them as read.
This repository currently does not call an LLM; it only exports messages so they can be summarized elsewhere.

## AI Agent Guide

Use this section to orient quickly and save context window.

First read: `README.md` (this file) and `AGENTS.md` for repo-specific rules.

High-level flow:
- `cmd/tg-summary` parses flags and launches the app.
- `internal/app` orchestrates login, TUI flow, export, and mark-as-read.
- `internal/telegram` wraps the Telegram client and data fetch.
- `internal/tui` contains Bubble Tea models for chat and topic selection.
- `internal/config` loads config from env and `.env`.
- Exported files go to `exports/` and sessions to `session/session.db`.

Key behaviors to preserve:
- Unread mode exports unread messages and marks them as read.
- Date range mode exports a specific range and does not mark as read.
- `--id` skips TUI and works with `--since` and `--until`.
- Forum chats require `--topic-id` or `--topic` in non-interactive mode.

Where to look for common tasks:
- CLI flags or new options: `cmd/tg-summary/`.
- Export format or file naming: `internal/app/` and related helpers.
- Telegram API changes or fetch logic: `internal/telegram/`.
- TUI changes: `internal/tui/`.
- Config or env updates: `internal/config/`.

Testing and lint:
- Run `mise run lint` before commits.
- Run `mise run test` for unit tests.
- Go formatting is required via `gofmt`.

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
cp .env.example .env
sed -i 's/TG_APP_ID=.*/TG_APP_ID=123456/' .env
sed -i 's/TG_APP_HASH=.*/TG_APP_HASH=your_api_hash/' .env
sed -i 's/TG_PHONE=.*/TG_PHONE=+1234567890/' .env
mise run build
./bin/tg-summary
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

   Tip: You can copy `.env.example` to `.env` and fill in your details:
   ```bash
   cp .env.example .env
   ```

## Usage

```bash
mise run build
./bin/tg-summary
```

Or run directly:
```bash
mise run run
```

Note: Interactive mode runs entirely inside the TUI (single alt-screen session) and does not print status messages to stdout.
It must be started from an interactive terminal. If stdin is piped or the command is run by a non-TTY wrapper, the app exits with a clear error; use `--id` for non-interactive exports.

TUI quick keys:
- `ctrl+c` to exit at any time.
- `q`/`esc` to exit from the chat list (in the topic list, `q`/`esc` goes back).
- `m` to switch export mode, `ctrl+r` to mark a chat as read (forum chats mark all topics).


## Date Range Export

You can export messages from a specific date range instead of just unread messages.
When using date range mode, messages will NOT be marked as read.

In the TUI, press `m` to choose the export mode. Select `Date range` to enter `since` and `until` interactively.

```bash
# Export from a specific date until now
./bin/tg-summary --since 2024-01-01

# Export messages within a specific range
./bin/tg-summary --since 2024-01-01 --until 2024-01-31
```

## Non-Interactive Export By Chat ID

Use `--id` to skip the TUI and export a specific chat in one shot. This works with date ranges too.

```bash
# Export unread messages from a chat
./bin/tg-summary --id 123456789

# Export with date range
./bin/tg-summary --id 123456789 --since 2024-01-01 --until 2024-01-31
```

### Forum Topics

For forum chats, you must provide a topic via `--topic-id` or `--topic`:

```bash
./bin/tg-summary --id 123456789 --topic-id 42
./bin/tg-summary --id 123456789 --topic "Release Notes"
```

### `-100...` IDs

`--id` accepts both raw MTProto `ChannelID` values and Bot API style `-100...` IDs (which are normalized automatically).
To find chat IDs, use a Bot API-based tool or client that exposes chat IDs; channels/supergroups are often shown with the `-100...` prefix.

## How It Works

1. Authenticate with Telegram using `gotgproto`.
2. Fetch dialogs and show them in a TUI list (Bubble Tea, alternate screen).
3. If the selected chat is a forum, show a second TUI to select a topic.
4. Export messages to `exports/<Chat_or_Topic>_<YYYY-MM-DD>.txt` (or `exports/<Chat_or_Topic>_<YYYY-MM-DD>_to_<YYYY-MM-DD>.txt` for date ranges).
5. In unread mode, mark messages as read up to the max exported ID.
6. Show an export summary screen and return to the chat list on Enter.

## Configuration

Required:
- `TG_APP_ID` integer app ID from Telegram.
- `TG_APP_HASH` app hash from Telegram.

Optional:
- `TG_PHONE` phone number for login.
- `LOG_LEVEL` `debug|info|warn|error` (default `info`).
- `RATE_LIMIT_MS` positive request interval in milliseconds (default `350`; non-positive values use the default).
- `TG_CONNECT_TIMEOUT_SECONDS` maximum time to wait for Telegram client startup before aborting (default `60`, set `0` to disable).
- `HISTORY_DELAY_MIN_MS` minimum pause between Telegram history pages (default `2000`).
- `HISTORY_DELAY_MAX_MS` maximum pause between Telegram history pages (default `4000`).
- `FLOOD_WAIT_MAX_SECONDS` maximum Telegram flood-wait delay to handle automatically (default `900`).

The session file is stored at `session/session.db`.

## Troubleshooting

If startup stops after the GoTGProto banner, the client is still trying to connect or authorize with Telegram. By default the app aborts after `TG_CONNECT_TIMEOUT_SECONDS=60` with a diagnostic error. Check network or proxy access to Telegram, try `LOG_LEVEL=debug`, or increase/disable the startup timeout with `TG_CONNECT_TIMEOUT_SECONDS=0` if you expect a long authorization step.

## Telegram Safety Pauses

History export uses an additional pacer for `messages.getHistory` and forum topic history calls. The first history page is requested immediately, then later pages wait for a random delay between `HISTORY_DELAY_MIN_MS` and `HISTORY_DELAY_MAX_MS`. This jitter avoids fixed metronome-like request timing.

If Telegram returns a `FLOOD_WAIT`, the client logs the wait, slows future history requests with adaptive backoff, and retries only when the requested wait is no longer than `FLOOD_WAIT_MAX_SECONDS`. Longer waits stop the export with an error instead of leaving the process paused for a long time.

These settings reduce risk but do not guarantee that Telegram will not limit or ban an account. Avoid large full-history exports, repeated runs, multiple concurrent sessions, and unnecessary mark-as-read actions.

## CLI Flags

- `--since YYYY-MM-DD` start date for export (enables date range mode).
- `--until YYYY-MM-DD` end date for export (requires `--since`; defaults to now when omitted).
- `--format <text|xml|xml-compact>` export format (default `text`).
- `--id <int64>` chat ID (raw or `-100...`) to export without TUI.
- `--topic-id <int>` forum topic ID for non-interactive mode.
- `--topic <string>` forum topic title for non-interactive mode.

## Output Format

Exports are written to `exports/` with a header and collapsed blocks per sender ID:
- `Chat Summary: <title>`
- `Export Date: <RFC1123>`
- `Total Messages: <count>`
- `[HH:MM] id=<sender_id>:` followed by indented message lines

Example file:
```text
Chat Summary: Project Team
Export Date: Mon, 27 Jan 2025 10:35:12 UTC
Total Messages: 3

[09:12] id=123:
  Morning! Status update?
[09:18-09:22] id=456:
  API is green, frontend build is running.
  Build is green, pushing summary in 30 min.
```

### XML Format

Use `--format xml` to export messages as XML:

```xml
<chat title="Project Team">
  <export_date>2025-01-27T10:35:12Z</export_date>
  <total_messages>3</total_messages>
  <message>
    <sender id="123"></sender>
    <time>2025-01-27T09:12:00Z</time>
    <text>Morning! Status update?</text>
  </message>
</chat>
```

### XML Compact Format

Use `--format xml-compact` to export a compact XML variant with short tags/attributes:

```xml
<c t="Project Team" d="2025-01-27T10:35:12Z" n="3">
  <m t="2025-01-27T09:12:00Z" s="123">Morning! Status update?</m>
</c>
```

Compact field mapping:
- `c` root tag: `t` title, `d` export date, `n` total messages, `s` since, `u` until.
- `m` message tag: `t` time, `s` sender id, `n` sender name.
- `r` reply tag (optional): `i` message id, `s` sender id, `n` sender name.
- `rx` reactions container (optional) with `x` entries: `e` emoji, `c` count.

## Project Structure

```
cmd/tg-summary/     - CLI entry point and flag parsing
internal/app/       - Orchestrates login, TUI flow, export, mark-as-read
internal/config/    - Env config loader (.env supported)
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
| `mise run run` | Run via `go run` (example: `mise run run -- --since 2024-01-01`) |
| `mise run exec` | Build and run binary |
| `mise run clean` | Clean build artifacts |
| `mise run tidy` | Update dependencies (`go mod tidy`) |
| `mise run lint` | Run golangci-lint |
| `mise run test` | Run unit tests |
| `mise run test-nocache` | Run tests without cache |
| `mise run test-cover` | Run tests with coverage report |
| `mise run setup-hooks` | Install git hooks |
