# Architecture, platform limits, and next steps

This page describes the current implementation, not new feature commitments. banhack233 combines fail2ban / sshguard-style defense, host audits, keepalive, notifications, and autostart. Similar detection ideas do not imply full compatibility with those projects.

## From logs to bans

```text
Startup: load configuration once
  → load state each cycle
  → release expired / newly whitelisted tracked automatic bans in production
  → read new text lines / query Windows events
  → successful-login match: clear this rule's failure counters for the IP
  → failure match: validate IP → whitelist → regional threshold → accumulate
  → threshold: firewall action / dry-run / notify cooldown
  → application logging and notification delivery or buffering
  → audit if due
  → persist state and wait for next cycle
```

Success clears hit counters, not existing bans or cooldowns; it cannot undo actions already triggered by earlier lines in the same cycle. The first matching failure expression supplies one result per line. Multiple log lines from one authentication exchange can still count as multiple events.

With `count_by_user=true` and a captured user, keys are `rule|IP|user`; otherwise `rule|IP`. Two people sharing a username still share a counter. Enforcement has only a source IP, not a device or MAC identity.

## Time, state, and expiry

- Failure windows use observation time rather than timestamps parsed from log text. Historical replay is treated as newly observed activity; default `start_at_end=true` avoids initial full text-log replay.
- The daemon records `ban_time` and releases it in later cycles; nft elements do not receive their own expiry timer. Stopping the daemon stops automatic release.
- An IP shared by multiple active rule/user bans is retained until the associated real bans can all be released.
- Failed removal retains state for retry. Custom actions own reversal.
- Persisted state is not persisted firewall enforcement. After reboot or external rule removal, current code does not automatically reconstruct every unexpired ban. Inspect both layers.
- `unban` changes firewall contents, not counters/state. Deleting state does not remove firewall entries.
- Renaming/removing a rule does not immediately undo its previous bans; review backend contents and expiry together.

## Text-log ingestion limits

Logs are tracked by path and byte offset, without inode/file-identity tracking. A smaller file restarts at zero, but a replacement file already larger than the old offset can lose its beginning. This is not a complete rotation/delivery guarantee.

Rules sharing a file share its offset: the first reader can consume lines needed by later rules. Prefer one rule containing the relevant patterns for a source. Duplicate forwarding into separate files can also duplicate counts.

There is no native Linux journald reader. Windows `eventlog:` is a separate implementation, not a general log URI scheme. Patterns use Go regexp, without PCRE lookarounds/backreferences or fail2ban jail/filter loading.

## Platform matrix

| Capability | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Release binaries | amd64 / arm64 | amd64 / arm64 | amd64 / arm64 |
| Autostart | systemd | launchd | SYSTEM scheduled task |
| Inputs | Text logs | Text logs; actual source must be configured | `OpenSSH/Operational` events or text files |
| Automatic firewall | nft, or iptables/ip6tables if nft is absent | PF table operations; filtering rules required | netsh rules; port-direction issue below |
| `secure-ssh` / `keepalive` | Supported | Unsupported | Unsupported |
| Malware scan | Lightweight process/persistence checks | Reports unsupported | Reports unsupported |
| GeoIP | Local IPv4 database | Same, with usable path | Same, with usable path |

Six successful builds do not prove six-platform blocking acceptance. Linux/Windows CI unit tests do not replace target-host firewall and log-input verification.

## Known limits and deployment impact

| Current implementation | Impact | Current handling |
| --- | --- | --- |
| Linux auto rules use `tcp dport 22` | Custom sshd ports are not followed | Use notify-only or a separately verified custom action; no `ssh_port` field exists |
| nft runtime failure does not fall back to iptables | Permission/kernel issues become scan errors | Inspect actual nft errors and repair the selected backend |
| Windows netsh uses `remoteport=22` | Inbound matching targets the remote source port, not the usual local SSH destination port, and generally misses intended traffic | Do not rely on this automatic rule until corrected; separately manage verified firewall policy |
| Windows queries the latest 80 events every cycle, without cursor/deduplication | Old failures can be counted repeatedly; event ordering differs from text replay | Validate in observation/notify mode; cursor/deduplication is outstanding |
| macOS only operates the `banhack233` PF table | No complete matching filter rule is installed; table membership does not prove blocking | Configure and validate PF filtering and authentication-log sources separately |
| Windows startup status does not inspect live task execution | `active=false` does not prove the task is stopped | Check `schtasks /Query /TN banhack233 /V /FO LIST` |
| Some macOS / non-admin Windows installer paths differ from defaults | Missing files silently use defaults | Pass `-config`, confirm existence, and review platform-specific fields |
| SSH audit simply reads the main file | Include/Match/effective precedence is not fully evaluated | Check `sshd -T` with the relevant connection context |
| Serial notifications, in-memory batches, no durable retry | One channel failure affects later channels; pending events can be lost | Test channels separately and retain logs; this is not a reliable message bus |

The Windows port-direction finding follows from [current action code](../internal/ban/action.go) and [Microsoft's netsh parameter reference](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/netsh-advfirewall). It is a source-confirmed implementation issue, not a fix delivered by this documentation change.

## Keepalive and protection boundaries

“24h SSH keepalive” describes a configured probe tolerance window, not a guarantee through network loss, host sleep, service restarts, or proxy timeouts. Password/root login remains supported; compare the command's policy with host requirements. Business TCP long connections also depend on socket keepalive, protocol heartbeats, and application timeouts in custom TCP services/clients. Global tuning requires explicit `-tcp`.

Whitelisting exempts all attempts from that source, including unwanted attempts behind the same office egress. SSH keys/certificates help manage identity, but this program does not enforce device bans by key fingerprint. It has no default connection-frequency drop rule and does not provide traffic scrubbing, automatic OS patching, or business-port policy management.

## Useful future work

These are candidates, **not implemented features**. Each needs reproducible acceptance criteria.

| Priority | Direction | Acceptance criteria |
| --- | --- | --- |
| High | Windows local destination port, event cursor/deduplication | Block the SSH target with normal client source ports; count each event once across restarts |
| High | Configurable SSH ports and platform firewall rules | IPv4/IPv6, custom ports, expiry, and actual connection checks |
| High | Strict config validation and isolated command side effects | Explicit errors for typos/missing files/invalid regexes; read-only diagnostics never remediate |
| Medium | Rotation, shared-file rules, state/firewall reconciliation | No missed file replacements; independent rule consumption; verifiable enforcement after restart |
| Medium | Notification deadlines, independent failures, durable retry | Failed webhook cannot block scanning/other channels; pending events recover |
| Medium | Consistent install paths and asset verification | Correct defaults on all platforms; failed downloads preserve working binaries |
| Later | Capacity tests, metrics, device-identity design | Reproducible environment, log volume, latency/memory/false-positive data, dependencies and limits |

Use the [issue evidence template](troubleshooting-en.md). Source: [scanner](../internal/daemon/daemon.go), [reconciliation](../internal/daemon/reconcile.go), [Windows events](../internal/daemon/windows_events.go), [backend](../internal/ban/action.go).
