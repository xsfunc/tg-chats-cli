---
name: gotd/td Telegram Client
description: "Use for Go code that directly uses github.com/gotd/td: MTProto clients, tg.Client raw API calls, auth flows, sessions, update dispatchers, peer storage, history export, mark-as-read, rate limiting, and FLOOD_WAIT handling."
metadata:
  tags: go, telegram, mtproto, gotd, tg-client, updates, sessions, flood-wait
---

# gotd/td Telegram Client

Use this skill when a task mentions `gotd/td`, `github.com/gotd/td`, `tg.Client`, MTProto, Telegram dialogs/history, unread exports, forum topics, update gaps, session storage, `FLOOD_WAIT`, or raw Telegram API requests in Go.

## Non-Negotiables

1. Protect `APP_ID` and `APP_HASH`: read them from env/config, never print them, never commit them.
2. Do not implement QR login, spammy send loops, or VoIP-number assumptions for user clients.
3. Add `FLOOD_WAIT` handling and rate limiting for userbots, history export, crawlers, and any repeated API calls.
4. Use cancellable contexts. Own goroutine lifetimes explicitly and stop work when `ctx.Done()` closes.
5. Check every Telegram type assertion before use. Never assume `tg.MessageClass` is `*tg.Message`.
6. Skip outgoing messages in echo/reply/new-message handlers unless the feature explicitly needs them.
7. Custom session storage must return `session.ErrNotFound` when empty and must return a copy of stored bytes.
8. Wrap errors with context in application/library code; avoid `panic` and `log.Fatal` outside tiny examples.

## Choose The Right Layer

| Need | Use |
| --- | --- |
| Maximum MTProto control, raw requests, custom sessions, update gap recovery | `gotd/td` directly |
| Faster userbot setup, built-in dispatcher, peer/session helpers | `gotgproto` skill |
| Message history, forum topics, mark-as-read, custom pacing | `gotd/td` raw API or gotgproto `ctx.Raw` |

## Client Setup

Prefer explicit options so session storage, update handling, middleware, and logging are visible at the call site.

```go
apiID, apiHash := cfg.AppID, cfg.AppHash
dispatcher := tg.NewUpdateDispatcher()

waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
    log.Warn("telegram flood wait", zap.Duration("wait", wait.Duration))
})

client := telegram.NewClient(apiID, apiHash, telegram.Options{
    SessionStorage: sessionStorage,
    Logger:         log,
    UpdateHandler:  dispatcher,
    Middlewares: []telegram.Middleware{
        waiter,
        ratelimit.New(rate.Every(350*time.Millisecond), 1),
    },
})
```

`telegram.ClientFromEnvironment` expects the library's environment names (`APP_ID`, `APP_HASH`, `SESSION_FILE`/`SESSION_DIR`). If the project uses `TG_APP_ID` or `TG_APP_HASH`, load config yourself and call `telegram.NewClient`.

## Session Storage Contract

```go
type store struct {
    mu   sync.RWMutex
    data []byte
}

func (s *store) LoadSession(context.Context) ([]byte, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if len(s.data) == 0 {
        return nil, session.ErrNotFound
    }
    return append([]byte(nil), s.data...), nil
}

func (s *store) StoreSession(_ context.Context, data []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.data = append(s.data[:0], data...)
    return nil
}
```

Use `telegram.FileSessionStorage` for simple local persistence. Use a custom `telegram.SessionStorage` only when the app already has a database or encrypted store.

## Authentication Pattern

```go
flow := auth.NewFlow(termAuth{phone: cfg.Phone}, auth.SendCodeOptions{})
if err := client.Auth().IfNecessary(ctx, flow); err != nil {
    return fmt.Errorf("authenticate telegram client: %w", err)
}
```

An authenticator should implement phone, code, and 2FA password prompts. If sign-up is not supported, return an explicit error from `SignUp` instead of silently creating accounts.

## Update Handling

```go
dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
    m, ok := u.Message.(*tg.Message)
    if !ok || m.Out {
        return nil
    }

    if err := handleIncoming(ctx, e, m); err != nil {
        return fmt.Errorf("handle incoming message: %w", err)
    }
    return nil
})
```

Use `telegram/updates` with durable state storage when missed updates matter. Wrap the dispatcher with peer storage hooks when later code needs reliable entity resolution.

## Raw API Patterns

Always handle every documented response shape for paged methods.

```go
res, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
    Peer:  peer,
    Limit: 100,
})
if err != nil {
    return fmt.Errorf("get telegram history: %w", err)
}

switch h := res.(type) {
case *tg.MessagesMessages:
    return consumeMessages(ctx, h.Messages)
case *tg.MessagesMessagesSlice:
    return consumeMessages(ctx, h.Messages)
case *tg.MessagesChannelMessages:
    return consumeMessages(ctx, h.Messages)
default:
    return fmt.Errorf("unexpected history response %T", res)
}
```

Mark read with `messages.readHistory` for normal peers and `channels.readHistory` for channels/supergroups when the channel ID and access hash are available. For export tools, only mark read after a successful export and use the maximum exported message ID.

## Sending And Media

- Use `message.NewSender(tg.NewClient(client))` for text, replies, formatted HTML, and resolved usernames.
- Use `uploader.NewUploader(api)` for uploads; check every upload/send error.
- Use `downloader.NewDownloader().Download(api, location)` for downloads; stream large files instead of buffering by default.
- Keep send loops paced. Telegram safety rules still apply to helper APIs.

## Common Fixes

| Symptom | Fix |
| --- | --- |
| Panic on updates | Replace direct assertions with `m, ok := x.(*tg.Message)` guard clauses |
| Bot replies to itself forever | Return early when `m.Out` is true |
| Re-login every run | Add persistent `SessionStorage` |
| Session corruption | Copy bytes in `LoadSession` and `StoreSession` |
| Frequent `FLOOD_WAIT` errors | Add `floodwait.NewWaiter`, rate limit, jitter, and max-wait policy |
| App ignores Ctrl+C | Use `signal.NotifyContext` and pass the derived context to `client.Run` |

## Integrated Example: Passive Unread Export

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

if err := client.Run(ctx, func(ctx context.Context) error {
    flow := auth.NewFlow(authenticator, auth.SendCodeOptions{})
    if err := client.Auth().IfNecessary(ctx, flow); err != nil {
        return fmt.Errorf("authenticate: %w", err)
    }

    api := tg.NewClient(client)
    dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
        OffsetPeer: &tg.InputPeerEmpty{},
        Limit:      100,
    })
    if err != nil {
        return fmt.Errorf("get dialogs: %w", err)
    }

    if err := exportUnreadDialogs(ctx, api, dialogs); err != nil {
        return fmt.Errorf("export unread dialogs: %w", err)
    }
    return nil
}); err != nil {
    return fmt.Errorf("run telegram client: %w", err)
}
```

Before shipping Telegram client changes, verify at least one focused `go test`/lint step and review credential, pacing, session, and mark-as-read behavior.

## References

- Documentation: https://pkg.go.dev/github.com/gotd/td
- Examples: https://github.com/gotd/td/tree/main/examples
- API credentials: https://my.telegram.org/apps
