# Changelog

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
