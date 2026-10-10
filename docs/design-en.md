# Architecture and platform boundaries

banhack233 combines fail2ban / sshguard-style defense, audits, SSH/optional TCP keepalive, notifications, autostart and release updates. It does not parse fail2ban configuration and is not a full commercial antivirus or EDR.

## From logs to actions

```text
Validate config → lock state writer
  → load state and release expired / newly whitelisted tracked bans
  → read incremental events per rule/source
  → preprocess successful logins: clear counters and set IP grace
  → password failure: canonical IP → whitelist/grace → username → threshold
  → SSH destination-port enforcement / dry-run / notify cooldown
  → application log and durable queue → independent delivery/retry
  → read-only audit → save state → next cycle
```

Defaults count only `Failed password`. Invalid-user, publickey, max-auth and disconnect companion messages do not add attempts. Success anywhere in the current read batch suppresses that IP's failures, followed by a default 10-minute grace period for this rule. Custom rules omitting `success_grace` have zero subsequent grace, while same-batch protection remains.

`count_by_user=true` groups by `rule|IP|username`. Different accounts behind one egress are separate, but shared accounts share a counter. Firewall enforcement remains IP-based. Public routing does not expose the client's NIC MAC to SSH, so it cannot distinguish office machines that way. Individual accounts/keys, SSH certificates or device-aware access networks can provide identity; this program does not implement device bans.

Office whitelisting is direct, but exempts everyone behind that egress. Success grace likewise trusts an IP with recent successful authentication. Reduce `success_grace` or use notify-only/manual response where appropriate. There is no six-connections-per-minute drop rule.

## Logs, state and expiry

- Text offsets belong to each rule/path. File identity detects replacement; truncation resets to zero. Partial lines wait, oversized records are discarded, and a cycle reads approximately 4 MiB. Unread tails of deleted rotated files cannot be recovered.
- Windows processes up to 1024 events per cycle in EventRecordID order, persisting/deduplicating the cursor. First `start_at_end=true` records the newest ID; detected channel clearing resets the cursor.
- Duplicate forwarding into different files has no global event identity. Failure windows use observation time rather than parsed historical timestamps. Keep initial EOF mode in production.
- A cross-process lock rejects another daemon/test sharing state. Use a dedicated queue path for each instance.
- Expiry requires the running daemon. Multiple active rule/user bans share an IP until all can be released.
- Failed removal is retried. Custom actions own reversal. `unban` changes the backend only; deleting state does not remove kernel rules.
- State and firewall persistence are separate. Reboot or external rule clearing does not currently trigger full reconstruction of all unexpired bans.
- Renamed/deleted rules do not immediately undo existing bans. Review old rules when changing `ssh_ports`; recorded ports support eventual cleanup.

## Platforms

| Capability | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Architectures | amd64 / arm64 | amd64 / arm64 | amd64 / arm64 |
| Autostart / daily update | systemd / timer | launchd / 86400 seconds | SYSTEM scheduled tasks |
| Logs | auth.log / secure / text | Configure actual authentication text source | OpenSSH/Operational or text |
| Firewall | nft, otherwise iptables/ip6tables | PF table with administrator-managed filter | netsh inbound `localport` |
| SSH ports | `ssh_ports`, default `[22]` | Configure PF explicitly | `ssh_ports`, default `[22]` |
| secure-ssh / keepalive | Supported | Unsupported | Unsupported |
| Malware scan | Lightweight helper | Reports unsupported | Reports unsupported |
| GeoIP | Optional local IPv4 database | Same | Same |

A failing installed nft backend does not silently fall back to another firewall. Fix its permissions/kernel support. PF refuses table changes if it cannot find a filter referencing `<banhack233>`; an administrator must still verify actual scope and effect. [Microsoft netsh local-port semantics](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/netsh-advfirewall)

Six builds, Linux/Windows/macOS unit tests and isolated Linux integration checks do not prove production traffic enforcement on every platform. Windows `autostart-status` still does not reliably report live task activity; inspect Task Scheduler. The updater separately checks live task state for restart health.

## Read-only diagnostics and cleanup

`status`, `doctor` and scheduled audits are read-only. Manual `malware-scan` writes a report. The only cleanup setting is `direct_kill`, default false; explicit cleanup is restricted to selected intrusion executable names. A temporary path, masscan, xmrig or an argument keyword alone never qualifies for termination. Such findings remain reviewable indicators, not proof of compromise.

Reports use `{name}.yyyy-MM-dd_HH-mm-ss.txt`; collisions suffix the name without overwriting. Default retention keeps 50 most recently modified reports, an LRU-like count policy rather than access-time retention.

`dry_run` controls firewall actions, not all commands. Manual cleanup, explicit configuration writes and updates have separate effects. `keepalive -write` changes SSH only; global TCP/conntrack tuning requires `-tcp`. Business TCP connections still depend on custom services/clients, socket keepalive, heartbeats and intermediate idle timeouts.

## Remaining work

- Native journald, historical event timestamps, unread rotated tails, full firewall reconstruction after restart.
- Protection health metrics and reproducible capacity benchmarks; no Prometheus endpoint or performance SLO currently.
- Managed macOS PF rules, real network acceptance across platforms and a device-identity integration design.
- Independent release signatures/attestations; current integrity checks trust SHA256SUMS from GitHub Releases.

See [notification semantics](notifications-en.md) and [update guarantees](updates-en.md). Remaining work is not presented as shipped capability.
