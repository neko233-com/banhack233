package ban

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func Apply(ip, action string, dryRun bool) (string, error) {
	if dryRun {
		return "dry-run", nil
	}
	switch action {
	case "", "auto":
		return applyAuto(ip)
	default:
		return action, exec.Command(action, ip).Run()
	}
}

func applyAuto(ip string) (string, error) {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("nft"); err == nil {
			if err := ensureNFT(); err != nil {
				return "nft", err
			}
			return "nft", runOK(exec.Command("nft", "add", "element", "inet", "banhack233", "blocked", "{", ip, "}"))
		}
		if _, err := exec.LookPath("iptables"); err == nil {
			return "iptables", exec.Command("iptables", "-I", "INPUT", "-s", ip, "-j", "DROP").Run()
		}
		return "", fmt.Errorf("no nft or iptables found")
	case "windows":
		name := "banhack233-" + ip
		return "netsh", exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=block", "remoteip="+ip).Run()
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func ensureNFT() error {
	script := `
add table inet banhack233
add set inet banhack233 blocked { type ipv4_addr; flags timeout; }
add chain inet banhack233 input { type filter hook input priority -100; policy accept; }
add rule inet banhack233 input ip saddr @blocked drop
`
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_ = runOK(exec.Command("nft", strings.Fields(line)...))
	}
	return nil
}

func runOK(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "File exists") || strings.Contains(msg, "already exists") {
			return nil
		}
		if msg != "" {
			return fmt.Errorf("%s: %s", strings.Join(cmd.Args, " "), msg)
		}
		return err
	}
	return nil
}
