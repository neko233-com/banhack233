# banhack233

## One-Line Install

Linux / macOS / Windows Git Bash or MSYS:

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.ps1 | iex
```

Check status after install:

```sh
banhack233 status
```

Default mode is `dry_run=true`: alert only, no real firewall ban. Switch to `dry_run=false` after review.

Chinese docs: [README.md](README.md)

## What It Is

`banhack233` is a simple host intrusion defense tool for Linux, macOS, and Windows.

It is not a line-by-line clone of fail2ban. It is a bundle of common host security capabilities:

- fail2ban-style log scan and IP ban.
- sshguard-style SSH brute-force defense.
- Host security audit.
- SSH 24-hour keepalive.
- TCP business long-connection keepalive, avoiding early system/NAT/conntrack cleanup.
- Notifications through console, Feishu/Lark, Discord, Slack, generic webhook, and email.
- Autostart through systemd, launchd, or Windows scheduled tasks.

Password SSH login and root password login are supported use cases. The default baseline keeps them enabled while reducing risk with low retry count, short login grace time, no empty passwords, alerts, whitelist, and automated bans.

## Supported Platforms

| Platform | Arch | Installer | Autostart |
| --- | --- | --- | --- |
| Linux | amd64 / arm64 | `install.sh` | systemd |
| macOS | amd64 / arm64 | `install.sh` | launchd |
| Windows | amd64 / arm64 | `install.ps1` or Git Bash `install.sh` | schtasks |

Default config paths:

```text
Linux:   /etc/banhack233/config.json
macOS:   /usr/local/etc/banhack233/config.json
Windows: %ProgramData%\banhack233\config.json
```

Default state paths:

```text
Linux:   /var/lib/banhack233/state.json
macOS:   /usr/local/var/banhack233/state.json
Windows: %ProgramData%\banhack233\state.json
```

## Quick Start

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
banhack233 status
banhack233 doctor
sudo banhack233 keepalive -write
sudo banhack233 install-autostart
```

Enable real bans after observing dry-run output:

```sh
sudo sed -i 's/"dry_run": true/"dry_run": false/' /etc/banhack233/config.json
sudo systemctl restart banhack233
```

## Main Commands

```sh
banhack233 status
banhack233 doctor
banhack233 test
banhack233 ban-list
sudo banhack233 unban 203.0.113.10
banhack233 secure-ssh
sudo banhack233 secure-ssh -write -force
sudo banhack233 keepalive -write
sudo banhack233 install-autostart
```

`doctor` checks SSH policy, root login policy, empty password setting, retry limit, login grace time, firewall backend, reboot requirement, dry-run status, and notification channels.

It does not treat Redis, MySQL, custom TCP services, or other business ports as default security issues. Business exposure is a business decision.

## SSH Baseline

Preview:

```sh
banhack233 secure-ssh
```

Apply:

```sh
sudo banhack233 secure-ssh -write -force
```

Default policy:

```text
PasswordAuthentication yes
PermitRootLogin yes
PermitEmptyPasswords no
MaxAuthTries 3
LoginGraceTime 30s
```

Keep an existing SSH session open before applying SSH changes.

## SSH 24h Keepalive + TCP Long Connections

```sh
sudo banhack233 keepalive -write
```

Writes:

```text
/etc/ssh/sshd_config.d/99-banhack233-keepalive.conf
/etc/sysctl.d/99-banhack233-keepalive.conf
```

SSH settings:

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

TCP settings:

```text
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.netfilter.nf_conntrack_tcp_timeout_established = 432000
```

Meaning:

- SSH sends keepalive every 60 seconds.
- SSH allows 1440 missed keepalive checks, about 24 hours.
- TCP keepalive starts after 60 seconds.
- conntrack established timeout is 5 days.

This helps prevent system, NAT, or conntrack layers from cleaning up idle business long connections too early. If an application has its own 5-minute idle timeout, fix the application heartbeat or timeout too.

## Config Example

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

Important fields:

| Field | Meaning |
| --- | --- |
| `dry_run` | Alert only when `true`; no real firewall ban |
| `start_at_end` | Monitor only new logs on first run |
| `ignore_ips` | Never-ban whitelist |
| `max_attempts` | Failure threshold |
| `find_time` | Detection window |
| `ban_time` | Ban duration recorded in state |
| `action` | `auto` or custom command |

## Notifications

Supported channels:

| Channel | Field | Payload |
| --- | --- | --- |
| Console | `console` | stdout / systemd journal |
| Feishu / Lark | `feishu` | `msg_type=text` |
| Discord | `discord` | `content` |
| Slack | `slack` | `text` |
| Generic webhook | `webhooks[]` | `text`, `json`, `discord`, `slack`, `feishu` |
| Email | `email` | SMTP |

### Feishu / Lark

```json
{
  "notifications": {
    "feishu": {
      "enabled": true,
      "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
    }
  }
}
```

### Discord

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

### Slack

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

### Generic Webhook

Use this for internal alert gateways, security platforms, or OpenClaw-style notification aggregators.

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

Formats:

| Format | Payload |
| --- | --- |
| `text` | Plain text |
| `json` | `{"text":"..."}` |
| `discord` | `{"content":"..."}` |
| `slack` | `{"text":"..."}` |
| `feishu` / `lark` | `{"msg_type":"text","content":{"text":"..."}}` |

### Email

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

SMTP auto-detection:

| Mail Domain | SMTP |
| --- | --- |
| `@qq.com` | `smtp.qq.com:465` |
| `@163.com` | `smtp.163.com:465` |
| `@126.com` | `smtp.126.com:465` |
| `@gmail.com` | `smtp.gmail.com:587` |
| `@outlook.com` | `smtp-mail.outlook.com:587` |
| `@hotmail.com` | `smtp-mail.outlook.com:587` |
| `@live.com` | `smtp-mail.outlook.com:587` |

QQ and 163 usually require SMTP authorization codes, not login passwords.

## Brute-Force Defense Flow

1. Install in dry-run.
2. Enable autostart.
3. Watch logs and alerts.
4. Add office IPs to `ignore_ips`.
5. Switch `dry_run=false`.
6. Restart service.
7. Check `ban-list`.

```sh
banhack233 status
journalctl -u banhack233 -n 100 --no-pager
banhack233 ban-list
```

## TCP Long-Connection Verification

```sh
sshd -T | egrep 'clientalive|tcpkeepalive|permitrootlogin|passwordauthentication'
sysctl net.ipv4.tcp_keepalive_time \
  net.ipv4.tcp_keepalive_intvl \
  net.ipv4.tcp_keepalive_probes \
  net.netfilter.nf_conntrack_tcp_timeout_established
```

Expected:

```text
clientaliveinterval 60
clientalivecountmax 1440
permitrootlogin yes
passwordauthentication yes
tcpkeepalive yes
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.netfilter.nf_conntrack_tcp_timeout_established = 432000
```

If clients still disconnect after a few minutes, check application idle timeout, protocol heartbeat, cloud load balancer idle timeout, proxy timeout, gateway timeout, and firewall connection cleanup.

## Troubleshooting

### Install Is Slow

GitHub downloads can be slow from some networks. Download a release asset manually and copy it to the server:

```sh
chmod +x banhack233-linux-amd64
sudo mv banhack233-linux-amd64 /usr/local/bin/banhack233
sudo banhack233 install-autostart
```

### No Real Bans

```sh
banhack233 status
banhack233 test
banhack233 ban-list
```

If `dry_run=true`, bans are not applied.

### No Notification

Check webhook URL, email SMTP authorization code, outbound network, and service logs:

```sh
journalctl -u banhack233 -n 100 --no-pager
```

## Development

```sh
go test ./...
go vet ./...
```

Build all release binaries:

```sh
./build-all.sh
```

GitHub Actions run CI and release builds.
