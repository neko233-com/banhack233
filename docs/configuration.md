# 配置参考

本页说明当前代码的读取规则、默认值和边界；完整样例见 [config.json.example](../configs/config.json.example)。操作步骤见 [运维手册](operations.md)，命令副作用见 [命令参考](commands.md)。

## 配置如何生效

- 使用 JSON，不支持注释、尾逗号或环境变量插值。时长写字符串，例如 `"30s"`、`"10m"`、`"24h"`，不能写 `"1d"`。
- 先加载程序默认值，再覆盖文件中的字段。遗漏的顶层字段保留默认值；显式提供 `rules` 数组会替换默认规则，数组元素不会逐项继承默认 SSH 模板。
- 例如自定义规则遗漏 `count_by_user` 时值为 `false`；遗漏 `reset_patterns` 时没有成功登录重置规则。不要把局部规则片段当作完整规则。
- 文件缺失、未知字段、非法正则、非法端口、重复规则名会报错；重启前先运行 `config-check`。
- `config-check` 只校验，不扫描、不修改防火墙、不发通知。
- 守护进程启动时加载配置，不热更新。修改后重启对应服务；所有相关命令和服务应使用同一个 `-config` 路径。
- 安装器调用本平台二进制的 `init-config`；已有文件保持不变。

## 顶层字段

| 字段 | 默认值 | 行为 |
| --- | --- | --- |
| `interval` | `"30s"` | 扫描轮询周期；启动后先执行一轮。小于等于零恢复默认值 |
| `audit_interval` | `"1h"` | 扫描周期内检查是否需要巡检；不是独立定时器 |
| `state_path` | 平台状态目录下 `state.json` | 保存日志偏移、命中时间、封禁、后端、冷却和上次巡检时间 |
| `dry_run` | `true` | 不新增或解除真实防火墙封禁；仍会写状态、日志和通知。不是所有命令的全局只读开关 |
| `start_at_end` | `true` | 首次文本读取从末尾开始；首次 Windows 查询仅记录最新事件 ID，不重放历史 |
| `ignore_ips` | `["127.0.0.1", "::1"]` | 单个 IPv4、IPv6 或 CIDR；不支持域名。显式数组会替换默认列表 |
| `rules` | 一条 SSH 认证失败规则 | `[]` 停止规则扫描，但不会自动停掉巡检或立即清空已有防火墙规则 |

同一状态路径有进程锁，拒绝两个守护进程或 `test` 同时写入。隔离回放使用独立状态和通知队列路径。

## SSH 规则字段

| 字段 | 默认 SSH 规则 / 归一化值 | 行为 |
| --- | --- | --- |
| `name` | `ssh-auth-failure`；空值变为 `rule` | 状态键的一部分；每条规则用唯一名称，避免 `|` |
| `log_paths` | 本机认证日志 | Linux 默认在 `auth.log` 存在时选它，否则选 `secure`；样例同时列两者，缺失文件会跳过 |
| `patterns` | 一条明确的密码失败表达式 | Go 正则，一个 `<HOST>` 或命名 IP 组，可选用户名组 |
| `reset_patterns` | `Accepted ... from <HOST>` | 优先处理，清空当前规则下该 IP 的所有用户名失败计数；不解除已存在封禁 |
| `max_attempts` | `5` | 达到阈值即触发；是匹配日志事件数，不是严格的密码输入次数 |
| `find_time` | `"10m"` | 保留扫描时刻在窗口内的失败记录；不按日志文本中的历史时间计算 |
| `ban_time` | `"1h"` | 真实封禁的到期时间；在 dry-run / notify 模式用于同一计数键的通知冷却 |
| `action` | `"auto"` | 自动选择后端；`"notify"` 仅通知；其他值作为可执行文件调用，并传入 IP |
| `count_by_user` | 默认 SSH 规则为 `true` | 能提取用户名时按规则+IP+用户名累计，否则退回规则+IP。最终仍按 IP 封禁 |
| `region_rules.max_attempts` | `广州: 100`、`Guangzhou: 100` | GeoIP 命中地区时覆盖阈值；无匹配或查询失败时使用 `max_attempts` |

`max_attempts`、`find_time`、`ban_time` 小于等于零会恢复归一化默认值，不表示“永不封禁”。地区规则的空名称或非正阈值会被删除。

自定义 `action` 是可执行程序名，参数只有 IP，不是 shell 命令或后端枚举。托管规则用 `auto`。顶层 `ssh_ports` 默认 `[22]`，多个 SSH 端口显式设为 `[22,2222]`；此字段不会修改 sshd 的监听端口。

地区匹配忽略大小写，支持国家/省/城市部分名称；多个地区命中时确定性选择最高阈值，降低误封。数据库仅 IPv4；IPv6 检测不依赖 GeoIP。公司出口优先用白名单。

## 仅统计密码失败

修改已有 SSH 规则的 `patterns`，保留该规则其他字段，特别是 `reset_patterns`、阈值和 `count_by_user`：

```json
{
  "patterns": ["Failed password for(?: invalid user)? (?P<user>\\S+) from <HOST>"]
}
```

这只是规则片段，不是完整配置。不会按每分钟建立多少次连接封禁；`MaxAuthTries` 是 sshd 单次连接内的认证限制。公司共用出口仍需 `ignore_ips`，详见 [共享出口操作](operations.md)。

## SSH 基线

以下字段位于 `hardening.ssh`。只由 `secure-ssh` 预览/写入；运行守护进程不会自动改写 sshd。

| 字段 | 默认值 | 行为 |
| --- | --- | --- |
| `enabled` | `true` | 关闭时 `secure-ssh` 不应用基线 |
| `password_authentication` | `true` | 支持密码认证 |
| `permit_root_login` | `"yes"` | 支持 root 登录；配置空字符串时渲染为 `no` |
| `allowed_users` | `[]` | 非空时生成 `AllowUsers`；写入要求非空，或显式 `-force` |
| `max_auth_tries` | `3` | 单连接认证尝试上限 |
| `login_grace_time` | `"20s"` | 完成认证的宽限时间 |
| `client_alive_interval` | `300` | SSH 服务端探测间隔，秒 |
| `client_alive_count_max` | `2` | 未响应探测次数 |
| `disable_empty_passwords` | `true` | 生成 `PermitEmptyPasswords no` |
| `disable_challenge_response` | `true` | 关闭 keyboard-interactive / challenge-response；使用这些认证方式时须调整 |

`keepalive` 是独立命令，不读取上述 JSON，也没有 `-config` 参数。它的预览包含密码/root 登录设置和保活参数；与 `secure-ssh` 的写入位置不同。使用前先比较输出，使用后检查 sshd 实际生效值。

## 日志、地区与恶意程序扫描

| 字段 | 默认值 | 行为 |
| --- | --- | --- |
| `logging.enabled` | `true` | 写应用轮转日志 |
| `logging.path` | 状态目录下 `banhack233.log` | 记录封禁、巡检和错误；不是 sshd 认证日志输入 |
| `logging.max_size_mb` | `10` | 单文件轮转大小 |
| `logging.max_age_days` | `30` | 旧轮转文件保留天数 |
| `geoip.enabled` | `true` | 地区阈值和通知地区补充 |
| `geoip.db_path` | 状态目录下 `ip2region_v4.xdb` | 文件缺失时无地区信息；不阻止基础阈值检测 |
| `malware.enabled` | `true` | Linux 自动巡检是否包含恶意程序扫描；手动 `malware-scan` 仍会扫描 |
| `malware.direct_kill` | `false` | 唯一清理开关，仅手动扫描处理限定的入侵程序名；巡检始终只读 |
| `malware.report_dir` | 状态目录下 `reports` | 手动扫描报告目录；不要与其他 `.txt` 文档混放 |
| `malware.report_keep` | `50` | 写报告后按修改时间保留最新 N 个 `.txt`，淘汰旧文件；非正值恢复 50 |

这是轻量 Linux 主机杀毒/入侵排查辅助，不是完整商业杀毒或 EDR。手动报告格式为 `{名字}.yyyy-MM-dd_HH-mm-ss.txt`；当前数量淘汰按修改时间实现“保留最新”的 LRU 类策略，不按读取次数刷新。自动巡检写应用日志，不能代替手动报告归档。

## 通知字段

所有字段位于 `notifications`。配置示例和具体渠道格式见 [README 通知说明](../README.md)。

| 字段 | 默认值 | 行为 |
| --- | --- | --- |
| `console` | `true` | 向进程标准输出写通知 |
| `audit` | `false` | 是否发送巡检通知；不会关闭巡检本身 |
| `batch.enabled` | `true` | 批量封禁事件；notify/巡检直接进入持久队列 |
| `batch.interval` | `"60s"` | 从第一条待发事件开始计时，独立工作协程每秒检查 |
| `batch.max_items` | `20` | 待发数量达到阈值则发送 |
| `feishu` / `discord` / `slack` | `enabled=false` | 各含 `enabled`、`url`、`location_language`；Feishu/Lark 可使用 `secret` 签名 |
| `webhooks[]` | 空 | `name`、`enabled`、`url`、`format`、`headers`、`secret`、`location_language` |
| `webhooks[].format` | 空值为纯文本 | 支持 `text`、`json`、`feishu`、`lark`、`discord`、`slack` |
| `email.enabled` | `false` | 开启 SMTP 邮件通知 |
| `email.from` / `to` | 空 | 发件邮箱、逗号分隔的收件邮箱 |
| `email.password` | 空 | SMTP 授权码或适用凭据，不是仓库中的真实密码示例 |
| `email.smtp_host` / `smtp_port` | 空 / `0` | 只推断缺失的值；465 使用 TLS，其他端口必须支持 STARTTLS |
| `email.location_language` | 空 | 通知地区显示语言；不会翻译整篇通知 |

通知使用有界持久队列，各渠道独立、10 秒超时；检查 Telegram/飞书业务错误，只重试失败渠道。见 [投递语义与配置](notifications.md)。

## 生产快捷命令会改什么

`enable-production` 只修改 `dry_run=false`，保留其他偏好；之后需要重启服务。

升级不静默改写已有规则。用 `safe-ssh` 预览、`safe-ssh -write` 迁移原默认规则。

源码依据：[配置结构与默认值](../internal/config/config.go)、[扫描逻辑](../internal/daemon/daemon.go)、[命令入口](../cmd/banhack233/main.go)。

## 新增防误伤与投递字段

- `ssh_ports`：目标 TCP 端口，默认 `[22]`（Linux/Windows），与真实 SSH 监听端口保持一致；PF 由管理员配置。
- 规则 `success_grace`：默认模板 `"10m"`；自定义规则省略时为 `"0s"`。成功登录对同规则该来源 IP 的所有用户名提供宽限；即使为零，同一读取批次仍优先处理成功。不会解除已存在封禁。
- `notifications.queue_path`：默认 `state_path + ".notifications.json"`；需私有可写路径，不能与状态文件相同。
- `notifications.telegram`：`enabled`、`bot_token` 或 `bot_token_env`、`chat_id`，可选 `message_thread_id`、`disable_notification`。只有 `bot_token_env` 读取指定环境变量，不做通用 JSON 变量替换。
