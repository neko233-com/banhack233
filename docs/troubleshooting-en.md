# Troubleshooting and FAQ

Identify the failing layer: log input → counters → firewall → notifications. `status` runs an audit; keep `malware.direct_kill=false` for observation. See [commands](commands-en.md) and [known implementation limits](design-en.md).

## Running service, no bans

| Check | Question | Next step |
| --- | --- | --- |
| Actual config | Does `systemctl cat banhack233` show the expected `ExecStart` | Check `-config`; missing files now fail validation |
| Mode | `dry_run=true` or `action=notify` | These modes intentionally do not apply bans |
| Whitelist | Is the source covered by an IP/CIDR exception | Ignoring trusted sources is expected |
| Input | Does the file exist, allow reads, and receive fresh failures | Initial EOF mode skips history; journal-only hosts need text-log routing |
| Counters | User splitting, successful resets, or regional thresholds | Replay independently; Guangzhou's example threshold is 100 rather than 5 |
| Backend | `scan error` or `no nft or iptables found` | Repair permissions/backend; nft runtime failure does not fall back |
| Port/platform | TCP 22, Windows, or macOS | Check platform limits; a listed rule is not proof of blocking |

Linux evidence collection:

```sh
banhack233 version
sudo systemctl cat banhack233
sudo systemctl status banhack233 --no-pager
sudo journalctl -u banhack233 -n 100 --no-pager
sudo banhack233 ban-list
```

Inspect only the relevant backend: `sudo nft list table inet banhack233`, or `sudo iptables -S INPUT` / `sudo ip6tables -S INPUT`. iptables output may include unrelated rules; do not delete them all.

## Why trigger before five typed failures?

Defaults count only explicit password-failure records. Duplicate forwarding across files, shared usernames, historical replay or custom broad expressions can still inflate counts. Windows event IDs are persisted and deduplicated. Compare actual config with `safe-ssh` and replay a minimal sample.

Replay the actual minimal log sample. For password-only policy, replace `patterns` using the [configuration fragment](configuration-en.md), retaining successful-login reset and office whitelisting.

## Whitelisted, still unable to connect

Restart after updating config. Whitelisting is not a network ACCEPT rule and cannot undo another tool's block. Verify the address observed by the server, IPv6, and CIDR. Follow [recovery steps](operations-en.md), then check cloud rules and sshd limits. Public routing does not expose a client's NIC MAC address for this identification.

## Expiry or restart inconsistency

Check daemon activity, dry-run mode, removal errors, and other still-active bans for the same IP. Clock changes affect recorded deadlines. Kernel rules and saved state are separate: service stops, host reboots, manual firewall edits, and restored state require checking both.

Custom actions have no generic reversal. Do not delete production state merely to reset counters; it also carries information needed for expiry release.

## Keepalive applied, sessions still disconnect

```sh
sudo sshd -t
sudo sshd -T
sudo sshd -T -C user=root,addr=203.0.113.10,host=client.example
```

Substitute the real user/source, then inspect `clientaliveinterval`, `clientalivecountmax`, `passwordauthentication`, `permitrootlogin`, and `port`. Check Include/Match, reload results, a new login, client sleep, and network loss. 24 hours is not an availability guarantee. Business TCP sessions also depend on protocol heartbeats and application/proxy timeouts. [OpenSSH effective-config testing](https://man.openbsd.org/sshd.8)

`keepalive -write` does not apply global TCP tuning without explicit `-tcp`, and cannot override an application's intentional disconnect.

## Missing or delayed notifications

Test channels independently:

```sh
banhack233 notify-test -channel feishu
banhack233 notify-test -channel telegram
banhack233 notify-test -channel discord
banhack233 notify-test -channel slack
banhack233 notify-test -channel webhook:custom-json
banhack233 notify-test -channel email
```

For Feishu/Lark check URL, signing secret, and clock. For Discord/Slack check destination and permissions. For generic webhooks check format, headers, and receiver logs. For email check SMTP authentication; set both host and port for a custom server. Do not paste credentials into issues.

An independent worker checks batches every second. Each channel has a 10-second deadline; failed channels retry from the durable queue for up to 24 hours. Telegram/Feishu business errors are checked. Confirm actual receipt and inspect queue/log files; see [notifications](notifications-en.md).

## Scan results and reports

No obvious indicator proves nothing about absence of compromise. Temporary executables and mining tools may be legitimate. Audits are always read-only; manual cleanup defaults off and only recognized intrusion executable names qualify. Review paths, reports and persistence evidence.

Manual `malware-scan -name incident-review` writes a report and prints its path. Names use second-resolution timestamps; collisions add a numeric suffix to the report name and never overwrite an existing report. All `.txt` files in the report directory participate in modification-time count pruning; archive long-term evidence separately.

## Minimal issue report

Provide the following in a [GitHub Issue](https://github.com/neko233-com/banhack233/issues/new), keeping credentials and full production config local:

```text
Version / commit:
OS, architecture, startup method:
Actual SSH port, IPv4 / IPv6:
Command (redacted), exit status, timestamp / timezone:
Expected versus actual behavior:
Actual -config path and relevant redacted fields:
Minimal authentication log sample (preserve structure; replace addresses/users):
Relevant scan error / target firewall rules:
Reproducible with independent state and dry_run=true:
```

Remove webhook URLs, signing secrets, SMTP credentials, and Authorization headers. Anonymize addresses, usernames, hostnames, and process arguments as appropriate. Do not upload complete `state.json` or real `config.json`.
