package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type systemService struct{}

func command(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}
func (systemService) Stop(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return command(ctx, "systemctl", "stop", "banhack233.service")
	case "darwin":
		return command(ctx, "launchctl", "bootout", "system/com.neko233.banhack233")
	case "windows":
		return command(ctx, "schtasks", "/End", "/TN", "banhack233")
	}
	return fmt.Errorf("unsupported service platform")
}
func (systemService) Start(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return command(ctx, "systemctl", "start", "banhack233.service")
	case "darwin":
		return command(ctx, "launchctl", "bootstrap", "system", "/Library/LaunchDaemons/com.neko233.banhack233.plist")
	case "windows":
		return command(ctx, "schtasks", "/Run", "/TN", "banhack233")
	}
	return fmt.Errorf("unsupported service platform")
}
func (systemService) Healthy(ctx context.Context) error {
	// Require three consecutive active checks; a config-error crash is not a successful update.
	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		var err error
		switch runtime.GOOS {
		case "linux":
			err = command(ctx, "systemctl", "is-active", "--quiet", "banhack233.service")
		case "darwin":
			out, e := exec.CommandContext(ctx, "launchctl", "print", "system/com.neko233.banhack233").Output()
			err = e
			if err == nil && !strings.Contains(string(out), "state = running") {
				err = fmt.Errorf("launchd daemon is not running")
			}
		case "windows":
			err = command(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "if ((Get-ScheduledTask -TaskName 'banhack233' -ErrorAction Stop).State -ne 'Running') { exit 1 }")
		default:
			err = fmt.Errorf("unsupported service platform")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func systemdQuote(s string) string {
	return strconv.Quote(strings.ReplaceAll(strings.ReplaceAll(s, "%", "%%"), "$", "$$"))
}
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(s)
}

func Schedule(ctx context.Context, action, configPath string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(exe+configPath, "\r\n") {
		return "", fmt.Errorf("paths cannot contain newlines")
	}
	switch runtime.GOOS {
	case "linux":
		unit := "/etc/systemd/system/banhack233-update.service"
		timer := "/etc/systemd/system/banhack233-update.timer"
		switch action {
		case "enable":
			service := fmt.Sprintf("[Unit]\nDescription=Verified stable release update for banhack233\nWants=network-online.target\nAfter=network-online.target\n\n[Service]\nType=oneshot\nExecStart=%s update -apply -restart -config %s\nTimeoutStartSec=15min\nUMask=0077\n", systemdQuote(exe), systemdQuote(configPath))
			if err = os.WriteFile(unit, []byte(service), 0644); err != nil {
				return "", err
			}
			if err = os.WriteFile(timer, []byte("[Unit]\nDescription=Daily stable release check for banhack233\n[Timer]\nOnCalendar=daily\nRandomizedDelaySec=1h\nPersistent=true\n[Install]\nWantedBy=timers.target\n"), 0644); err != nil {
				return "", err
			}
			if err = command(ctx, "systemctl", "daemon-reload"); err == nil {
				err = command(ctx, "systemctl", "enable", "--now", "banhack233-update.timer")
			}
		case "disable":
			if err = command(ctx, "systemctl", "disable", "--now", "banhack233-update.timer"); err != nil {
				return "", err
			}
			_ = os.Remove(unit)
			_ = os.Remove(timer)
			err = command(ctx, "systemctl", "daemon-reload")
		case "status":
			out, e := exec.CommandContext(ctx, "systemctl", "list-timers", "--all", "banhack233-update.timer", "--no-pager").CombinedOutput()
			return string(out), e
		default:
			return "", fmt.Errorf("unknown auto-update action")
		}
	case "darwin":
		path := "/Library/LaunchDaemons/com.neko233.banhack233.update.plist"
		switch action {
		case "enable":
			body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Label</key><string>com.neko233.banhack233.update</string><key>ProgramArguments</key><array><string>%s</string><string>update</string><string>-apply</string><string>-restart</string><string>-config</string><string>%s</string></array><key>StartInterval</key><integer>86400</integer><key>StandardOutPath</key><string>/var/log/banhack233-update.log</string><key>StandardErrorPath</key><string>/var/log/banhack233-update.log</string></dict></plist>`, xmlEscape(exe), xmlEscape(configPath))
			if err = os.WriteFile(path, []byte(body), 0644); err != nil {
				return "", err
			}
			_ = command(ctx, "launchctl", "bootout", "system/com.neko233.banhack233.update")
			err = command(ctx, "launchctl", "bootstrap", "system", path)
		case "disable":
			err = command(ctx, "launchctl", "bootout", "system/com.neko233.banhack233.update")
			if err == nil {
				err = os.Remove(path)
			}
		case "status":
			out, e := exec.CommandContext(ctx, "launchctl", "print", "system/com.neko233.banhack233.update").CombinedOutput()
			return string(out), e
		}
	case "windows":
		switch action {
		case "enable":
			path, e := windowsScript(exe, configPath)
			if e != nil {
				return "", e
			}
			args := "-NoProfile -NonInteractive -ExecutionPolicy Bypass -WindowStyle Hidden -File \"" + path + "\""
			ps := "$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument " + psQuote(args) + "; $t=New-ScheduledTaskTrigger -Daily -At '03:15' -RandomDelay (New-TimeSpan -Hours 1); $s=New-ScheduledTaskSettingsSet -StartWhenAvailable -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 15); Register-ScheduledTask -TaskName 'banhack233-update' -Action $a -Trigger $t -Settings $s -User SYSTEM -RunLevel Highest -Force | Out-Null"
			err = command(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
		case "disable":
			err = command(ctx, "schtasks", "/Delete", "/F", "/TN", "banhack233-update")
		case "status":
			out, e := exec.CommandContext(ctx, "schtasks", "/Query", "/TN", "banhack233-update", "/V", "/FO", "LIST").CombinedOutput()
			return string(out), e
		}
	default:
		return "", fmt.Errorf("auto-update unsupported on %s", runtime.GOOS)
	}
	if err != nil {
		return "", err
	}
	return "automatic updates: " + action + " (daily stable releases; SHA256 verification; preserves configuration)", nil
}

func windowsScript(exe, configPath string) (string, error) {
	path := filepath.Join(filepath.Dir(exe), "banhack233-update.ps1")
	body := fmt.Sprintf(`param([switch]$Rollback, [switch]$NoRestart)
$ErrorActionPreference = 'Stop'
Start-Sleep -Seconds 2
$target = %s
$configPath = %s
$helper = Join-Path (Split-Path -Parent $target) ('.banhack233-helper-' + [Guid]::NewGuid().ToString('N') + '.exe')
$updateLog = $target + '.update.log'
if ((Test-Path -LiteralPath $updateLog) -and (Get-Item -LiteralPath $updateLog).Length -gt 5MB) { Move-Item -LiteralPath $updateLog -Destination ($updateLog + '.old') -Force }
try {
  Copy-Item -LiteralPath $target -Destination $helper
  $updateArgs = @('update', '-target', $target, '-config', $configPath)
  if ($Rollback) { $updateArgs += '-rollback' } else { $updateArgs += '-apply' }
  if (-not $NoRestart) { $updateArgs += '-restart' }
  $ErrorActionPreference = 'Continue'
  & $helper @updateArgs 2>&1 | Out-File -LiteralPath $updateLog -Append -Encoding utf8
  $updateExit = $LASTEXITCODE
  $ErrorActionPreference = 'Stop'
  if ($updateExit -ne 0) { throw "Update failed; see $updateLog" }
} catch {
  $_ | Out-File -LiteralPath $updateLog -Append -Encoding utf8
  throw
} finally { if (Test-Path -LiteralPath $helper) { Remove-Item -LiteralPath $helper -Force } }
`, psQuote(exe), psQuote(configPath))
	return path, os.WriteFile(path, []byte(body), 0600)
}
