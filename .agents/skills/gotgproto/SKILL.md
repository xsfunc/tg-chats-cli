---
name: gotgproto Telegram Client
description: "Use for Go userbot/client code built on github.com/celestix/gotgproto: simplified gotd/td auth, sessions, dispatcher handlers, filters, ext.Context helpers, peer storage, raw API access, history export, and Telegram media operations."
metadata:
  tags: go, telegram, mtproto, gotgproto, userbot, dispatcher, sessions, peer-storage
---

# gotgproto Telegram Client

Use this skill when a task mentions `gotgproto`, `github.com/celestix/gotgproto`, Telegram userbots, handler groups, `ext.Context`, `ext.Update`, `ctx.Raw`, `ctx.PeerStorage`, string sessions, or simplified Telegram client setup in Go.

## Non-Negotiables

1. Inherit Telegram client safety from `gotd/td`: protect `APP_ID`/`APP_HASH`, avoid QR login, avoid VoIP-number assumptions, and pace repeated API calls.
2. Use persistent session storage for real clients. Use in-memory sessions only for tests or throwaway tools.
3. Check `update.EffectiveMessage` for nil before reading fields.
4. Skip outgoing messages in reply/echo/new-message handlers unless the feature explicitly needs them.
5. Return `dispatcher.EndGroups` when a handler intentionally consumes an update.
6. Handle every error from `ctx.SendMessage`, `ctx.Reply`, `ctx.DownloadMedia`, `ctx.Raw`, and helper methods.
7. Call `client.Idle()` or otherwise block the process when the app is expected to keep receiving updates.
8. Drop to raw `gotd/td` via `ctx.Raw` for dialogs, history, forum topics, mark-as-read, and any unsupported helper.

## Choose The Right API

| Need | Use |
| --- | --- |
| Quick userbot/client with auth, sessions, dispatcher, peer helpers | `gotgproto` |
| Raw MTProto control, custom update gap recovery, custom middleware | `gotd/td` directly |
| Dialog/history export with convenience context | `gotgproto` plus `ctx.Raw` |
| Existing Pyrogram/Telethon/GramJS string session migration | `sessionMaker.*Session` imports |

## Client Setup

```go
appID, err := strconv.Atoi(os.Getenv("TG_APP_ID"))
if err != nil {
    return fmt.Errorf("parse TG_APP_ID: %w", err)
}

client, err := gotgproto.NewClient(
    appID,
    os.Getenv("TG_APP_HASH"),
    gotgproto.ClientTypePhone(os.Getenv("TG_PHONE")),
    &gotgproto.ClientOpts{
        Session:         sessionMaker.SqlSession(sqlite.Open("session.db")),
        AuthConversator: gotgproto.BasicConversator(),
    },
)
if err != nil {
    return fmt.Errorf("start telegram client: %w", err)
}

client.Idle()
```

Use `sessionMaker.JsonFileSession` for simple local storage, `sessionMaker.SqlSession` for durable SQLite storage, and `sessionMaker.SimpleSession`/`InMemory` only for tests.

## Dispatcher Pattern

```go
client.Dispatcher.AddHandlerToGroup(
    handlers.NewMessage(filters.Message.Text, func(ctx *ext.Context, u *ext.Update) error {
        msg := u.EffectiveMessage
        if msg == nil || msg.Out {
            return nil
        }

        if _, err := ctx.Reply(u, ext.ReplyTextString("received"), nil); err != nil {
            return fmt.Errorf("reply to telegram message: %w", err)
        }
        return dispatcher.EndGroups
    }),
    0,
)
```

Use filters to narrow handler activation (`filters.Message.Text`, `Media`, `Photo`, `Group`, `Supergroup`, `Channel`) instead of filtering everything inside the handler.

## Context Helpers

Use `ext.Context` helpers for common operations:

```go
if _, err := ctx.SendMessage(chatID, &tg.MessagesSendMessageRequest{
    Message: "hello",
}); err != nil {
    return fmt.Errorf("send telegram message: %w", err)
}

if _, err := ctx.SendMedia(chatID, request); err != nil {
    return fmt.Errorf("send telegram media: %w", err)
}

chat, err := ctx.GetChat(chatID)
if err != nil {
    return fmt.Errorf("get telegram chat: %w", err)
}
_ = chat
```

For media downloads, prefer `ctx.DownloadMedia` with `ext.DownloadOutputPath` for files and `ext.DownloadOutputBuffer` only when the expected payload is small.

## Raw API Pattern

Use `ctx.Raw` when helper methods do not expose the needed MTProto request.

```go
dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
    OffsetPeer: &tg.InputPeerEmpty{},
    Limit:      100,
})
if err != nil {
    return fmt.Errorf("get telegram dialogs: %w", err)
}

switch d := dialogs.(type) {
case *tg.MessagesDialogs:
    return consumeDialogs(ctx, d.Dialogs)
case *tg.MessagesDialogsSlice:
    return consumeDialogs(ctx, d.Dialogs)
default:
    return fmt.Errorf("unexpected dialogs response %T", dialogs)
}
```

For history:

```go
history, err := ctx.Raw.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
    Peer:  ctx.PeerStorage.GetInputPeerById(chatID),
    Limit: 100,
})
if err != nil {
    return fmt.Errorf("get telegram history: %w", err)
}
```

Handle `*tg.MessagesMessages`, `*tg.MessagesMessagesSlice`, and `*tg.MessagesChannelMessages` before processing messages. Guard each message with `m, ok := msg.(*tg.Message)`.

## Peer Storage

gotgproto maintains peer storage automatically. Prefer:

```go
peer := ctx.PeerStorage.GetInputPeerById(chatID)
user := ctx.PeerStorage.GetInputUserById(userID)
```

Refresh a context created outside handlers when entities may have changed:

```go
ctx := client.CreateContext()
client.RefreshContext(ctx)
```

## Mark-As-Read And Export Tools

- Resolve peers through `ctx.PeerStorage`.
- Export before marking read.
- Track the maximum exported message ID.
- Use `messages.readHistory` for normal peers.
- Use `channels.readHistory` for channels/supergroups when channel identity and access hash are available.
- Add pacing/jitter for paged history scans; gotgproto convenience does not remove Telegram rate-limit risk.

## Common Fixes

| Symptom | Fix |
| --- | --- |
| Handler panics on service updates | Guard `update.EffectiveMessage == nil` |
| Replies loop forever | Return early for outgoing messages |
| Multiple handlers process one command | Return `dispatcher.EndGroups` from the consuming handler |
| Program exits after login | Call `client.Idle()` or block on a cancellable context |
| Need dialogs/history/forum topics | Use `ctx.Raw` and handle all response variants |
| Peer lookup fails | Create/refresh context after dialogs are loaded, then use `ctx.PeerStorage` |

## Integrated Example: Unread Scanner

```go
client, err := gotgproto.NewClient(appID, appHash, gotgproto.ClientTypePhone(phone), opts)
if err != nil {
    return fmt.Errorf("start telegram client: %w", err)
}

ctx := client.CreateContext()
dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
    OffsetPeer: &tg.InputPeerEmpty{},
    Limit:      100,
})
if err != nil {
    return fmt.Errorf("get dialogs: %w", err)
}

if err := exportUnreadDialogs(ctx, dialogs); err != nil {
    return fmt.Errorf("export unread dialogs: %w", err)
}
```

Before shipping gotgproto changes, run a focused Go test or lint step and review session persistence, handler nil checks, outgoing-message handling, pacing, and raw API response switches.

## References

- Repository: https://github.com/celestix/gotgproto
- Documentation: https://pkg.go.dev/github.com/celestix/gotgproto
- Examples: https://github.com/celestix/gotgproto/tree/beta/examples
- Underlying library: https://github.com/gotd/td
- API credentials: https://my.telegram.org/apps
