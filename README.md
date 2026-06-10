# banhack233

## 一行安装

Linux / macOS / Windows Git Bash 或 MSYS:

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.ps1 | iex
```

安装后先看状态：

```sh
banhack233 status
```

默认 `dry_run=true`，只告警、不真封禁。确认无误后再改成 `dry_run=false`。

## 项目定位

`banhack233` 是傻瓜式主机防入侵工具，主要解决这些问题：

- 服务器需要 SSH 密码登录，甚至需要 root 密码登录，但仍然要防爆破。
- 需要自动扫 SSH 登录失败日志，超过阈值后封禁 IP。
- 需要开机自启动、通知告警、安全巡检、默认每 1 小时审计一次。
- 需要 SSH 至少 24 小时不断线。
- 需要 Linux、macOS、Windows 尽量自适应。

English docs: [README-EN.md](README-EN.md)

## 类似软件

`banhack233` 不是逐项复制某一个项目，而是面向常见主机场景组合能力。类似或参考的软件/能力包括：

1. `fail2ban`：日志扫描、阈值判断、封禁攻击 IP。
2. `sshguard`：SSH 暴力破解防护。
3. `denyhosts`：登录失败来源识别与阻断思路。
4. `crowdsec`：安全事件检测、封禁、告警生态。
5. `ufw` / `nftables` / `iptables` / Windows 防火墙：实际封禁后端。

## 功能

1. 系统安全巡检：SSH 策略、防火墙后端、系统更新/重启需求、通知渠道等。
2. SSH 登录失败扫描：支持 auth.log、secure、Windows OpenSSH event log。
3. 自动封禁：Linux 优先 `nft`，降级 `iptables`；Windows 使用防火墙规则。
4. root 密码登录支持：不强制禁 root，不强制禁密码登录。
5. SSH 安全基线：禁空密码、低重试、短登录宽限、保留强密码场景。
6. SSH 24 小时保活：默认只改 SSH keepalive，不改系统 TCP sysctl。
7. TCP 系统保活高级项：必须显式 `-tcp` 才会写全局 TCP keepalive/conntrack 参数。
8. 多渠道通知：控制台、飞书/Lark、Discord、Slack、通用 webhook、邮箱。
9. 邮箱 SMTP 自动识别：QQ、163、126、Gmail、Outlook、Hotmail、Live 等。
10. 开机自启动：Linux systemd、macOS launchd、Windows schtasks。
11. 安全默认值：`dry_run=true`、`start_at_end=true`、`ignore_ips` 白名单。

## 支持平台

| 平台 | 架构 | 安装方式 | 自启动 |
| --- | --- | --- | --- |
| Linux | amd64 / arm64 | `install.sh` | systemd |
| macOS | amd64 / arm64 | `install.sh` | launchd |
| Windows | amd64 / arm64 | `install.ps1` 或 Git Bash `install.sh` | schtasks |

Linux 默认配置路径：

```text
/etc/banhack233/config.json
/var/lib/banhack233/state.json
```

macOS 默认配置路径：

```text
/usr/local/etc/banhack233/config.json
/usr/local/var/banhack233/state.json
```

Windows 默认配置路径：

```text
%ProgramData%\banhack233\config.json
%ProgramData%\banhack233\state.json
```

非管理员 PowerShell 安装时使用：

```text
%LOCALAPPDATA%\banhack233\config.json
```

## 快速开始

1. 安装：

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
```

2. 查看状态：

```sh
banhack233 status
```

3. 检查主机安全建议：

```sh
banhack233 doctor
```

4. 开启 SSH 24 小时保活：

```sh
sudo banhack233 keepalive -write
```

5. 开启开机自启动：

```sh
sudo banhack233 install-autostart
```

6. 观察一段时间后开启真实封禁：

```sh
sudo sed -i 's/"dry_run": true/"dry_run": false/' /etc/banhack233/config.json
sudo systemctl restart banhack233
```

## 常用命令

### 查看整体状态

```sh
banhack233 status
```

输出包含：

- 版本。
- 配置路径。
- 是否 dry-run。
- 是否从日志末尾开始监控。
- 自启动后端和状态。
- 规则数量。
- 当前审计结果。

### 安全审计

```sh
banhack233 doctor
```

会检查：

- SSH 密码登录策略。
- root 登录策略。
- 空密码是否禁止。
- SSH 最大尝试次数。
- SSH 登录宽限时间。
- 防火墙后端是否可用。
- 系统是否需要重启。
- 是否仍在 dry-run。
- 通知渠道是否开启。

不会把 Redis、MySQL、自研 TCP 服务等业务端口当默认问题。业务软件由业务自己决定是否公开。

### 预览 SSH 配置

```sh
banhack233 secure-ssh
```

预览要写入的 SSH 配置，不会改系统。

### 应用 SSH 基线

```sh
sudo banhack233 secure-ssh -write -force
```

默认策略支持：

- `PasswordAuthentication yes`
- `PermitRootLogin yes`
- `PermitEmptyPasswords no`
- `MaxAuthTries 3`
- `LoginGraceTime 30s`

注意：如果你当前只靠 SSH 管理服务器，执行前建议保留一个已登录 session，避免误配置导致无法连回。

### SSH 24 小时保活

```sh
sudo banhack233 keepalive -write
```

默认只写入 SSH 配置，不修改系统 TCP keepalive / conntrack 参数，避免影响业务 TCP 长连接、HTTP、数据库连接池等服务。

会写入：

```text
/etc/ssh/sshd_config.d/99-banhack233-keepalive.conf
```

SSH 配置：

```text
PasswordAuthentication yes
PermitRootLogin yes
TCPKeepAlive yes
ClientAliveInterval 60
ClientAliveCountMax 1440
PermitEmptyPasswords no
MaxAuthTries 3
LoginGraceTime 30s
```

含义：SSH 每 60 秒保活一次，连续 1440 次，约 24 小时。

### TCP 系统保活高级项

默认不修改 TCP 系统参数。如果你明确知道服务器需要全局 TCP keepalive / conntrack 调整，可手动加 `-tcp`：

```sh
sudo banhack233 keepalive -write -tcp
```

这会额外写入：

```text
/etc/sysctl.d/99-banhack233-keepalive.conf
```

TCP 系统配置为：

```text
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.netfilter.nf_conntrack_tcp_timeout_established = 432000
```

含义：

- TCP keepalive 60 秒开始探测。
- 探测间隔 30 秒。
- 连续 10 次失败才认为断开。
- conntrack established 超时 5 天。

这能防止系统、NAT、conntrack 太快清理业务长连接，但属于全局内核参数，会影响整台机器上的 TCP/HTTP/数据库/代理等连接行为。生产服务器建议先评估业务连接模型，再决定是否启用。

如果业务服务代码自己设置了 5 分钟 idle timeout，需要同时修改应用层心跳或超时配置。

### 开机自启动

Linux:

```sh
sudo banhack233 install-autostart
systemctl status banhack233
```

macOS:

```sh
sudo banhack233 install-autostart
sudo launchctl print system/com.neko233.banhack233
```

Windows:

```powershell
banhack233.exe install-autostart
schtasks /Query /TN banhack233
```

### 查看封禁列表

```sh
banhack233 ban-list
```

Linux 优先使用 `nft`，没有则使用 `iptables`。

### 解封 IP

```sh
sudo banhack233 unban 203.0.113.10
```

### 测试一次扫描

```sh
banhack233 test
```

## 配置文件

示例：`configs/config.json.example`

核心字段：

```json
{
  "interval": "30s",
  "audit_interval": "1h",
  "state_path": "/var/lib/banhack233/state.json",
  "dry_run": true,
  "start_at_end": true,
  "ignore_ips": ["127.0.0.1", "::1"],
  "rules": [
    {
      "name": "ssh-auth-failure",
      "log_paths": ["/var/log/auth.log", "/var/log/secure"],
      "patterns": [
        "Failed password.*from (?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)",
        "Invalid user .* from (?P<ip>\\d+\\.\\d+\\.\\d+\\.\\d+)"
      ],
      "max_attempts": 5,
      "find_time": "10m",
      "ban_time": "1h",
      "action": "auto"
    }
  ]
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `interval` | 守护进程扫描间隔 |
| `audit_interval` | 自动安全审计间隔 |
| `state_path` | 状态文件路径 |
| `dry_run` | `true` 时只告警，不真封禁 |
| `start_at_end` | 首次运行从日志末尾开始，避免扫历史日志刷屏 |
| `ignore_ips` | 永不封禁白名单 |
| `rules` | 检测规则列表 |
| `max_attempts` | 时间窗口内失败次数阈值 |
| `find_time` | 检测时间窗口 |
| `ban_time` | 状态里记录的封禁时长 |
| `action` | `auto` 或自定义命令 |

## 通知策略

支持这些通知：

| 渠道 | 配置字段 | 说明 |
| --- | --- | --- |
| 控制台 | `console` | 输出到 stdout / systemd journal |
| 飞书 / Lark | `feishu` | 飞书机器人 text 消息 |
| Discord | `discord` | Discord webhook `content` 消息 |
| Slack | `slack` | Slack incoming webhook `text` 消息 |
| 通用 webhook | `webhooks[]` | 支持 `text`、`json`、`discord`、`slack`、`feishu` 格式 |
| 邮箱 | `email` | SMTP，支持常见邮箱自动识别 |

`webhooks[]` 适合接企业内部通知网关、告警平台、OpenClaw 类聚合通知服务。

## 飞书通知

配置：

```json
{
  "notifications": {
    "console": true,
    "feishu": {
      "enabled": true,
      "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
    }
  }
}
```

测试：

```sh
banhack233 test
```

当触发规则或审计有发现时，会发送飞书消息。

## Discord 通知

配置：

```json
{
  "notifications": {
    "discord": {
      "enabled": true,
      "url": "https://discord.com/api/webhooks/xxx/yyy"
    }
  }
}
```

Discord 使用 `{"content":"..."}` payload。

## Slack 通知

配置：

```json
{
  "notifications": {
    "slack": {
      "enabled": true,
      "url": "https://hooks.slack.com/services/xxx/yyy/zzz"
    }
  }
}
```

Slack 使用 `{"text":"..."}` payload。

## 通用 webhook 通知

配置：

```json
{
  "notifications": {
    "webhooks": [
      {
        "name": "internal-alert",
        "enabled": true,
        "url": "https://example.com/webhook",
        "format": "json",
        "headers": {
          "Authorization": "Bearer token"
        }
      }
    ]
  }
}
```

`format` 支持：

| format | payload |
| --- | --- |
| `text` | 纯文本 |
| `json` | `{"text":"..."}` |
| `discord` | `{"content":"..."}` |
| `slack` | `{"text":"..."}` |
| `feishu` / `lark` | `{"msg_type":"text","content":{"text":"..."}}` |

## 邮箱通知

配置：

```json
{
  "notifications": {
    "email": {
      "enabled": true,
      "from": "alert@qq.com",
      "to": "admin@example.com",
      "password": "smtp-authorization-code",
      "smtp_host": "",
      "smtp_port": 0
    }
  }
}
```

自动识别 SMTP：

| 邮箱 | SMTP |
| --- | --- |
| `@qq.com` | `smtp.qq.com:465` |
| `@163.com` | `smtp.163.com:465` |
| `@126.com` | `smtp.126.com:465` |
| `@gmail.com` | `smtp.gmail.com:587` |
| `@outlook.com` | `smtp-mail.outlook.com:587` |
| `@hotmail.com` | `smtp-mail.outlook.com:587` |
| `@live.com` | `smtp-mail.outlook.com:587` |

QQ、163 通常需要 SMTP 授权码，不是网页登录密码。

## 防爆破工作流演示

1. 先 dry-run 安装：

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
sudo banhack233 install-autostart
```

2. 查看是否捕获攻击：

```sh
banhack233 status
journalctl -u banhack233 -n 100 --no-pager
```

3. 确认白名单：

```json
"ignore_ips": ["127.0.0.1", "::1", "你的办公出口 IP"]
```

4. 开启真实封禁：

```sh
sudo sed -i 's/"dry_run": true/"dry_run": false/' /etc/banhack233/config.json
sudo systemctl restart banhack233
```

5. 查看封禁：

```sh
banhack233 ban-list
```

## TCP 业务长连接高级项演示

默认不建议修改系统 TCP 参数。只有确认业务需要全局 TCP keepalive / conntrack 调整时，才执行：

```sh
sudo banhack233 keepalive -write -tcp
```

验证：

```sh
sysctl net.ipv4.tcp_keepalive_time \
  net.ipv4.tcp_keepalive_intvl \
  net.ipv4.tcp_keepalive_probes \
  net.netfilter.nf_conntrack_tcp_timeout_established
```

启用 `-tcp` 后期望：

```text
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.netfilter.nf_conntrack_tcp_timeout_established = 432000
```

不启用 `-tcp` 时，这些系统参数保持原样。

如果客户端仍 5 分钟掉线，检查：

- 业务服务是否有应用层 idle timeout。
- 业务协议是否有心跳包。
- 云厂商负载均衡是否有 idle timeout。
- 代理、网关、防火墙是否有自己的空闲连接回收。

## root 密码登录说明

本项目支持 root 密码登录，不强制禁止。

推荐最低基线：

```text
PermitRootLogin yes
PasswordAuthentication yes
PermitEmptyPasswords no
MaxAuthTries 3
LoginGraceTime 30s
TCPKeepAlive yes
ClientAliveInterval 60
ClientAliveCountMax 1440
```

同时建议：

- root 密码足够强。
- 开启飞书或邮箱通知。
- `dry_run=false` 前先观察。
- 办公出口 IP 写入 `ignore_ips`。

## 排障

### 安装慢

远端从 GitHub 下载可能慢。可手动下载 release 资产后复制到服务器：

```sh
chmod +x banhack233-linux-amd64
sudo mv banhack233-linux-amd64 /usr/local/bin/banhack233
sudo banhack233 install-autostart
```

### SSH 还是断

先确认系统配置：

```sh
sshd -T | egrep 'clientalive|tcpkeepalive'
```

再确认是否有：

- 云负载均衡 idle timeout。
- 业务网关 idle timeout。
- 应用层主动断开。

如果你明确启用了 `banhack233 keepalive -write -tcp`，再额外检查：

```sh
sysctl net.ipv4.tcp_keepalive_time net.netfilter.nf_conntrack_tcp_timeout_established
```

### 没有封禁

检查：

```sh
banhack233 status
banhack233 test
banhack233 ban-list
```

如果 `dry_run=true`，不会真封禁。

### 飞书没收到

检查：

```sh
banhack233 test
journalctl -u banhack233 -n 100 --no-pager
```

确认 webhook 地址正确，飞书机器人未被禁用。

## 开发

```sh
go test ./...
go vet ./...
sh build-all.sh
```

发布 tag：

```sh
git tag v0.1.8
git push origin v0.1.8
```

GitHub Actions 会自动构建 release 资产。
