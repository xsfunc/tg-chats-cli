# Setup

## Prerequisites

- [mise](https://mise.jdx.dev/)
- Telegram API credentials from [my.telegram.org](https://my.telegram.org)

Install pinned tools:

```bash
mise install
```

## Configuration

`tg-arc` reads process environment variables only. It does not load `.env` files. Use a shell, `mise.local.toml`, direnv, CI secrets, systemd, or another environment manager.

Required:

| Variable | Description |
| --- | --- |
| `TG_APP_ID` | Integer Telegram app ID. |
| `TG_APP_HASH` | Telegram app hash. |

Optional:

| Variable | Default | Description |
| --- | --- | --- |
| `TG_PHONE` | empty | Phone number for login. If empty, Telegram prompts for it. |
| `TG_SESSION_PATH` | `session/session.db` | Telegram session SQLite path. Use a separate file per Telegram account. |
| `TG_PROXY_URL` | empty | SOCKS proxy for Telegram MTProto, for example `socks5://172.28.224.1:1080`. HTTP proxies are not supported. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`; unknown values behave like `info`. |
| `RATE_LIMIT_MS` | `350` | Global Telegram API request interval in milliseconds. Invalid or non-positive values reset to `350`; positive values below `50` are clamped to `50`. |
| `TG_CONNECT_TIMEOUT_SECONDS` | `60` | Startup connection timeout. Set `0` to disable. Invalid or negative values reset to `60`. Login code and 2FA input time is not counted. |
| `HISTORY_DELAY_MIN_MS` | `2000` | Minimum pacer delay before dialog, forum topic, history, and mark-as-read requests. Invalid or non-positive values reset to `2000`; positive values below `500` are clamped to `500`. |
| `HISTORY_DELAY_MAX_MS` | `4000` | Maximum pacer delay for the same requests. Invalid or non-positive values reset to `4000`; positive values below `500` are clamped to `500`; reversed min/max values are swapped. |
| `FLOOD_WAIT_MAX_SECONDS` | `900` | Maximum Telegram `FLOOD_WAIT` retried automatically. Invalid or non-positive values reset to `900`. Longer waits fail the run. |

Shell example:

```bash
export TG_APP_ID=123456
export TG_APP_HASH=your_api_hash
export TG_PHONE=+1234567890
mise dev -- history
```

Local mise example:

```bash
cp mise.local.toml.example mise.local.toml
$EDITOR mise.local.toml
mise dev -- history
```

`mise.local.toml` is ignored by git and is suitable for local secrets. Keep shared tool versions and non-secret tasks in `mise.toml`.

## Sessions And Databases

The Telegram session defaults to `session/session.db`; override it with `TG_SESSION_PATH` or `--session`. The message cache defaults to `data/tg-arc.db`; override it with `--db`.

One message database can hold multiple Telegram accounts because persisted rows are scoped by `account_id`. Use a separate session file per Telegram account; reusing a session path reuses that Telegram login.

## Proxy And Startup Troubleshooting

If startup stops after the GoTGProto banner, the client is still connecting to Telegram. The app aborts startup network phases after `TG_CONNECT_TIMEOUT_SECONDS` and returns a diagnostic error. Human time spent entering phone, code, or 2FA input is not counted.

For WSL with Xray running on Windows, set `TG_PROXY_URL` to the Windows host IP and Xray SOCKS port. The host IP is usually the `nameserver` from `/etc/resolv.conf`:

```bash
export TG_PROXY_URL=socks5://172.28.224.1:1080
```
