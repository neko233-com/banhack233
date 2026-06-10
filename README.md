# banhack233

自适应主机防护守护进程。目标平台：Linux Ubuntu/Debian/CentOS，Windows。

## 是否等效 fail2ban

不是逐项复制 fail2ban，而是覆盖常用主机防入侵场景：

- 自动扫 SSH 登录失败日志。
- 达阈值后封禁 IP。
- Linux 使用 `nft`/`iptables`，Windows 使用防火墙。
- 内置通知和安全巡检。
- 默认每 1 小时 `doctor` 审计一次。
- 默认 `start_at_end=true`，新安装只监控新日志，不因历史爆破记录狂刷通知。
- 支持 `ignore_ips` 白名单、防误封。
- 支持保留 SSH 密码登录，但禁 root 密码、低重试、短登录宽限、禁空密码。

功能：

- fail2ban 风格日志扫描：SSH 登录失败等规则可配置。
- 自动封禁：Linux 优先 `nft`，降级 `iptables`；Windows 使用防火墙规则。
- 通知：控制台、飞书 webhook、邮箱。
- 邮箱自动识别 SMTP：`@qq.com`、`@163.com`、Gmail、Outlook/Hotmail/Live。
- 开机自启动：Linux systemd，Windows schtasks。
- 一键安装脚本：`.sh`、`.ps1`。
- GitHub Actions：跨平台测试、构建、发布。

## 安装

Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
sudo nano /etc/banhack233/config.json
sudo banhack233 doctor
sudo banhack233 ban-list
sudo banhack233 secure-ssh
# 确认 allowed_users 等配置后再执行:
sudo banhack233 secure-ssh -write
sudo banhack233 install-autostart
```

Windows PowerShell:

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.ps1 | iex
& "$env:LOCALAPPDATA\banhack233\banhack233.exe" install-autostart -config "$env:LOCALAPPDATA\banhack233\config.json"
```

## 本地运行

```sh
go test ./...
go run ./cmd/banhack233 version
go run ./cmd/banhack233 init-config -config ./config.local.json
go run ./cmd/banhack233 doctor -config ./config.local.json
go run ./cmd/banhack233 secure-ssh -config ./config.local.json
go run ./cmd/banhack233 test -config configs/config.json.example
```

默认配置 `dry_run=true`，只告警不封禁。生产封禁前改为 `false`。

## SSH 密码登录安全建议

想保留密码登录时，建议：

- 禁止 root 密码登录：`PermitRootLogin no`。
- 只允许指定普通用户：配置 `hardening.ssh.allowed_users`。
- 使用 20 位以上随机强密码。
- `MaxAuthTries 3`、`LoginGraceTime 20s`。
- 禁空密码、禁交互式 challenge response。
- 开启飞书/邮箱通知。
- 观察 24 小时后把 `dry_run` 改为 `false`。
- 云厂商安全组只开放必要端口；SSH 端口可只放行常用管理 IP。

## 配置

见 [configs/config.json.example](configs/config.json.example)。

规则字段：

- `log_paths`: 扫描日志文件。
- `patterns`: 正则，推荐使用命名组 `(?P<ip>...)` 提取 IP。
- `max_attempts`: 窗口内最大失败次数。
- `find_time`: 检测窗口。
- `ban_time`: 封禁时长记录，用于避免重复通知。
- `action`: `auto` 或自定义命令路径。
- `start_at_end`: 首次运行从日志末尾开始，避免处理多年历史日志。
- `ignore_ips`: 永不封禁的 IP。

邮箱说明：QQ/163 常用为授权码，不是登录密码。自定义服务商填 `smtp_host`、`smtp_port`。
