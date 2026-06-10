package autostart

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	serviceName = "banhack233.service"
	taskName    = "banhack233"
)

type StatusInfo struct {
	Backend string
	Enabled bool
	Active  bool
	Detail  string
}

func Enable(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "linux":
		unitPath := filepath.Join("/", "etc", "systemd", "system", serviceName)
		content := fmt.Sprintf("[Unit]\nDescription=banhack233 adaptive attack blocker\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=%s run -config %s\nRestart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n", escapeSystemd(exe), escapeSystemd(configPath))
		if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
			return err
		}
		if err := run(exec.Command("systemctl", "daemon-reload")); err != nil {
			return err
		}
		return run(exec.Command("systemctl", "enable", "--now", serviceName))
	case "windows":
		cmd := fmt.Sprintf("\"%s\" run -config \"%s\"", exe, configPath)
		if err := run(exec.Command("schtasks", "/Create", "/F", "/TN", taskName, "/SC", "ONSTART", "/RL", "HIGHEST", "/RU", "SYSTEM", "/TR", cmd)); err != nil {
			return err
		}
		return run(exec.Command("schtasks", "/Run", "/TN", taskName))
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func Disable() error {
	switch runtime.GOOS {
	case "linux":
		unitPath := filepath.Join("/", "etc", "systemd", "system", serviceName)
		_ = run(exec.Command("systemctl", "disable", "--now", serviceName))
		_ = os.Remove(unitPath)
		return run(exec.Command("systemctl", "daemon-reload"))
	case "windows":
		_ = run(exec.Command("schtasks", "/End", "/TN", taskName))
		return run(exec.Command("schtasks", "/Delete", "/F", "/TN", taskName))
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func Status() (StatusInfo, error) {
	switch runtime.GOOS {
	case "linux":
		unitPath := filepath.Join("/", "etc", "systemd", "system", serviceName)
		st := StatusInfo{Backend: "systemd", Detail: unitPath}
		if _, err := os.Stat(unitPath); err != nil {
			return st, nil
		}
		st.Enabled = true
		st.Active = run(exec.Command("systemctl", "is-active", "--quiet", serviceName)) == nil
		return st, nil
	case "windows":
		st := StatusInfo{Backend: "schtasks", Detail: taskName}
		if err := run(exec.Command("schtasks", "/Query", "/TN", taskName)); err != nil {
			return st, nil
		}
		st.Enabled = true
		return st, nil
	default:
		return StatusInfo{Backend: runtime.GOOS}, nil
	}
}

func escapeSystemd(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, " ", "\\x20")
}

func run(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s: %s", strings.Join(cmd.Args, " "), msg)
	}
	return nil
}
