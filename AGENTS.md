# AGENTS.md

Self-contained routing and repository rules for LLM agents working on `tg-arc`.

## Project Summary

`tg-arc` is a Go CLI that logs in to Telegram with a user account, saves selected chat or forum-topic history into SQLite, syncs unread messages, marks saved unread ranges as read, and exports cached messages as Markdown. It does not call an LLM.

Default paths:

- Message DB: `data/tg-arc.db`
- Telegram session DB: `session/session.db`

## Routing

- CLI flags, commands, argument validation: `cmd/tg-arc/`
- App orchestration, history/sync behavior, mark-as-read: `internal/app/`
- Telegram client, dialogs, topics, history fetch, flood wait handling: `internal/telegram/`
- SQLite schema, migrations, account scoping, upserts, run metadata: `internal/store/`
- Offline `chats` and `export` output: `internal/exporter/`
- Bubble Tea chat/topic TUI, progress, summary screens: `internal/tui/`
- Environment config: `internal/config/`
- User docs: `docs/`; root `README.md` is only the entry point.

## Architecture Invariants

- Rows are scoped by `account_id`; several Telegram accounts may share one message DB.
- Use a separate session file per Telegram account.
- Unread `history` and `sync` mark messages as read only after successful SQLite writes.
- Date range `history` never marks messages as read.
- `sync` saves unread messages only and does not accept date ranges.
- Forum `history --id` requires `--topic-id` or `--topic`; forum `sync --id` without topic flags syncs all unread topics.
- `--id` accepts raw MTProto IDs and Bot API `-100...` IDs.
- If `--message-limit` truncates unread messages, skip mark-as-read for that item and store a warning.
- Offline commands (`chats`, `export`) must not require Telegram config or login.

## Commands

Use `mise`, not `make`.

- `mise install`: install configured tools.
- `mise dev -- <args>`: run via `go run ./cmd/tg-arc`.
- `mise run build`: build `bin/tg-arc`.
- `mise run lint`: run `golangci-lint`.
- `mise run test`: run unit tests.
- `mise run all`: tidy, lint, test, build.
- `mise run setup-hooks`: enable local git hooks.

## Go Rules

- Run `gofmt` on changed Go files.
- Prefer the standard library; avoid new dependencies unless clearly required.
- Do not change public APIs unless explicitly requested.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Handle errors immediately; no `panic` for normal failures.
- Use guard clauses and minimal nesting.
- Use `any` instead of `interface{}`.
- Avoid global state and `init()` where possible.
- Be explicit about context cancellation, goroutine lifetimes, and channel closure.

## Tests And Docs

- If behavior changes, add or update a nearby test when practical.
- Prefer table-driven tests; avoid flaky waits and `time.Sleep` unless necessary.
- Run at least one relevant verification step; run `mise run lint` after code changes.
- Update docs under `docs/` when behavior, flags, setup, or structure changes.
- Keep root `README.md` short and avoid duplicating detailed docs there.

## Git Rules

- Commit directly to `main` when asked to commit.
- Use Conventional Commits: `type(scope): summary`.
- Before confirming changes, check `git status -sb`, `git diff`, and `git diff --staged`.
- Do not revert unrelated user changes. Current untracked or unrelated files should be called out, not removed.

## CI And Releases

GitHub Actions must use `actions/checkout@v6`, `actions/cache@v5`, and `jdx/mise-action@v4`. Release tags `v*` build Linux `x64` and `arm64` archives for GitHub Releases and mise's GitHub backend.
