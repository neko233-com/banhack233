# Changelog

## v0.2.1 — 2026-10-11

- 发布构建使用 Go 1.27.2，修复 macOS 26 加载旧工具链产物失败；v0.2.0 标签未通过门禁，没有发布 Release 或更新网站。
- 生产模式启动时立即将本项目遗留 nft 全端口规则迁移为配置的 SSH 端口，保留已有封禁集合，无需等待新失败事件。

- 补充中英文命令参考、完整配置参考、部署与恢复手册、排障 FAQ、工作原理与平台边界。
- 默认仅计密码错误，成功登录优先处理并给予 10 分钟宽限；提供 `safe-ssh` 旧规则迁移，保留公司白名单与阈值。
- Windows 事件 ID 游标/去重、本地目标端口修正；新增 `ssh_ports`、文件身份/半行处理、多规则独立游标、状态写入锁。
- 新增 Telegram；Discord 禁止提及、限制消息长度；渠道独立并发、10 秒超时、业务错误检查、有界持久重试。SMTP 使用 TLS/STARTTLS。
- `config-check` 严格验证；修复 JSON 数组隐式继承默认规则的问题；生产切换只改 dry_run。状态/诊断/定时巡检只读，临时目录和合法挖矿工具不单独触发清理；报告不覆盖同名文件。
- 每日正式 Release 更新：资产 SHA256、版本与配置校验、保留配置/状态、旧二进制备份、服务重启失败回滚；Linux timer、macOS launchd、Windows 隐藏 helper。
- 安装器校验下载并生成本平台配置；维护运行时文件的 gitignore。HTML 增至 19 页，中英文新增通知与自动更新专题。
- Password-only defaults, successful-login grace, safer diagnostics, Telegram and durable channel-isolated delivery, verified daily updates and rollback, cross-platform cursor/port fixes, strict validation and bilingual operational documentation.

旧配置不自动迁移：升级后运行 `safe-ssh` 预览，再按需 `safe-ssh -write` 并重启。每日更新需显式 `auto-update -enable`；规则、白名单和凭据保持不变。Existing configs are preserved; migrate the default SSH rule explicitly and opt into daily updates.

## v0.1.22 — 2026-10-06

- 新增 GitHub Pages 中英文 HTML 文档：章节检索、命令复制、配置下载、移动端目录及离线包。
- 文档直接从同一 Release 标签的 README、发布说明和版本记录生成，并显示版本与提交。
- Actions 仅由正式 `vX.Y.Z` 标签触发。普通 push/PR 不运行；测试、打包和 Release 发布成功后才更新 Pages。
- 新增旧版本覆盖保护、发布并发控制、草稿上传后发布、资产校验和与非空发布说明。
- 修正共享出口章节中过时的默认检测说明，补充仅密码失败配置与完整发布流程。
- Bilingual HTML docs now ship through GitHub Pages and an offline archive. Stable tags alone trigger the gated release pipeline; failed or older releases cannot replace current docs.

## v0.1.21 — 2026-10-03

- `<HOST>` 宏支持 IPv4/IPv6；双栈封禁与解封。
- 无效日志 IP 跳过处理，不中断扫描。
- Added dual-stack host matching and resilient handling of malformed log lines.

## v0.1.20 — 2026-10-03

- 扩展 SSH 认证失败日志检测；成功登录重置该 IP 的失败计数。
- Expanded SSH authentication-failure detection and reset counters after successful login.

## v0.1.19 — 2026-10-03

- 支持共享出口白名单、按用户计数、地区阈值与到期解封。
- 默认封禁范围限制到 SSH 端口；运维与集成检查工具改为 Go。
- Added office whitelist controls, per-user counters, regional thresholds, expiry handling, and SSH-port-scoped enforcement.

较早版本见 [GitHub Releases](https://github.com/neko233-com/banhack233/releases)。Earlier versions are available on GitHub Releases.
