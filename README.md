# banhack233

傻瓜式主机防入侵工具。自动防 SSH 爆破、封禁攻击 IP、开机自启、定时巡检、飞书/邮箱通知。默认支持 SSH 密码登录和 root 登录。

## 一行安装

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.sh | sh
```

支持 Linux、macOS、Windows Git Bash/MSYS。Windows PowerShell 可用：

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/banhack233/main/scripts/install.ps1 | iex
```

## 常用命令

```sh
banhack233 status
banhack233 doctor
banhack233 ban-list
banhack233 secure-ssh
banhack233 install-autostart
```

默认 `dry_run=true`，先观察不真封禁。确认无误后改配置为 `dry_run=false`。

配置文件：

- Linux/macOS: `/etc/banhack233/config.json`
- Windows: `%ProgramData%\banhack233\config.json` 或 `%LOCALAPPDATA%\banhack233\config.json`

## 开发

```sh
go test ./...
sh build-all.sh
```
