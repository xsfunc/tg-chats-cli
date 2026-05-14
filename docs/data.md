# Data And Export

## Storage Model

SQLite is migrated automatically with `PRAGMA user_version`; the current schema version is `2`.

Main tables:

- `accounts`: logged-in Telegram accounts.
- `users`: cached Telegram users, keyed by `(account_id, id)`.
- `chats`: dialog metadata and unread cursors, keyed by `(account_id, id)`.
- `topics`: forum topics, keyed by `(account_id, chat_id, topic_id)`.
- `messages`: text messages, keyed by `(account_id, chat_id, topic_id, message_id)`.
- `sync_runs` and `sync_run_items`: run status, saved counts, mark-read status, warnings, and errors.

Existing v1 databases are migrated to v2 by creating a legacy account and moving previous rows under `account_id=1`. The next logged-in account can then adopt the legacy account row.

Current history and sync fetches save incoming text messages only. Cached rows are upserted, so rerunning history or sync refreshes existing messages without duplicating them.

## Account Scope

Every persisted chat, topic, user, message, and run belongs to an `account_id`. This allows several Telegram accounts to share one message database while keeping data separate. Keep Telegram session files separate too:

```bash
./bin/tg-arc sync --session session/account-a.db --db data/tg-arc.db
./bin/tg-arc sync --session session/account-b.db --db data/tg-arc.db
```

## Markdown Export

`export` reads from SQLite only; it never connects to Telegram or fetches missing messages. It prints a chronological Markdown transcript. Sender names come from cached `users` rows. Cached outgoing rows, if present, are labeled `Me`; missing users fall back to `sender_id=<id>`.

```md
# Telegram Chat Export

Account: Test (@test), account_id=1
Chat: Project, chat_id=123456789
Topic: Release, topic_id=42
Range: 2026-05-03 21:20 +03:00 .. 2026-05-03 21:21 +03:00
Messages: 2

## Transcript

[2026-05-03 21:20 +03:00] Alice (@alice):
Message text

[2026-05-03 21:21 +03:00] Me:
Reply text
```

Date filters in `export` use local dates and print timestamps with the local UTC offset.
