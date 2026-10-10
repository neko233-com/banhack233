# Command reference and side effects

Examples target Linux unless stated otherwise. `-config` is a subcommand option, placed after the command. For `whitelist`, put options before IP arguments. Not every command accepts `-config`.

```sh
banhack233 status -config /etc/banhack233/config.json
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10
```

Example addresses are documentation placeholders; substitute a real trusted address before adding an exception.

## Inspection and diagnosis

| Command | Options | Actual behavior |
| --- | --- | --- |
| `help` / `-h` / `--help` | None | Help output |
| `version` | None | Version, commit, build time |
| `status` | `-config` | Config summary, autostart status, and a system audit |
| `doctor` / `audit` | `-config` | System audit and recommendations; does not apply the SSH baseline |
| `ban-list` | None | Rules from the currently auto-selected backend, not state-file expiry records |
| `autostart-status` | None | Autostart configuration; Windows does not currently determine whether the task is running |
| `whitelist` | `-config`, no IP arguments | Displays whitelist |

`config-check`, `status`, `doctor` and scheduled audits never kill processes. `dry_run` controls firewall actions; manual malware cleanup requires explicit policy.

## Configuration and daemon

| Command | Options | Actual behavior |
| --- | --- | --- |
| `init-config` | `-config`, `-force` | Writes platform defaults; refuses existing files unless `-force` is supplied |
| `run` | `-config` | Foreground daemon; writes state/logs and performs configured bans, audits, notifications |
| `test` | `-config` | One real daemon scan cycle; writes state and can apply/release bans in production mode |
| `enable-production` | `-config` | Changes only dry_run to false; restart required |
| `whitelist` | `-config`, one or more IPs/CIDRs | Appends persistent config exceptions; restart required. Does not create firewall ACCEPT rules |
| `unban [-config path] <ip>` | One IP only | Removes entries for configured SSH ports and legacy port 22; does not edit whitelist or state |

There is no `whitelist remove` subcommand. Edit `ignore_ips` and restart to remove an exception. `unban` is temporary firewall remediation; add a whitelist entry for a lasting exception.

## SSH and TCP keepalive

Both commands below require Linux. Without `-write`, they preview changes.

| Command | Options | Write scope |
| --- | --- | --- |
| `secure-ssh` | `-config`, `-write`, `-force` | Updates a managed block in `/etc/ssh/sshd_config`, backs up and validates; requires `allowed_users` or `-force` |
| `keepalive` | `-write`, `-ssh-hours` (24), `-tcp` | Default SSH drop-in preview; `-write` applies SSH settings; only explicit `-tcp` writes global TCP/conntrack settings |

```sh
sudo banhack233 secure-ssh -config /etc/banhack233/config.json
sudo banhack233 keepalive -ssh-hours 24
```

Review the preview and retain a recovery login before applying. `keepalive` also includes `PasswordAuthentication yes` and `PermitRootLogin yes`, not only probe settings. `-force` bypasses the allowed-user requirement, not OpenSSH configuration precedence.

After writing, run `sshd -t`, inspect effective values with `sshd -T`, and test a new connection. The program attempts to reload `ssh` / `sshd` but currently does not propagate reload errors; an “applied” message still requires verification. [OpenSSH test modes](https://man.openbsd.org/sshd.8)

## Scanning and notifications

| Command | Options | Actual behavior |
| --- | --- | --- |
| `malware-scan` | `-config`, `-name` (`malware-scan`), `-kill` | Manual scan and report path; `-kill` or configured `direct_kill=true` permits termination |
| `notify-test` | `-config`, `-channel`, `-message` | Actually sends test messages to configured channels; no firewall ban |

```sh
sudo banhack233 malware-scan -name incident-review
banhack233 notify-test -channel feishu,email -message 'banhack233 channel check'
banhack233 notify-test -channel webhook:custom-json
```

Selectors support `console,telegram,feishu,discord,slack,email,webhook` and `webhook:<name>`. Omitting `-kill` does not override `direct_kill=true`. Non-Linux scans report unsupported; this is not a clean-health result.

## Autostart

```sh
sudo banhack233 install-autostart -config /etc/banhack233/config.json
banhack233 autostart-status
sudo banhack233 uninstall-autostart
```

Installation writes startup configuration and attempts to start: systemd on Linux, launchd on macOS, scheduled tasks on Windows. System configuration and firewall operations require administrative privileges. See [Windows examples](operations-en.md).

`uninstall-autostart` stops/removes startup configuration. It does not remove binaries, config, reports, firewall bans, SSH changes, or sysctl changes.

## Exit status and verification

CLI errors return exit code 1. Per-cycle daemon errors are logged and scanning continues; notification delivery failures do not stop log ingestion. `test` returns final queue-flush errors. `config-check` is the read-only validator.

`jq empty /etc/banhack233/config.json` checks JSON syntax only, not field names, regexes, permissions, or production suitability. Use [isolated log replay](operations-en.md) for functional checks; production `test` is not a side-effect-free linter.

Source: [CLI](../cmd/banhack233/main.go), [autostart](../internal/autostart/autostart.go), [audit](../internal/audit/audit.go).

## Updates and migration

| Command | Effect |
| --- | --- |
| `config-check -config path` | Strict, read-only validation |
| `safe-ssh [-write] [-config path]` | Preview/apply password-only default-rule migration, saves `.before-safe-ssh` backup |
| `update [-apply] [-restart] [-rollback] [-config path]` | Check by default; explicitly install or restore previous verified binary |
| `auto-update [-enable/-disable/-status] [-config path]` | Daily OS schedule; no action defaults to status |

See [update prerequisites, platform commands and rollback](updates-en.md).
