package hardening

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

const sshdConfigPath = "/etc/ssh/sshd_config"

func SecureSSH(cfg config.Config, write bool) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("secure-ssh supports linux only")
	}
	ssh := cfg.Hardening.SSH
	if !ssh.Enabled {
		return "ssh hardening disabled in config", nil
	}
	managed := renderSSHDManagedBlock(ssh)
	if !write {
		return managed, nil
	}
	original, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return "", err
	}
	backup := filepath.Join("/etc/ssh", "sshd_config.banhack233."+time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, original, 0o600); err != nil {
		return "", err
	}
	next := replaceManagedBlock(string(original), managed)
	tmp := sshdConfigPath + ".banhack233.tmp"
	if err := os.WriteFile(tmp, []byte(next), 0o600); err != nil {
		return "", err
	}
	if err := exec.Command("sshd", "-t", "-f", tmp).Run(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("sshd validation failed: %w", err)
	}
	if err := os.Rename(tmp, sshdConfigPath); err != nil {
		return "", err
	}
	_ = exec.Command("systemctl", "reload", "ssh").Run()
	_ = exec.Command("systemctl", "reload", "sshd").Run()
	return "ssh hardened; backup=" + backup, nil
}

func renderSSHDManagedBlock(ssh config.SSHHardening) string {
	var b strings.Builder
	b.WriteString("# BEGIN banhack233 managed\n")
	b.WriteString("PasswordAuthentication " + yesNo(ssh.PasswordAuthentication) + "\n")
	if ssh.PermitRootLogin == "" {
		ssh.PermitRootLogin = "no"
	}
	b.WriteString("PermitRootLogin " + ssh.PermitRootLogin + "\n")
	if len(ssh.AllowedUsers) > 0 {
		b.WriteString("AllowUsers " + strings.Join(ssh.AllowedUsers, " ") + "\n")
	}
	if ssh.MaxAuthTries <= 0 {
		ssh.MaxAuthTries = 3
	}
	b.WriteString(fmt.Sprintf("MaxAuthTries %d\n", ssh.MaxAuthTries))
	if ssh.LoginGraceTime == "" {
		ssh.LoginGraceTime = "20s"
	}
	b.WriteString("LoginGraceTime " + ssh.LoginGraceTime + "\n")
	if ssh.ClientAliveInterval <= 0 {
		ssh.ClientAliveInterval = 300
	}
	if ssh.ClientAliveCountMax <= 0 {
		ssh.ClientAliveCountMax = 2
	}
	b.WriteString(fmt.Sprintf("ClientAliveInterval %d\n", ssh.ClientAliveInterval))
	b.WriteString(fmt.Sprintf("ClientAliveCountMax %d\n", ssh.ClientAliveCountMax))
	if ssh.DisableEmptyPasswords {
		b.WriteString("PermitEmptyPasswords no\n")
	}
	if ssh.DisableChallengeResponse {
		b.WriteString("KbdInteractiveAuthentication no\n")
		b.WriteString("ChallengeResponseAuthentication no\n")
	}
	b.WriteString("# END banhack233 managed\n")
	return b.String()
}

func replaceManagedBlock(original, block string) string {
	start := strings.Index(original, "# BEGIN banhack233 managed")
	end := strings.Index(original, "# END banhack233 managed")
	if start >= 0 && end > start {
		end += len("# END banhack233 managed")
		next := original[:start] + strings.TrimRight(block, "\n") + original[end:]
		return strings.TrimRight(next, "\n") + "\n"
	}
	return strings.TrimRight(original, "\n") + "\n\n" + block
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
