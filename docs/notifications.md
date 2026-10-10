# 通知渠道、海外接入与可靠投递

支持 Telegram、Discord、Slack、Feishu/Lark、通用 webhook 和邮箱。下列 JSON 是 `notifications` 内的片段，合并到实际配置；示例都是占位符。完整字段见 [配置参考](configuration.md)。

## Telegram

1. 在 Telegram 官方 `@BotFather` 创建机器人，令牌保存在服务器本地。
2. 私聊机器人并发送 `/start`；群组则将机器人加入群并允许发送消息。频道需要给予机器人发消息权限。
3. 使用自己的 Bot API `getUpdates` 响应核对 `chat.id`，不要把聊天标题当 ID。论坛话题还要核对 `message_thread_id`。
4. 配置并运行 `notify-test -channel telegram`，在目标聊天确认收件。

```json
{
  "telegram": {
    "enabled": true,
    "bot_token_env": "BANHACK233_TELEGRAM_BOT_TOKEN",
    "chat_id": "-1000000000000",
    "message_thread_id": 42,
    "disable_notification": false
  }
}
```

普通聊天删除 `message_thread_id`。可改用本地配置中的 `bot_token`，不要提交真实值。指定 `bot_token_env` 时以环境变量为准，不回退到文件令牌；变量必须存在于**守护进程的运行环境**。

Linux systemd 可通过 `systemctl edit banhack233` 添加：

```ini
[Service]
EnvironmentFile=/etc/banhack233/notifications.env
```

创建仅 root 可读的环境文件，保存 `BANHACK233_TELEGRAM_BOT_TOKEN=你的令牌`；执行 `systemctl daemon-reload` 和 `systemctl restart banhack233`。手动 `notify-test` 也需要相同变量；不要将 `systemctl show ... Environment` 的含密输出贴到 Issue。Windows SYSTEM 任务不自动继承当前用户临时设置的变量。

消息使用纯文本，避免日志内容被当成 Telegram Markdown/HTML 指令；长消息截断到安全长度。支持话题、静默通知，检查 HTTP 状态和 Bot API 的 `ok`。参见 [Telegram Bot API](https://core.telegram.org/bots/api#sendmessage)。

## Discord

在目标服务器频道创建 incoming webhook，将 URL 只保存在本机配置中：

```json
{"discord": {"enabled": true, "url": "https://discord.com/api/webhooks/WEBHOOK_ID/WEBHOOK_TOKEN"}}
```

发送 embed 卡片，设置 `wait=true`，禁用 `allowed_mentions`，限制字段与正文长度。日志中的 `@everyone` 不会触发全员提醒。429 会读取服务端重试时间；确认机器人仍有目标频道的访问权限。[Discord webhook 文档](https://docs.discord.com/developers/resources/webhook#execute-webhook)

## Slack、飞书与 Lark

```json
{
  "slack": {"enabled": true, "url": "https://hooks.slack.com/services/WORKSPACE/CHANNEL/TOKEN"},
  "feishu": {"enabled": true, "url": "https://open.feishu.cn/open-apis/bot/v2/hook/PLACEHOLDER", "secret": "", "location_language": "zh-CN"}
}
```

Slack 使用 incoming webhook 和 Block Kit。飞书/Lark 使用机器人卡片；开启签名时填写本机 `secret` 并保持时钟准确。Lark 可使用 `feishu.url` 的 Lark 地址，也可配置 `webhooks[].format="lark"`。程序检查飞书/Lark 的业务错误码，HTTP 200 不再直接视为成功。

## 通用 webhook 与邮箱

```json
{
  "webhooks": [{"name": "operations", "enabled": true, "url": "https://alerts.example.com/hooks/banhack233", "format": "json", "headers": {}}],
  "email": {"enabled": true, "from": "sender@example.com", "to": "operator@example.com", "password": "LOCAL_SMTP_CREDENTIAL", "smtp_host": "smtp.example.com", "smtp_port": 465}
}
```

Webhook 格式支持 `text`、`json`、`discord`、`slack`、`feishu`、`lark`。自定义 Authorization 头仅留在本地。HTTP 重定向被拒绝，避免凭据被转发到其他地址；应填写最终接收端 URL。

邮箱保留 QQ/163/126/Gmail/Outlook/Hotmail/Live 预设，只推断缺失的主机或端口。465 使用隐式 TLS，其他端口必须 STARTTLS；认证密码使用邮箱服务支持的授权码/应用密码。普通邮箱账号密码未必可用于 SMTP。

## 队列与频率

| 项目 | 行为 |
| --- | --- |
| 批量 | 默认 60 秒或 20 条；从首条事件计时 |
| 扫描隔离 | 守护进程只入队；独立工作协程投递，不阻塞扫描 |
| 渠道隔离 | 同一告警的渠道独立并发，单渠道最多 10 秒 |
| 持久化 | 默认 `state_path + ".notifications.json"`；0600 私有文件，原子替换 |
| 重试 | 只重发失败渠道；1、2、4…分钟，普通退避上限 1 小时；尊重较长 Retry-After |
| 保留 | 最多 1000 条告警；单条最多 24 小时，过期记录错误后丢弃 |
| 队列满 | 新告警入队失败并写本地错误；防护扫描继续 |
| 关闭渠道 | 关闭后的渠道不再补发其旧消息 |

队列保存事件，不保存 webhook URL、令牌或 SMTP 密码；事件本身可能包含 IP、用户名或进程信息，应同样限制访问。修改 webhooks 数组顺序会改变通用渠道的队列身份，维护前先处理积压。进程在接收端收到消息后、保存成功状态前中断，重启可能重复发送；这是有界重试，不是 exactly-once 消息系统。无法连接海外 API 时，需要主机可用的网络出口；HTTP 客户端支持标准代理环境变量。

## 验收与排障

```sh
banhack233 config-check
banhack233 notify-test -channel telegram
banhack233 notify-test -channel discord
banhack233 notify-test -channel feishu,slack,email
banhack233 notify-test -channel webhook:operations
```

手动测试立即发送，不写重试队列；错误返回非零。确认目标收件、时间、主机来源、权限和队列积压。Telegram 400 常见于 chat/topic ID，401 为令牌，403 为权限或用户未启动机器人，429 为限流。日志不输出令牌 URL 或接收端错误正文。真实渠道需要你本机的凭据；仓库自动测试使用模拟 HTTP 服务。
