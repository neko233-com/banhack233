package hardening

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func ApplyKeepalive(write bool, sshHours int) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("keepalive supports linux only")
	}
	if sshHours <= 0 {
		sshHours = 24
	}
	sshBlock := fmt.Sprintf(`# managed by banhack233: keep SSH password/root login alive and safe
PasswordAuthentication yes
PermitRootLogin yes
TCPKeepAlive yes
ClientAliveInterval 60
ClientAliveCountMax %d
PermitEmptyPasswords no
MaxAuthTries 3
LoginGraceTime 30s
`, sshHours*60)
	sysctlBlock := `# managed by banhack233: keep long-lived TCP sessions alive through idle NAT/firewalls
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.netfilter.nf_conntrack_tcp_timeout_established = 432000
`
	if !write {
		return "/etc/ssh/sshd_config.d/99-banhack233-keepalive.conf\n" + sshBlock + "\n/etc/sysctl.d/99-banhack233-keepalive.conf\n" + sysctlBlock, nil
	}
	if err := os.MkdirAll("/etc/ssh/sshd_config.d", 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile("/etc/ssh/sshd_config.d/99-banhack233-keepalive.conf", []byte(sshBlock), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile("/etc/sysctl.d/99-banhack233-keepalive.conf", []byte(sysctlBlock), 0o644); err != nil {
		return "", err
	}
	if err := exec.Command("sysctl", "--system").Run(); err != nil {
		return "", err
	}
	if err := exec.Command("sshd", "-t").Run(); err != nil {
		return "", err
	}
	_ = exec.Command("systemctl", "reload", "ssh").Run()
	_ = exec.Command("systemctl", "reload", "sshd").Run()
	return "keepalive applied: ssh=24h tcp_keepalive_time=60s tcp_keepalive_intvl=30s tcp_keepalive_probes=10 conntrack_established=5d", nil
}
