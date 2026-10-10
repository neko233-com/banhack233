# Notification channels and reliable delivery

Telegram, Discord, Slack, Feishu/Lark, generic webhooks and email are supported. Merge the following fragments into `notifications`. Values are placeholders; keep real credentials local. See [configuration](configuration-en.md).

## Telegram

Create a bot with Telegram's official `@BotFather`. Send `/start` in a private chat, or add the bot to a group with send permission; channels require permission to publish. Check `chat.id` in your own Bot API `getUpdates` response. Forum topics additionally need `message_thread_id`.

```json
{"telegram": {"enabled": true, "bot_token_env": "BANHACK233_TELEGRAM_BOT_TOKEN", "chat_id": "-1000000000000", "message_thread_id": 42, "disable_notification": false}}
```

Remove `message_thread_id` for ordinary chats. A local `bot_token` is also supported. If `bot_token_env` is configured it takes precedence, without falling back when the variable is missing. The variable must exist in the **daemon environment**, not just an interactive terminal.

For systemd, add this using `systemctl edit banhack233`:

```ini
[Service]
EnvironmentFile=/etc/banhack233/notifications.env
```

Create a root-only environment file containing your token assignment, then run `systemctl daemon-reload` and restart the daemon. Manual `notify-test` needs the same variable. Do not paste environment dumps into issues. Windows SYSTEM tasks do not automatically inherit a user's temporary environment.

Messages are plain text without Telegram Markdown/HTML parsing, bounded for provider limits. Topic IDs, silent notifications, HTTP errors and Bot API `ok` are supported. [Telegram Bot API](https://core.telegram.org/bots/api#sendmessage)

## Discord

Create an incoming webhook in the destination channel:

```json
{"discord": {"enabled": true, "url": "https://discord.com/api/webhooks/WEBHOOK_ID/WEBHOOK_TOKEN"}}
```

Delivery uses bounded embeds, `wait=true`, and disabled `allowed_mentions`, so an `@everyone` string cannot ping the server. Rate-limit responses supply retry timing. Confirm channel permissions. [Discord webhook reference](https://docs.discord.com/developers/resources/webhook#execute-webhook)

## Slack, Feishu and Lark

```json
{
  "slack": {"enabled": true, "url": "https://hooks.slack.com/services/WORKSPACE/CHANNEL/TOKEN"},
  "feishu": {"enabled": true, "url": "https://open.feishu.cn/open-apis/bot/v2/hook/PLACEHOLDER", "secret": "", "location_language": "zh-CN"}
}
```

Slack uses incoming webhooks and Block Kit. Feishu/Lark uses bot cards; configure the local signing secret if required and keep the clock correct. Lark URLs work through `feishu.url` or `webhooks[].format="lark"`. Feishu/Lark business errors are checked even on HTTP 200.

## Generic webhooks and email

```json
{
  "webhooks": [{"name": "operations", "enabled": true, "url": "https://alerts.example.com/hooks/banhack233", "format": "json", "headers": {}}],
  "email": {"enabled": true, "from": "sender@example.com", "to": "operator@example.com", "password": "LOCAL_SMTP_CREDENTIAL", "smtp_host": "smtp.example.com", "smtp_port": 465}
}
```

Formats: `text`, `json`, `discord`, `slack`, `feishu`, `lark`. Keep Authorization headers local. Redirects are rejected to avoid forwarding credentials; configure the final endpoint. SMTP presets cover QQ, 163, 126, Gmail, Outlook, Hotmail and Live. Only missing host/port values are inferred. Port 465 uses implicit TLS; other ports require STARTTLS. Use the provider's supported SMTP/app credential.

## Queue and frequency

| Setting | Behavior |
| --- | --- |
| Batching | 60 seconds from the first event, or 20 events by default |
| Scan isolation | Daemon enqueues; a separate worker performs delivery |
| Channel isolation | Independent concurrent channels, each with a 10-second deadline |
| Persistence | `state_path + ".notifications.json"`, private file, atomic replacement |
| Retry | Failed channels only; 1, 2, 4… minutes up to one hour, honoring longer Retry-After |
| Retention | Up to 1000 alerts and 24 hours per alert; expiry is logged |
| Full queue | New enqueue fails and is logged; protection scans continue |
| Disabled channels | Their queued deliveries are dropped |

The queue stores events, not endpoint URLs or credentials. Events may contain sensitive addresses/process details. Reordering generic webhook entries changes their queue identity; drain pending deliveries first. A crash after receiver acceptance but before local acknowledgement can duplicate a message. This is bounded retry, not exactly-once messaging. Overseas APIs require working host connectivity; standard HTTP proxy environment variables are supported.

## Acceptance and troubleshooting

```sh
banhack233 config-check
banhack233 notify-test -channel telegram
banhack233 notify-test -channel discord
banhack233 notify-test -channel feishu,slack,email
banhack233 notify-test -channel webhook:operations
```

Manual tests send immediately without using the retry queue and fail with nonzero exit status. Verify actual receipt, destination, permissions and backlog. Telegram 400 often means chat/topic ID, 401 token, 403 permission or a user who has not started the bot, and 429 rate limiting. Logs omit token URLs and provider error bodies. Automated tests use mock servers; live acceptance needs your local credentials.
