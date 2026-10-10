# 正式版本自动更新与回滚

从 v0.2.0 开始支持每日检查、校验后安装、重启托管服务及启动失败回滚。更新只下载本项目 GitHub 正式 Release，不执行 `main` 分支脚本，不建立 SSH 连接，也不触发 GitHub Actions。

## 首次从旧版升级

旧二进制没有 updater 时，先使用 [安装命令](../README.md) 升级到 v0.2.0 或更高版本。安装器会验证 `SHA256SUMS.txt`，保留已有配置；旧规则不会自动变成新默认值。

```sh
sudo banhack233 safe-ssh -config /etc/banhack233/config.json
sudo banhack233 safe-ssh -write -config /etc/banhack233/config.json
sudo banhack233 config-check -config /etc/banhack233/config.json
sudo systemctl restart banhack233
```

`safe-ssh` 只迁移名为 `ssh-auth-failure` 的规则：密码专用、按用户计数、成功清零、10 分钟宽限；保留阈值、白名单、日志路径和其他配置，保存 `.before-safe-ssh` 备份。自定义规则单独检查。将公司出口和应急管理地址加入白名单后再切生产。

## 开启每日更新

先安装并验收托管服务。每日更新安装时要求服务正在运行，避免把人为停止的服务擅自启动。

```sh
sudo banhack233 install-autostart -config /etc/banhack233/config.json
sudo banhack233 auto-update -enable -config /etc/banhack233/config.json
banhack233 auto-update -status
```

| 平台 | 调度 | 查看结果 |
| --- | --- | --- |
| Linux | systemd timer 每日，最多随机延迟 1 小时；支持错过后补检查 | `journalctl -u banhack233-update.service` |
| macOS | launchd 每 86400 秒；不是精确的午夜时刻 | `/var/log/banhack233-update.log` |
| Windows | SYSTEM 每日 03:15，最多随机延迟 1 小时；错过后补检查 | 二进制旁的 `.update.log`，5 MB 后保留一份 `.old`；手动启动故障见 `.update-launcher.log` |

macOS 使用实际的 `-config` 路径。Windows 在管理员 PowerShell 中用完整程序路径执行相同 `auto-update -enable` 命令；计划任务通过临时 helper 避免正在运行的 EXE 无法替换。不要在用户可写目录安装 SYSTEM 执行的二进制或更新脚本。

## 手动检查、安装与关闭

```sh
banhack233 update -config /etc/banhack233/config.json
sudo banhack233 update -apply -restart -config /etc/banhack233/config.json
sudo banhack233 auto-update -disable
```

不带 `-apply` 只检查。`-restart` 重启的是 banhack233，不是 sshd，不改 SSH/TCP 保活设置。省略 `-restart` 只替换磁盘二进制，已有进程继续运行旧版，需自己安排重启。Windows 手动应用会启动隐藏 helper，CLI 返回“已启动”不代表完成，应查看更新日志和 `version`。

## 安装保障

1. 只接受已发布、非草稿、非预发布的 `vMAJOR.MINOR.PATCH`，拒绝降级；`dev` 构建不自动升级。
2. 精确选择本机 OS/架构资产及同 Release 的 SHA256SUMS；下载有超时/大小限制，HTTPS 校验开启。
3. 校验 SHA256、执行新二进制 `version` 与 `config-check`；失败时不替换旧程序。
4. 同目录暂存，保存 `.previous` 和 `.update.json`，进程锁防止多个安装器同时执行。
5. 重启后连续检查三次服务活跃状态；失败尝试恢复旧二进制并启动，任务返回错误。

配置、白名单、通知凭据、状态和报告不被 updater 改写。新版本若不接受旧配置会拒绝安装，需要管理员先完成迁移。SHA256 校验提供资产完整性，不等同于独立发布签名；信任边界仍包括 GitHub 仓库、发布权限和本机管理员。[GitHub latest Release 定义](https://docs.github.com/en/rest/releases/releases#get-the-latest-release)

启动活跃检查不等于业务防护验收；仍需检查近期扫描日志、真实防火墙端口和通知收件。主机断电/磁盘故障等不能保证自动恢复，保留独立备份和控制台入口。

GitHub API 返回 403/429 时检查出口网络和限额。可给 updater 的运行环境配置 `BANHACK233_GITHUB_TOKEN` 提高 API 配额；它只发送给 `api.github.com`，不会转发到资产下载地址或写入更新日志。更新失败保留当前版本，下一日再次检查；系统任务日志是排障依据。

## 回滚

```sh
sudo banhack233 auto-update -disable
sudo banhack233 update -rollback -restart -config /etc/banhack233/config.json
banhack233 version
sudo journalctl -u banhack233 -n 100 --no-pager
```

回滚验证 `.previous` 与记录的 SHA256，仅恢复二进制；不恢复旧状态或覆盖当前白名单。回退旧版后，较新的配置字段可能不兼容，需审阅版本差异。关闭每日更新可防止次日再次升到刚回退的版本。安装脚本的 `.installer-backup` 与 updater 的 `.previous` 是两种备份；首次安装脚本升级后应按 [手工恢复流程](operations.md) 使用前者。
