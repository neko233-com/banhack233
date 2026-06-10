package audit

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/malware"
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
		findings = append(findings, auditLinuxFirewall()...)
		findings = append(findings, auditLinuxUpdates()...)
		if cfg.Malware.Enabled {
			findings = append(findings, auditMalware(cfg.Malware)...)
		}
	}
	if cfg.DryRun {
		findings = append(findings, Finding{Level: "info", Message: "dry_run=true, firewall bans only notify", Fix: "Set dry_run=false after test."})
	}
	if !hasNotification(cfg.Notifications) {
		findings = append(findings, Finding{Level: "warn", Message: "no notification channel enabled", Fix: "Enable feishu, discord, slack, webhook, or email."})
	}
	return findings
}

func auditMalware(cfg config.MalwareConfig) []Finding {
	var out []Finding
	opts := malware.Options{Kill: cfg.AutoRemediate, Quarantine: cfg.AutoRemediate}
	for _, f := range malware.Scan(cfg, opts) {
		if f.Level == "info" {
			continue
		}
		out = append(out, Finding{Level: f.Level, Message: fmt.Sprintf("malware indicator: %s %s", f.Type, f.Target), Fix: f.Detail})
	}
	return out
}

func hasNotification(cfg config.NotificationSet) bool {
	if cfg.Console || cfg.Feishu.Enabled || cfg.Discord.Enabled || cfg.Slack.Enabled || cfg.Email.Enabled {
		return true
	}
	for _, target := range cfg.Webhooks {
		if target.Enabled {
			return true
		}
	}
	return false
}

func auditLinuxSSH(cfg config.Config) []Finding {
	values := readSSHDConfig("/etc/ssh/sshd_config")
	ssh := cfg.Hardening.SSH
	var out []Finding
	if ssh.PasswordAuthentication {
		out = append(out, Finding{Level: "info", Message: "SSH password login enabled by policy", Fix: "Use strong password, low MaxAuthTries, fail2ban automation, and alerts."})
	}
	if get(values, "PermitRootLogin") != strings.ToLower(ssh.PermitRootLogin) {
		out = append(out, Finding{Level: "medium", Message: "SSH PermitRootLogin differs from config", Fix: "Run secure-ssh only after confirming current session safety."})
	}
	if get(values, "PasswordAuthentication") != boolWord(ssh.PasswordAuthentication) {
		out = append(out, Finding{Level: "medium", Message: "SSH PasswordAuthentication differs from config", Fix: "Run banhack233 secure-ssh -write after confirming users."})
	}
	if get(values, "PermitEmptyPasswords") != "no" {
		out = append(out, Finding{Level: "high", Message: "SSH empty password setting not explicitly disabled", Fix: "Set PermitEmptyPasswords no."})
	}
	if get(values, "MaxAuthTries") == "" {
		out = append(out, Finding{Level: "medium", Message: "SSH MaxAuthTries not explicitly limited", Fix: "Set MaxAuthTries 3."})
	}
	if get(values, "LoginGraceTime") == "" {
		out = append(out, Finding{Level: "medium", Message: "SSH LoginGraceTime not explicitly limited", Fix: "Set LoginGraceTime 20s."})
	}
	return out
}

func auditLinuxFirewall() []Finding {
	if _, err := exec.LookPath("nft"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		return nil
	}
	return []Finding{{Level: "high", Message: "no nft or iptables firewall backend found", Fix: "Install nftables or iptables."}}
}

func auditLinuxUpdates() []Finding {
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		return []Finding{{Level: "medium", Message: "system reboot required after updates", Fix: "Schedule reboot."}}
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		out, _ := exec.Command("sh", "-c", "apt list --upgradable 2>/dev/null | sed 1d | head -n 1").Output()
		if strings.TrimSpace(string(out)) != "" {
			return []Finding{{Level: "info", Message: "package updates available", Fix: "Run apt update && apt upgrade during maintenance window."}}
		}
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		if err := exec.Command("dnf", "check-update", "-q").Run(); err != nil {
			return []Finding{{Level: "info", Message: "package updates may be available", Fix: "Run dnf upgrade during maintenance window."}}
		}
	}
	return nil
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
