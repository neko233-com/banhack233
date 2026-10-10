# 命令参考与副作用

示例默认 Linux。`-config` 是子命令参数，放在子命令后；`whitelist` 的选项要放在 IP 参数前。不是所有命令都支持 `-config`。

```sh
banhack233 status -config /etc/banhack233/config.json
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10
```

示例 IP 为文档占位地址，执行白名单操作前替换成真实可信出口。

## 查看与诊断

| 命令 | 参数 | 实际行为 |
| --- | --- | --- |
| `help` / `-h` / `--help` | 无 | 显示帮助 |
| `version` | 无 | 显示版本、提交和构建时间 |
| `status` | `-config` | 配置摘要、自启动状态，并执行一次系统巡检 |
| `doctor` / `audit` | `-config` | 执行系统巡检，输出建议；不自动应用 SSH 基线 |
| `ban-list` | 无 | 查询当前自动选择后端的规则；不是状态文件的到期时间列表 |
| `autostart-status` | 无 | 查询自启动配置；Windows 当前不判断任务是否正在运行 |
| `whitelist` | `-config`，无 IP 参数 | 显示白名单 |

**`status` / `doctor` 不是在任何配置下都只读。** Linux 巡检会调用恶意程序扫描；`malware.enabled=true` 且 `direct_kill=true` 时可结束可疑进程。仅观察时保持 `direct_kill=false`。`dry_run` 只控制防火墙动作。

## 配置与守护进程

| 命令 | 参数 | 实际行为 |
| --- | --- | --- |
| `init-config` | `-config`、`-force` | 写本机平台默认配置；已有文件默认拒绝覆盖，`-force` 才覆盖 |
| `run` | `-config` | 前台持续运行，写状态和日志；按配置封禁、巡检、通知 |
| `test` | `-config` | 执行一轮与守护进程相同的扫描；会写状态，生产配置下可真实封禁或解封 |
| `enable-production` | `-config` | 写生产配置预设，不重启服务；具体覆盖字段见 [配置参考](configuration.md) |
| `whitelist` | `-config`、一个或多个 IP/CIDR | 永久追加到配置；重启守护进程后生效，不会创建系统防火墙 ACCEPT 规则 |
| `unban <ip>` | 仅一个 IP | 使用当前自动选择的后端解除封禁；不修改白名单或状态文件 |

没有 `whitelist remove` 子命令。移除白名单时手工编辑 `ignore_ips` 并重启。`unban` 是临时防火墙操作；希望持续免封，应添加白名单。

## SSH 与 TCP 保活

仅 Linux 支持以下两项；预览时不写文件。

| 命令 | 参数 | 写入范围 |
| --- | --- | --- |
| `secure-ssh` | `-config`、`-write`、`-force` | `-write` 更新 `/etc/ssh/sshd_config` 托管块，备份旧文件并验证；要求 `allowed_users` 或 `-force` |
| `keepalive` | `-write`、`-ssh-hours`（默认 24）、`-tcp` | 默认预览 SSH drop-in；`-write` 写 SSH 配置；显式追加 `-tcp` 才写全局 TCP/conntrack 参数 |

```sh
sudo banhack233 secure-ssh -config /etc/banhack233/config.json
sudo banhack233 keepalive -ssh-hours 24
```

审阅预览、保留可恢复登录通道后再使用 `-write`。`keepalive` 预览也包含 `PasswordAuthentication yes`、`PermitRootLogin yes` 等登录策略，并非只改两个探测字段。`-force` 只跳过 `secure-ssh` 的用户列表要求，不代表能强制覆盖 OpenSSH 的其他配置优先级。

写入后用 `sshd -t` 检查语法、`sshd -T` 查看实际值，并验证新连接。程序会尝试 reload `ssh` / `sshd`，但目前不传播 reload 的错误；看到“applied”仍需检查服务结果。[OpenSSH 测试模式](https://man.openbsd.org/sshd.8)

## 扫描和通知

| 命令 | 参数 | 实际行为 |
| --- | --- | --- |
| `malware-scan` | `-config`、`-name`（默认 `malware-scan`）、`-kill` | 手动扫描并输出报告路径；`-kill` 或配置 `direct_kill=true` 可结束可疑进程 |
| `notify-test` | `-config`、`-channel`、`-message` | 向已配置渠道真实发送测试消息；不执行封禁 |

```sh
sudo banhack233 malware-scan -name incident-review
banhack233 notify-test -channel feishu,email -message 'banhack233 channel check'
banhack233 notify-test -channel webhook:custom-json
```

渠道选择支持 `console,feishu,discord,slack,email,webhook` 和 `webhook:<name>`。手动扫描也会遵守 `direct_kill=true`，省略 `-kill` 不会覆盖它。非 Linux 扫描会报告不支持，不应解释为主机安全。

## 自启动

```sh
sudo banhack233 install-autostart -config /etc/banhack233/config.json
banhack233 autostart-status
sudo banhack233 uninstall-autostart
```

安装会写系统自启动配置并尝试启动：Linux systemd、macOS launchd、Windows 计划任务。管理员/root 权限用于系统配置和防火墙操作。Windows 示例见 [运维手册](operations.md)。

`uninstall-autostart` 停止并移除自启动；不卸载二进制、不删除配置/报告、不清空防火墙封禁，也不撤销 SSH/sysctl 改动。

## 退出与验证

CLI 返回错误时退出码为 1。守护进程遇到单轮错误会记录 `scan error:` 并继续循环，因此“进程活着”不代表扫描成功。`test` 的返回结果也不能代替通知收件验证：结束时刷新批次的错误目前没有反馈到其返回值。

检查 JSON 语法可使用 `jq empty /etc/banhack233/config.json`；这不检查字段名、正则、文件权限或配置是否适合生产。完整验证使用 [隔离日志回放](operations.md)，不要把生产 `test` 当成无副作用的 lint。

源码依据：[命令入口](../cmd/banhack233/main.go)、[自启动](../internal/autostart/autostart.go)、[系统巡检](../internal/audit/audit.go)。
