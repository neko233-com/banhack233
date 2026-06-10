package audit

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/neko233-com/banhack233/internal/config"
)

type Finding struct {
	Level   string
	Message string
	Fix     string
}

func Run(cfg config.Config) []Finding {
	var findings []Finding
	if runtime.GOOS == "linux" {
		findings = append(findings, auditLinuxSSH(cfg)...)
	}
	if cfg.DryRun {
		findings = append(findings, Finding{Level: "info", Message: "dry_run=true, firewall bans only notify", Fix: "Set dry_run=false after test."})
	}
	if !cfg.Notifications.Console && !cfg.Notifications.Feishu.Enabled && !cfg.Notifications.Email.Enabled {
		findings = append(findings, Finding{Level: "warn", Message: "no notification channel enabled", Fix: "Enable feishu or email."})
	}
	return findings
}

func auditLinuxSSH(cfg config.Config) []Finding {
	values := readSSHDConfig("/etc/ssh/sshd_config")
	ssh := cfg.Hardening.SSH
	var out []Finding
	if ssh.PasswordAuthentication {
		out = append(out, Finding{Level: "info", Message: "SSH password login enabled by policy", Fix: "Use strong password, low MaxAuthTries, fail2ban automation, and alerts."})
	}
	if get(values, "PermitRootLogin") != "no" {
		out = append(out, Finding{Level: "high", Message: "SSH root login not disabled", Fix: "Set PermitRootLogin no."})
	}
	if get(values, "PasswordAuthentication") != boolWord(ssh.PasswordAuthentication) {
		out = append(out, Finding{Level: "medium", Message: "SSH PasswordAuthentication differs from config", Fix: "Run banhack233 secure-ssh -write after confirming users."})
	}
	if get(values, "PermitEmptyPasswords") != "no" {
		out = append(out, Finding{Level: "high", Message: "SSH empty password setting not explicitly disabled", Fix: "Set PermitEmptyPasswords no."})
	}
	return out
}

func readSSHDConfig(path string) map[string]string {
	values := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			values[fields[0]] = strings.ToLower(fields[1])
		}
	}
	return values
}

func get(values map[string]string, key string) string {
	return strings.TrimSpace(values[key])
}

func boolWord(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func Format(findings []Finding) string {
	if len(findings) == 0 {
		return "audit ok"
	}
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "[%s] %s fix=%s\n", f.Level, f.Message, f.Fix)
	}
	return strings.TrimSpace(b.String())
}
