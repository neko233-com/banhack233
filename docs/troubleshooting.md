# 排障与常见问题

先定位“日志输入 → 计数 → 防火墙 → 通知”中的哪一层出了问题。`status` 会执行巡检；只观察时先确认 `malware.direct_kill=false`。命令参数见 [命令参考](commands.md)，已知实现问题见 [工作原理](design.md)。

## 服务在运行，但没有封禁

| 检查 | 如何判断 | 下一步 |
| --- | --- | --- |
| 真正使用的配置 | `systemctl cat banhack233` 的 `ExecStart` 与预期一致吗 | 核对 `-config`，文件缺失会回退默认值 |
| 运行模式 | `dry_run=true` 或 `action=notify` 吗 | 这是观察/通知模式，不会写真实封禁 |
| 白名单 | 来源是否落在某个 `ignore_ips` 或 CIDR 内 | 可信来源被跳过属于预期 |
| 日志输入 | 文件存在、可读、最近确有新失败日志吗 | 默认 `start_at_end=true` 首次跳过旧日志；journal-only 需先接入文本日志 |
| 计数条件 | 用户拆分、成功重置、地区阈值是否改变了预期 | 用隔离回放验证；广州示例阈值为 100，不是全局 5 |
| 后端动作 | 服务日志有 `scan error` / `no nft or iptables found` 吗 | 修复权限/后端；有 nft 但运行失败不会切换 iptables |
| 端口与平台 | SSH 是否在 TCP 22；是否使用 Windows/macOS | 对照平台限制，规则存在不等于实际阻断 |

Linux 采集命令：

```sh
banhack233 version
sudo systemctl cat banhack233
sudo systemctl status banhack233 --no-pager
sudo journalctl -u banhack233 -n 100 --no-pager
sudo banhack233 ban-list
```

仅对主机实际使用的后端查询：`sudo nft list table inet banhack233`，或 `sudo iptables -S INPUT` / `sudo ip6tables -S INPUT`。iptables 的列表可能含其他系统规则，不要全部删除。

## 没输错五次，为什么已经触发

默认只计明确密码失败。跨文件重复转发、共享用户名、历史回放或自定义宽泛正则仍会增加计数；Windows 已保存事件 ID 并去重。用 `safe-ssh` 核对策略，再回放最小日志样本。

根据真实原始日志回放确认来源。若需求是“只统计密码错误”，替换规则的 `patterns`，使用 [密码专用片段](configuration.md)，同时保留成功重置和公司出口白名单。

## 白名单加了仍然连不上

白名单写入配置后需要重启。它不会创建网络层允许规则，也不会解除其他程序的封禁。检查服务器看到的出口 IP、IPv6、CIDR 是否正确；按 [误封恢复步骤](operations.md) 显式解封，再检查云防火墙和 sshd 自身限制。MAC 地址无法穿过公网路由直接用于这种客户端识别。

## 期限到了还没解封，或者重启后状态不一致

检查守护进程是否活跃、是否改成 dry-run、上一次释放是否报错、是否还有另一条规则或用户计数对应的有效封禁。系统时钟变化也会影响记录的时间。内核规则和状态文件不是同一份数据：停服务、重启主机、手工清规则或恢复旧状态都需要分别核对两层。

自定义动作不提供通用自动撤销。不要为了“重置计数”直接删除生产状态文件：它同时保存到期释放所需的信息。

## SSH 保活写了但仍断线

```sh
sudo sshd -t
sudo sshd -T
sudo sshd -T -C user=root,addr=203.0.113.10,host=client.example
```

用实际用户和来源替换最后一行，检查有效值中的 `clientaliveinterval`、`clientalivecountmax`、`passwordauthentication`、`permitrootlogin` 和 `port`。检查 Include/Match、服务 reload、新登录会话、客户端休眠和网络中断。24h 不是可用性保证；业务 TCP 长连接还受协议心跳和应用/代理超时约束。[OpenSSH 有效配置测试](https://man.openbsd.org/sshd.8)

`keepalive -write` 不会自动应用 TCP 全局参数；只有明确使用 `-tcp` 才会写它们。它也不代表可以覆盖应用主动断开的逻辑。

## 通知没有收到或延迟

逐个渠道验证：

```sh
banhack233 notify-test -channel feishu
banhack233 notify-test -channel telegram
banhack233 notify-test -channel discord
banhack233 notify-test -channel slack
banhack233 notify-test -channel webhook:custom-json
banhack233 notify-test -channel email
```

Feishu/Lark 核对 webhook 与签名 secret、系统时间；Discord/Slack 核对对应渠道地址与权限；通用 webhook 核对 `format`、headers 和接收端日志。邮箱核对 SMTP 授权方式，使用自定义服务器时同时填 host 和 port。不要在工单里粘贴凭据。

独立工作协程每秒检查批次，每渠道 10 秒超时；失败渠道从持久队列重试，最长 24 小时。已检查 Telegram/飞书业务错误。仍应核对实收、队列与日志，见 [通知排障](notifications.md)。

## 扫描结果与报告

“没有明显特征”不能证明主机无入侵。临时目录程序和挖矿工具可能合法；巡检始终只读，手动清理默认关闭且限定已知入侵程序名。先核对路径、报告和持久化证据。

手动 `malware-scan -name incident-review` 会写报告；命令输出的路径是依据。报告名精确到秒，同名同秒会在名字后增加序号，避免覆盖已有报告。报告目录中所有 `.txt` 都参与按修改时间保留 N 份的淘汰，长期证据请复制到独立归档。

## 最小问题报告

提交 [GitHub Issue](https://github.com/neko233-com/banhack233/issues/new) 时附以下内容，原始凭据和完整生产配置留在本机：

```text
版本/提交：
操作系统、架构、自启动方式：
SSH 实际监听端口、IPv4/IPv6：
命令（敏感值已替换）、退出码、发生时间/时区：
预期行为 / 实际行为：
实际 -config 路径及相关字段（已脱敏）：
最少几行可复现的认证日志（保持结构，IP/用户名用占位值）：
相关 scan error / 目标后端规则：
独立状态目录、dry_run=true 下能否复现：
```

webhook URL、签名 secret、SMTP 授权码、Authorization header 必须移除；IP、用户名、主机名和报告中的进程参数按需匿名化。不要上传整个 `state.json` 或真实 `config.json`。
