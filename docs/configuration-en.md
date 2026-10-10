# Configuration reference

This page describes current defaults, loading semantics, and limits. See the [complete example](../configs/config.json.example), [operations guide](operations-en.md), and [command reference](commands-en.md).

## Loading and applying configuration

- Configuration is JSON: no comments, trailing commas, or environment-variable expansion. Durations are strings such as `"30s"`, `"10m"`, and `"24h"`; `"1d"` is invalid.
- The program loads defaults, then overlays the file. Omitted top-level fields retain defaults. An explicit `rules` array replaces the default rules; individual array entries do not inherit the default SSH template.
- A custom rule without `count_by_user` therefore uses `false`; without `reset_patterns`, it has no successful-login reset. A rule fragment is not a complete rule.
- A missing config file returns defaults rather than an error. Confirm the file exists and inspect the service's actual `-config` argument.
- Unknown JSON fields are currently ignored. A typo such as `dryrun` does not set `dry_run`. There is no dedicated strict `config-check` command.
- The daemon reads configuration at startup, with no hot reload. Restart it after changes and use the same config path for administrative commands.
- `init-config` generates platform-specific defaults. The repository example primarily targets Linux; some installer paths differ from program defaults. See [platform limits](design-en.md).

## Top-level fields

| Field | Default | Behavior |
| --- | --- | --- |
| `interval` | `"30s"` | Polling interval; one cycle runs immediately at startup. Nonpositive values restore the default |
| `audit_interval` | `"1h"` | Audit eligibility checked during scan cycles, not by an independent timer |
| `state_path` | Platform state directory + `state.json` | Log offsets, hit timestamps, bans, backends, cooldowns, and last audit |
| `dry_run` | `true` | Neither applies nor releases real firewall bans; still writes state/logs and sends notifications. Not a global read-only switch |
| `start_at_end` | `true` | First encounter with a text-log path starts at EOF; existing offsets are reused. Does not apply to Windows events |
| `ignore_ips` | `["127.0.0.1", "::1"]` | IPv4, IPv6, or CIDR, not hostnames. An explicit array replaces this list |
| `rules` | One SSH authentication rule | `[]` stops rule scanning, but does not disable audits or immediately remove existing firewall entries |

Do not run two daemons, or a daemon and `test`, against the same state file. Atomic file replacement does not provide a cross-process lock.

## Rule fields

| Field | Default SSH rule / normalized fallback | Behavior |
| --- | --- | --- |
| `name` | `ssh-auth-failure`; empty becomes `rule` | Part of state identity; use unique names without `|` |
| `log_paths` | Platform authentication logs | Linux chooses `auth.log` if present, otherwise `secure`; the example lists both. Missing files are skipped |
| `patterns` | Six authentication-failure patterns | Go regular expressions; prefer one `<HOST>` or named `(?P<ip>...)`, optionally `(?P<user>...)` |
| `reset_patterns` | `Accepted ... from <HOST>` | Processed first; clears all username hit counters for that IP within this rule, without removing existing bans |
| `max_attempts` | `5` | Triggers at the threshold; counts matching log events, not exactly typed passwords |
| `find_time` | `"10m"` | Window uses observation time, not historical timestamps parsed from log text |
| `ban_time` | `"1h"` | Ban expiry, or notification cooldown per counting key in dry-run / notify mode |
| `action` | `"auto"` | Automatic backend selection; `"notify"` only notifies; other strings are executable names invoked with the IP |
| `count_by_user` | `true` in the default rule | Rule+IP+username if captured, otherwise rule+IP. Enforcement remains IP-based |
| `region_rules.max_attempts` | `广州: 100`, `Guangzhou: 100` | Regional threshold overrides; lookup failure or no match uses the base threshold |

Nonpositive attempts or durations restore defaults; they do not disable bans. Empty region names and nonpositive regional thresholds are removed.

`action` is not a backend enum. Setting `"nft"` attempts to execute `nft <IP>` rather than selecting the automatic nft implementation. Custom actions do not run through a shell and cannot embed arguments or pipelines in the value; they own their cleanup and lifetime. There is no `ssh_port` field: Linux automatic rules target destination TCP 22.

Region matching ignores case and supports partial country/region/city names. Overlapping matching keys have no defined priority; avoid overlapping keys with different thresholds. GeoIP does not replace a trusted-address whitelist. The current database reader is IPv4-only; IPv6 blocking does not require GeoIP.

## Password-only detection

Replace the existing SSH rule's `patterns`, retaining all its other fields, especially `reset_patterns`, thresholds, and `count_by_user`:

```json
{
  "patterns": ["Failed password for(?: invalid user)? (?P<user>\\S+) from <HOST>"]
}
```

This is a rule fragment, not a full configuration. It does not count new connections per minute. sshd's `MaxAuthTries` limits authentication attempts within one connection. Shared office addresses still need `ignore_ips`; see [operations](operations-en.md).

## SSH baseline

Fields below belong to `hardening.ssh`. They affect `secure-ssh`; starting the daemon does not automatically rewrite sshd configuration.

| Field | Default | Behavior |
| --- | --- | --- |
| `enabled` | `true` | Enables baseline preview/application |
| `password_authentication` | `true` | Password authentication supported |
| `permit_root_login` | `"yes"` | Root login supported; an empty string renders as `no` |
| `allowed_users` | `[]` | Nonempty lists generate `AllowUsers`; writing requires a list or explicit `-force` |
| `max_auth_tries` | `3` | Per-connection authentication attempts |
| `login_grace_time` | `"20s"` | Authentication grace period |
| `client_alive_interval` | `300` | Server probe interval in seconds |
| `client_alive_count_max` | `2` | Unanswered probes |
| `disable_empty_passwords` | `true` | Writes `PermitEmptyPasswords no` |
| `disable_challenge_response` | `true` | Disables keyboard-interactive / challenge-response; adjust if those methods are required |

`keepalive` is independent: it does not read this JSON and has no `-config` option. Its preview includes password/root-login settings and keepalive values, written to a different location from `secure-ssh`. Compare previews and verify effective sshd configuration after applying either command.

## Logging, GeoIP, and malware scanning

| Field | Default | Behavior |
| --- | --- | --- |
| `logging.enabled` | `true` | Application log output |
| `logging.path` | State directory + `banhack233.log` | Bans, audits, errors; not an SSH authentication-log input |
| `logging.max_size_mb` | `10` | Rotation size per file |
| `logging.max_age_days` | `30` | Retention age for rotated logs |
| `geoip.enabled` | `true` | Regional thresholds and notification enrichment |
| `geoip.db_path` | State directory + `ip2region_v4.xdb` | Missing database removes location detail, not base-threshold detection |
| `malware.enabled` | `true` | Includes scanning in Linux audits; manual `malware-scan` still scans |
| `malware.direct_kill` | `false` | Only cleanup configuration switch; can terminate suspicious processes independently of `dry_run` |
| `malware.report_dir` | State directory + `reports` | Manual reports; do not mix unrelated `.txt` files here |
| `malware.report_keep` | `50` | After writing a report, retain the newest N `.txt` files by modification time; nonpositive values restore 50 |

This is a lightweight Linux host antivirus / intrusion-triage helper, not a full commercial antivirus or EDR. Manual reports use `{name}.yyyy-MM-dd_HH-mm-ss.txt`. The LRU-like count policy keeps recently modified files; reading does not refresh retention. Automatic audits write application logs, not a separate manual report archive.

## Notifications

All fields below belong to `notifications`. Channel-specific examples remain in the [README](../README-EN.md).

| Field | Default | Behavior |
| --- | --- | --- |
| `console` | `true` | Notification output to stdout |
| `audit` | `false` | Sends audit findings; does not enable/disable audit execution |
| `batch.enabled` | `true` | Batches ban events; `action=notify` and audits send directly |
| `batch.interval` | `"60s"` | Eligibility checked during scan cycles, not an exact independent timer |
| `batch.max_items` | `20` | Sends when pending count reaches the limit |
| `feishu` / `discord` / `slack` | `enabled=false` | `enabled`, `url`, `location_language`; Feishu/Lark can use a signing `secret` |
| `webhooks[]` | Empty | `name`, `enabled`, `url`, `format`, `headers`, `secret`, `location_language` |
| `webhooks[].format` | Empty means plain text | `text`, `json`, `feishu`, `lark`, `discord`, `slack` |
| `email.enabled` | `false` | SMTP delivery |
| `email.from` / `to` | Empty | Sender; comma-separated recipients |
| `email.password` | Empty | SMTP authorization code or applicable credential |
| `email.smtp_host` / `smtp_port` | Empty / `0` | Set both to override presets; if either is absent, inference replaces both |
| `email.location_language` | Empty | Location display language, not full-message translation |

Notifications are not a durable queue. Pending batches live in memory, delivery failures have no guaranteed retry, and channels run sequentially: one failure prevents later channels in that call. HTTP 2xx only confirms transport acceptance, not provider-side business success. Test each channel and verify receipt.

## Production shortcut changes

`enable-production` sets `dry_run=false`, replaces `geoip` and `logging` with platform defaults, sets `notifications.audit=false`, and replaces batching with enabled / 60 seconds / 20 items. Rules, whitelists, and channel credentials remain. It does not restart the service.

If database paths or logging policies are customized, edit `dry_run` manually or restore those fields afterward. Preserving a config file does not mean every field stays unchanged.

Source: [configuration](../internal/config/config.go), [scanner](../internal/daemon/daemon.go), [CLI](../cmd/banhack233/main.go).
