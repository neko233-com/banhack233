# Configuration reference

This page describes current defaults, loading semantics, and limits. See the [complete example](../configs/config.json.example), [operations guide](operations-en.md), and [command reference](commands-en.md).

## Loading and applying configuration

- Configuration is JSON: no comments, trailing commas, or environment-variable expansion. Durations are strings such as `"30s"`, `"10m"`, and `"24h"`; `"1d"` is invalid.
- The program loads defaults, then overlays the file. Omitted top-level fields retain defaults. An explicit `rules` array replaces the default rules; individual array entries do not inherit the default SSH template.
- A custom rule without `count_by_user` therefore uses `false`; without `reset_patterns`, it has no successful-login reset. A rule fragment is not a complete rule.
- Missing files, unknown fields, malformed regexes, invalid ports and duplicate rule names are errors. Run `config-check` before restarting.
- `config-check` validates without scanning, changing the firewall or sending messages.
- The daemon reads configuration at startup, with no hot reload. Restart it after changes and use the same config path for administrative commands.
- Installers call the platform binary’s `init-config`; existing files remain unchanged.

## Top-level fields

| Field | Default | Behavior |
| --- | --- | --- |
| `interval` | `"30s"` | Polling interval; one cycle runs immediately at startup. Nonpositive values restore the default |
| `audit_interval` | `"1h"` | Audit eligibility checked during scan cycles, not by an independent timer |
| `state_path` | Platform state directory + `state.json` | Log offsets, hit timestamps, bans, backends, cooldowns, and last audit |
| `dry_run` | `true` | Neither applies nor releases real firewall bans; still writes state/logs and sends notifications. Not a global read-only switch |
| `start_at_end` | `true` | First text read starts at EOF; first Windows query records the newest event ID without replay |
| `ignore_ips` | `["127.0.0.1", "::1"]` | IPv4, IPv6, or CIDR, not hostnames. An explicit array replaces this list |
| `rules` | One SSH authentication rule | `[]` stops rule scanning, but does not disable audits or immediately remove existing firewall entries |

A process lock rejects concurrent daemons or `test` using the same state path. Keep independent state and queue paths for replay tests.

## Rule fields

| Field | Default SSH rule / normalized fallback | Behavior |
| --- | --- | --- |
| `name` | `ssh-auth-failure`; empty becomes `rule` | Part of state identity; use unique names without `|` |
| `log_paths` | Platform authentication logs | Linux chooses `auth.log` if present, otherwise `secure`; the example lists both. Missing files are skipped |
| `patterns` | One explicit password-failure expression | Go regexp; one `<HOST>` or named IP group; optional username group |
| `reset_patterns` | `Accepted ... from <HOST>` | Processed first; clears all username hit counters for that IP within this rule, without removing existing bans |
| `max_attempts` | `5` | Triggers at the threshold; counts matching log events, not exactly typed passwords |
| `find_time` | `"10m"` | Window uses observation time, not historical timestamps parsed from log text |
| `ban_time` | `"1h"` | Ban expiry, or notification cooldown per counting key in dry-run / notify mode |
| `action` | `"auto"` | Automatic backend selection; `"notify"` only notifies; other strings are executable names invoked with the IP |
| `count_by_user` | `true` in the default rule | Rule+IP+username if captured, otherwise rule+IP. Enforcement remains IP-based |
| `region_rules.max_attempts` | `广州: 100`, `Guangzhou: 100` | Regional threshold overrides; lookup failure or no match uses the base threshold |

Nonpositive attempts or durations restore defaults; they do not disable bans. Empty region names and nonpositive regional thresholds are removed.

Custom `action` values are executable names invoked with one IP argument, not shell commands or backend names. Use `auto` for managed rules. Top-level `ssh_ports` defaults to `[22]`; set `[22,2222]` explicitly when both are SSH ports. It does not change sshd ports.

Region matching is case-insensitive and supports partial country/region/city names. Multiple matches deterministically choose the highest threshold to reduce false bans. The database is IPv4-only; IPv6 detection works without GeoIP. Prefer explicit office whitelists.

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
| `malware.direct_kill` | `false` | Only cleanup switch; manual scans only, restricted intrusion executable names; audits always read-only |
| `malware.report_dir` | State directory + `reports` | Manual reports; do not mix unrelated `.txt` files here |
| `malware.report_keep` | `50` | After writing a report, retain the newest N `.txt` files by modification time; nonpositive values restore 50 |

This is a lightweight Linux host antivirus / intrusion-triage helper, not a full commercial antivirus or EDR. Manual reports use `{name}.yyyy-MM-dd_HH-mm-ss.txt`. The LRU-like count policy keeps recently modified files; reading does not refresh retention. Automatic audits write application logs, not a separate manual report archive.

## Notifications

All fields below belong to `notifications`. Channel-specific examples remain in the [README](../README-EN.md).

| Field | Default | Behavior |
| --- | --- | --- |
| `console` | `true` | Notification output to stdout |
| `audit` | `false` | Sends audit findings; does not enable/disable audit execution |
| `batch.enabled` | `true` | Batches ban events; notify/audit events enter the durable queue directly |
| `batch.interval` | `"60s"` | Measured from the first pending event; an independent worker checks once per second |
| `batch.max_items` | `20` | Sends when pending count reaches the limit |
| `feishu` / `discord` / `slack` | `enabled=false` | `enabled`, `url`, `location_language`; Feishu/Lark can use a signing `secret` |
| `webhooks[]` | Empty | `name`, `enabled`, `url`, `format`, `headers`, `secret`, `location_language` |
| `webhooks[].format` | Empty means plain text | `text`, `json`, `feishu`, `lark`, `discord`, `slack` |
| `email.enabled` | `false` | SMTP delivery |
| `email.from` / `to` | Empty | Sender; comma-separated recipients |
| `email.password` | Empty | SMTP authorization code or applicable credential |
| `email.smtp_host` / `smtp_port` | Empty / `0` | Infers only missing values; TLS 465 or mandatory STARTTLS for other ports |
| `email.location_language` | Empty | Location display language, not full-message translation |

Notifications use a bounded durable queue and independent channels with 10-second deadlines. Telegram/Feishu business errors are checked; failed channels retry without resending confirmed channels. See [delivery guarantees and setup](notifications-en.md).

## Production shortcut changes

`enable-production` changes only `dry_run=false`; all other preferences remain. Restart the service afterward.

Existing rules are not silently migrated by upgrades. Use `safe-ssh` to preview and `safe-ssh -write` to migrate the named default rule.

Source: [configuration](../internal/config/config.go), [scanner](../internal/daemon/daemon.go), [CLI](../cmd/banhack233/main.go).

## New safety and delivery fields

- `ssh_ports`: destination TCP ports, default `[22]` (Linux/Windows); match actual SSH listeners. PF remains administrator-managed.
- Rule `success_grace`: default template `"10m"`; omitted in a custom rule means `"0s"`. A successful login protects the source IP across usernames in that rule. The current batch is protected even with zero grace. It does not release existing bans.
- `notifications.queue_path`: defaults to `state_path + ".notifications.json"`; private writable file, distinct from state.
- `notifications.telegram`: `enabled`, `bot_token` or `bot_token_env`, `chat_id`, optional `message_thread_id` and `disable_notification`. Only `bot_token_env` reads the named environment variable; JSON is not generally expanded.
