# 部署、恢复与日常运维

以下主流程面向 Linux + systemd + SSH TCP 22。自定义 SSH 端口、macOS、Windows 请先读 [平台边界](design.md)。所有命令中的示例 IP 必须换成实际地址。

## 首次上线的验收顺序

1. 从 [README 顶部安装命令](../README.md) 安装，记录 `banhack233 version`，核对实际配置路径。
2. 加入公司出口和应急管理出口白名单；保留 `dry_run=true`、`malware.direct_kill=false`。
3. 确认 `log_paths` 对应文件确实存在且有新 SSH 日志。只有 journal 的系统不能直接按默认文本路径扫描。
4. `install-autostart -config ...` 后检查服务启动参数和日志。`start_at_end=true` 时，先启动再产生新的测试日志。
5. 用下节隔离回放验证匹配，再单独执行 `notify-test`，确认接收端收到。
6. 需要 SSH 基线或保活时先预览，确认 root/密码登录及当前管理用户设置，应用后从另一会话测试新登录。
7. 修改 `dry_run=false`、重启服务，再核对防火墙后端和新事件。不要通过连续输错真实办公账号来验证封禁。

```sh
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10 2001:db8::10
sudo banhack233 install-autostart -config /etc/banhack233/config.json
sudo systemctl status banhack233 --no-pager
sudo systemctl cat banhack233
sudo journalctl -u banhack233 -n 100 --no-pager
```

已有服务更新配置后，使用 `systemctl restart banhack233`；再次执行 `install-autostart` 不应代替明确重启。白名单只约束本程序，不能覆盖云防火墙、其他防爆破工具或 sshd 自身策略。

## 隔离日志回放，不触碰真实封禁

以下是完整的 shell 示例：新临时目录、独立状态、仅两个密码失败事件，关闭真实封禁、恶意程序扫描和外部通知。需要已安装 `banhack233`；无需 root。

```sh
umask 077
work_dir="$(mktemp -d)"
cat > "$work_dir/auth.log" <<'LOG'
Oct 10 12:00:01 host sshd[123]: Failed password for demo from 198.51.100.42 port 51001 ssh2
Oct 10 12:00:02 host sshd[123]: Failed password for demo from 198.51.100.42 port 51002 ssh2
LOG
cat > "$work_dir/config.json" <<JSON
{
  "dry_run": true,
  "start_at_end": false,
  "state_path": "$work_dir/state.json",
  "geoip": {"enabled": false},
  "logging": {"enabled": false},
  "malware": {"enabled": false, "direct_kill": false},
  "notifications": {"console": true, "audit": false, "batch": {"enabled": false}},
  "rules": [{
    "name": "replay-password",
    "log_paths": ["$work_dir/auth.log"],
    "patterns": ["Failed password for (?P<user>[^ ]+) from <HOST>"],
    "reset_patterns": ["Accepted password for (?P<user>[^ ]+) from <HOST>"],
    "max_attempts": 2, "find_time": "10m", "ban_time": "1m",
    "count_by_user": true, "action": "auto"
  }]
}
JSON
banhack233 test -config "$work_dir/config.json"
banhack233 test -config "$work_dir/config.json"
printf 'Replay files: %s\n' "$work_dir"
```

预期：首次有一次 dry-run 告警，第二次没有新增日志就不重复告警；状态文件保存偏移和冷却，不应新增真实防火墙规则。每个独立用例创建新目录，避免前一个用例的冷却影响结果。

扩展验收用例：阈值设 3 后输入“失败、失败、成功、失败”应不触发；将测试地址加到 `ignore_ips` 应不触发；替换成 IPv6 文档地址 `2001:db8::42` 验证日志匹配。单元测试还覆盖到期释放、多用户和坏日志，见 [源码测试](../internal/daemon/daemon_test.go)。

## 公司共享出口与误封恢复

公网服务无法按客户端网卡 MAC 区分 NAT 后面的电脑。按用户名计数可以隔离部分失败累计，但仍由源 IP 防火墙规则执行，不能保证同一出口其他用户不受影响。

从仍可用的管理会话或主机控制台操作：

```sh
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10
sudo banhack233 unban 203.0.113.10
sudo systemctl restart banhack233
sudo banhack233 ban-list
```

确认你加入的是服务器实际看到的出口地址，IPv4/IPv6 分别检查。生产模式会在下一轮释放状态中记录的白名单自动封禁；dry-run 不改现有防火墙，故恢复步骤保留显式 `unban`。

如果仍连不上，分别检查其他封禁工具、云规则、sshd 日志、监听地址和有效配置。`unban` 自动选择后端；若安装过新的防火墙工具导致选择变化，要核对原后端残留，不能以一次命令成功作为所有层已解除的证明。

## 升级前备份

安排短维护窗口，先停止守护进程，避免复制正在变化的状态。下面按 Linux 默认路径备份，不包含 SSH 或 sysctl 配置；如果本次也要修改这些文件，应分别备份。

```sh
backup_dir="/var/backups/banhack233/$(date +%Y-%m-%d_%H-%M-%S)"
sudo install -d -m 700 "$backup_dir"
sudo systemctl stop banhack233
sudo cp -p /etc/banhack233/config.json "$backup_dir/config.json"
sudo cp -p "$(command -v banhack233)" "$backup_dir/banhack233"
if sudo test -f /var/lib/banhack233/state.json; then
  sudo cp -p /var/lib/banhack233/state.json "$backup_dir/state.json"
fi
printf 'Backup: %s\n' "$backup_dir"
```

备份失败应先处理错误，不要继续替换程序。备份含通知凭据和来源地址，保持仅管理用户可读，不上传到 Issue。

## 指定版本安装与回滚

例如重新安装已发布的 `v0.1.22`，同时固定安装脚本版本和二进制版本：

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/v0.1.22/scripts/install.sh | sh -s -- v0.1.22
banhack233 version
sudo systemctl restart banhack233
sudo journalctl -u banhack233 -n 100 --no-pager
```

安装器保留已存在的配置，但首次创建配置时该版本脚本仍从 `main` 下载样例；严格可重复部署请自行保存版本对应配置。安装器不会自动校验发布资产的 SHA-256；需要时从该 Release 下载 `SHA256SUMS.txt` 与目标资产后先校验，再安装。

回滚使用刚才备份的二进制和配置；将 `backup_dir` 设置为已核实的那次备份路径，在同一维护窗口停止服务后恢复，最后重启并检查版本、日志和新登录。不要自动覆盖状态：版本兼容性和实际防火墙状态需一起核对；恢复旧状态不会自动恢复内核防火墙规则。

```sh
# backup_dir 必须指向已经核实的备份目录
sudo systemctl stop banhack233
sudo install -m 755 "$backup_dir/banhack233" /usr/local/bin/banhack233
sudo install -m 600 "$backup_dir/config.json" /etc/banhack233/config.json
sudo systemctl start banhack233
banhack233 version
```

## 暂停和退回观察模式

改 `dry_run=true` 并重启，会停止后续真实封禁，也停止本程序对已有真实封禁的自动释放；不会回滚已有规则。必要时逐一 `unban` 并核对后端。直接停止服务同样不清除规则，且到期解封也会停止。

卸载自启动用 `uninstall-autostart`，保留配置和报告便于恢复。SSH 托管块、保活 drop-in、显式 `-tcp` 写入的 sysctl 文件需要分别审阅和恢复；删除 sysctl 文件不等于即时恢复已应用的内核参数。不要清空整台主机的防火墙来代替定向处理。

## Windows 与 macOS 的启动路径

Windows 管理员 PowerShell 安装通常使用以下路径；非管理员安装需改成安装器输出的 LocalAppData 路径。示例只建立启动配置，不能代替平台封禁验收。

```powershell
$exe = Join-Path $env:ProgramFiles 'banhack233/banhack233.exe'
$cfg = Join-Path $env:ProgramData 'banhack233/config.json'
& $exe status -config $cfg
& $exe install-autostart -config $cfg
schtasks /Query /TN banhack233 /V /FO LIST
```

首次 Windows 配置应使用本机 `init-config` 生成或逐项校对，不能原样使用含 `/var/log/...` 的 Linux 样例。计划任务以 SYSTEM 运行，所用二进制和配置必须对该账号可访问。macOS 安装脚本写 `/etc/banhack233/config.json`，程序默认读 `/usr/local/etc/banhack233/config.json`；显式指定实际配置，并校对状态/日志路径和日志来源。

## 日常检查与事件留存

关注最近一轮成功扫描、连续 `scan error`、日志是否增长、预期后端规则、通知实收和磁盘剩余空间。应用日志默认 10 MB 轮转、旧文件保留 30 天；报告默认保留 50 份 `.txt`。手动扫描必须记录输出的报告路径；需要长期留存的报告复制到独立归档目录，避免被数量淘汰。

没有内置 Prometheus 端点、告警重试队列或性能 SLO。高日志量环境应测量实际扫描时长、进程 CPU/RSS、状态文件大小和误报率，再决定轮询周期与阈值。
