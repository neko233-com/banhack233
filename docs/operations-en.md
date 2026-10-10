# Deployment, recovery, and operations

The main procedure targets Linux + systemd + SSH TCP 22. For custom SSH ports, macOS, or Windows, read [platform limits](design-en.md). Replace all example addresses before real operations.

## First deployment acceptance

1. Install from the [README](../README-EN.md), record `banhack233 version`, and confirm the actual config path.
2. Whitelist office and recovery-management egress addresses. Keep `dry_run=true` and `malware.direct_kill=false`.
3. Confirm authentication files exist and receive new logs. A journal-only system cannot use the default text paths without log routing.
4. Install autostart with an explicit config; inspect startup arguments and logs. With `start_at_end=true`, start scanning before generating new test events.
5. Validate matching using isolated replay below, then run `notify-test` separately and confirm receipt.
6. Preview any SSH baseline/keepalive changes, retain the required root/password policy and management users, then test a new login from another session after applying.
7. Set `dry_run=false`, restart, and inspect the backend and new events. Do not validate bans by repeatedly mistyping a real office account password.

```sh
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10 2001:db8::10
sudo banhack233 install-autostart -config /etc/banhack233/config.json
sudo systemctl status banhack233 --no-pager
sudo systemctl cat banhack233
sudo journalctl -u banhack233 -n 100 --no-pager
```

For an existing service, explicitly restart after configuration changes; reinstalling autostart is not a substitute. Whitelisting only affects this program, not cloud firewalls, other blockers, or sshd policy.

## Isolated log replay without real bans

This complete shell example uses a new temporary directory and state file, two password-failure events, no real firewall changes, no malware scan, and no external notifications. An installed binary is required; root is not.

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

Expect one dry-run alert on the first call and no repeated alert without new lines on the second. State stores offsets/cooldowns; no real firewall rule should be added. Use a new directory per independent test case so prior cooldowns do not interfere.

Additional cases: with threshold 3, failure/failure/success/failure should not trigger; whitelisting the test address should suppress it; substituting `2001:db8::42` checks IPv6 matching. [Scanner tests](../internal/daemon/daemon_test.go) cover further cases, including expiry, users, and malformed logs.

## Shared egress and lockout recovery

A public SSH server cannot distinguish computers behind NAT by their NIC MAC addresses. Per-user counters separate some failures, but enforcement still blocks a source IP and can affect coworkers.

From an available management session or host console:

```sh
sudo banhack233 whitelist -config /etc/banhack233/config.json 203.0.113.10
sudo banhack233 unban 203.0.113.10
sudo systemctl restart banhack233
sudo banhack233 ban-list
```

Use the egress address actually observed by the server; check IPv4 and IPv6 separately. Production reconciliation releases tracked automatic bans for newly whitelisted addresses. Dry-run leaves existing firewall rules untouched, so the recovery procedure explicitly includes `unban`.

If access still fails, inspect other blockers, cloud rules, sshd logs, listeners, and effective configuration. `unban` auto-selects a backend; installing another firewall tool can change that selection. Check the original backend for leftovers.

## Back up before upgrading

Use a short maintenance window and stop the daemon before copying state. This example uses Linux defaults and does not back up SSH/sysctl settings; back those up separately if changing them.

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

Resolve backup failures before replacing the program. Backups contain notification credentials and source addresses; restrict access and do not attach them to public issues.

## Version-pinned installation and rollback

For example, reinstall the published `v0.1.22`, pinning both script and binary versions:

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/banhack233/v0.1.22/scripts/install.sh | sh -s -- v0.1.22
banhack233 version
sudo systemctl restart banhack233
sudo journalctl -u banhack233 -n 100 --no-pager
```

Existing configuration is preserved. On first installation this script still fetches the example from `main`; preserve a version-specific config for reproducible deployment. The installer does not automatically verify release SHA-256 hashes. Download the selected asset and `SHA256SUMS.txt` from that release and verify them before manual installation when required.

To roll back, set `backup_dir` to the verified backup, stop the service, restore binary/config, then restart and verify version, logs, and a new login. Do not automatically restore old state: check version compatibility and live firewall contents together. Restoring state does not recreate kernel firewall rules.

```sh
# backup_dir must identify the verified backup directory
sudo systemctl stop banhack233
sudo install -m 755 "$backup_dir/banhack233" /usr/local/bin/banhack233
sudo install -m 600 "$backup_dir/config.json" /etc/banhack233/config.json
sudo systemctl start banhack233
banhack233 version
```

## Pause or return to observation

Setting `dry_run=true` and restarting stops new real bans and automatic release of existing real bans; it does not undo them. Unban individual addresses and inspect the backend as needed. Stopping the daemon also leaves rules in place and stops expiry processing.

Use `uninstall-autostart` to remove startup configuration, retaining config/reports for recovery. SSH managed blocks, keepalive drop-ins, and optional TCP sysctl changes require separate review/restoration. Removing a sysctl file does not immediately restore already-applied kernel values. Do not flush the host's entire firewall as a substitute for targeted cleanup.

## Windows and macOS startup paths

Administrator PowerShell installations normally use these paths. For non-admin installs, substitute the LocalAppData paths printed by the installer. This establishes startup, not proof of effective blocking.

```powershell
$exe = Join-Path $env:ProgramFiles 'banhack233/banhack233.exe'
$cfg = Join-Path $env:ProgramData 'banhack233/config.json'
& $exe status -config $cfg
& $exe install-autostart -config $cfg
schtasks /Query /TN banhack233 /V /FO LIST
```

For Windows, generate first-time configuration with native `init-config` or review every field rather than copying Linux `/var/log/...` paths. The task runs as SYSTEM and must access the binary/config. On macOS, the shell installer writes `/etc/banhack233/config.json` while the program defaults to `/usr/local/etc/banhack233/config.json`; pass the actual path and review state/log paths and authentication sources.

## Routine checks and evidence retention

Check recent successful scanning, repeated `scan error` messages, source-log growth, backend rules, notification receipt, and disk capacity. Application logs default to 10 MB rotation / 30-day retention; reports keep 50 `.txt` files. Record manual scan report paths and move long-term evidence into a separate archive outside count-based pruning.

There is no built-in Prometheus endpoint, retry queue, or performance SLO. On high-volume hosts, measure scan duration, CPU/RSS, state size, and false positives before adjusting intervals or thresholds.
