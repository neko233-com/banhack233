# Verified stable updates and rollback

v0.2.1 adds daily checks, verified installation, managed-service restart and startup-failure rollback. Updates download this project's published GitHub Release assets; they do not execute `main` scripts, create SSH connections or trigger GitHub Actions.

## Upgrade an older installation

Use the [installer](../README-EN.md) once to obtain v0.2.1 or later. Installers verify SHA256SUMS and preserve configuration. Existing rules do not silently inherit new defaults.

```sh
sudo banhack233 safe-ssh -config /etc/banhack233/config.json
sudo banhack233 safe-ssh -write -config /etc/banhack233/config.json
sudo banhack233 config-check -config /etc/banhack233/config.json
sudo systemctl restart banhack233
```

`safe-ssh` migrates only the rule named `ssh-auth-failure`: password-only matching, per-user counting, success reset and 10-minute grace. It preserves thresholds, whitelist, paths and other preferences, saving `.before-safe-ssh`. Review custom rules separately. Whitelist office/recovery egress addresses before production.

## Enable daily updates

Install and validate the managed daemon first. Restart updates require it to be running, so an intentionally stopped service is not silently started.

```sh
sudo banhack233 install-autostart -config /etc/banhack233/config.json
sudo banhack233 auto-update -enable -config /etc/banhack233/config.json
banhack233 auto-update -status
```

| Platform | Schedule | Result log |
| --- | --- | --- |
| Linux | Daily systemd timer, up to one hour jitter, missed-run catch-up | `journalctl -u banhack233-update.service` |
| macOS | launchd every 86400 seconds, not an exact midnight schedule | `/var/log/banhack233-update.log` |
| Windows | SYSTEM daily at 03:15, up to one hour jitter, catch-up | Executable-adjacent `.update.log`, one `.old` after 5 MB; manual launcher failures in `.update-launcher.log` |

Use the actual macOS config path. On Windows run the full executable path in administrator PowerShell; a temporary helper avoids replacing an in-use executable. SYSTEM executables and update scripts must live in an administrator-controlled directory.

## Check, install or disable

```sh
banhack233 update -config /etc/banhack233/config.json
sudo banhack233 update -apply -restart -config /etc/banhack233/config.json
sudo banhack233 auto-update -disable
```

Without `-apply`, the command only checks. `-restart` restarts banhack233, not sshd, and does not change keepalive. Without it, the running process keeps the old binary until an operator restarts it. Manual Windows apply starts a hidden helper: “started” is not completion; inspect its log and `version`.

## Installation guarantees and boundaries

1. Only published, non-draft, non-prerelease `vMAJOR.MINOR.PATCH` versions, strictly newer; `dev` builds do not auto-upgrade.
2. Exact OS/architecture asset and checksum manifest from the same Release; bounded HTTPS downloads.
3. Verify SHA256, binary `version`, and the new binary's `config-check` before replacing anything.
4. Same-directory staging, `.previous` backup, `.update.json` checksum receipt and installation lock.
5. After restart, require three consecutive active checks; failure attempts binary restoration and restart, while reporting a failed update.

The updater preserves config, whitelist, credentials, state and reports. Incompatible config causes refusal and requires operator migration. SHA256 proves artifact integrity, not an independent publisher signature: GitHub repository/release access and local administrator trust remain relevant. [GitHub latest Release semantics](https://docs.github.com/en/rest/releases/releases#get-the-latest-release)

An active process is not proof of effective protection. Verify scan logs, actual firewall ports and notification receipt. Power loss and disk failures still require independent backups and recovery access.

For GitHub API 403/429, check connectivity and rate limits. An optional `BANHACK233_GITHUB_TOKEN` in the updater environment raises API quota; it is sent only to `api.github.com`, never asset endpoints or update logs. Failed updates retain the installed version and are checked again on the next daily run. Inspect scheduler logs for the result.

Since v0.2.2, asset bodies have a ten-minute download limit, headers thirty seconds, and each update fifteen minutes overall. Linux/Windows scheduler jobs allow twenty minutes. If an older updater times out on a slow link, bootstrap it using the current installer, then rerun `auto-update -enable` to refresh scheduler limits. Failed downloads retain the existing installation.

## Rollback

```sh
sudo banhack233 auto-update -disable
sudo banhack233 update -rollback -restart -config /etc/banhack233/config.json
banhack233 version
sudo journalctl -u banhack233 -n 100 --no-pager
```

Rollback verifies the saved backup digest and restores only the executable, preserving current whitelist/state. Older code may not accept newer config fields; review compatibility. Disable daily updates to avoid reinstalling the reverted release tomorrow. Installer `.installer-backup` and updater `.previous` are different: after the initial installer upgrade, use the [manual recovery procedure](operations-en.md) for the installer backup.
