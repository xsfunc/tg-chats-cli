# tg-arc

`tg-arc` is a local Telegram archive CLI. It logs in with your Telegram user account, saves incoming text messages from selected chats or forum topics to SQLite, syncs unread messages, marks saved unread ranges as read after successful database writes, and exports cached messages as Markdown.

It does not call LLMs or send your archive to a remote service.

## Quick Start

Prerequisites: [mise](https://mise.jdx.dev/) and Telegram API credentials from [my.telegram.org](https://my.telegram.org).

```bash
cp mise.local.toml.example mise.local.toml
$EDITOR mise.local.toml
mise install
mise dev -- history
```

The first online run prompts for Telegram login code or 2FA when needed. By default, the Telegram session is stored in `session/session.db` and messages are stored in `data/tg-arc.db`.

Common commands:

```bash
mise dev -- history                       # interactive chat/topic picker
mise dev -- sync --chat-limit 5           # save unread messages
mise dev -- chats                         # list cached chats, offline
mise dev -- export --id 123456789 > chat.md
```

Build a binary with:

```bash
mise run build
./bin/tg-arc history
```

## Documentation

- [Setup](docs/setup.md): prerequisites, Telegram credentials, environment variables, local secrets, proxy notes.
- [CLI Usage](docs/usage.md): commands, flags, interactive and non-interactive examples.
- [Data And Export](docs/data.md): SQLite scope, schema summary, offline chat listing, Markdown export.
- [Architecture](docs/architecture.md): package map, application flow, important invariants.
- [Development](docs/development.md): mise tasks, tests, lint, hooks, release workflow.

For agent-specific repository rules, see [AGENTS.md](AGENTS.md).
