package ban

import (
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
)

func Apply(ip, action string, dryRun bool) (string, error) {
	if _, err := netip.ParseAddr(ip); err != nil {
		return "", fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	if action == "notify" {
		return "notify", nil
	}
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
	case "darwin":
		if err := ensurePF(); err != nil {
			return "pf", err
		}
		return "pf", runOK(exec.Command("pfctl", "-t", "banhack233", "-T", "add", ip))
	case "windows":
		name := "banhack233-" + ip
		return "netsh", exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=block", "remoteip="+ip).Run()
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func ensurePF() error {
	_ = runOK(exec.Command("pfctl", "-E"))
	return runOK(exec.Command("pfctl", "-t", "banhack233", "-T", "show"))
}

func ensureNFT() error {
	script := `
add table inet banhack233
add set inet banhack233 blocked { type ipv4_addr; flags timeout; }
add chain inet banhack233 input { type filter hook input priority -100; policy accept; }
`
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := runOK(exec.Command("nft", strings.Fields(line)...)); err != nil {
			return err
		}
	}
	out, err := exec.Command("nft", "list", "chain", "inet", "banhack233", "input").CombinedOutput()
	if err != nil {
		return fmt.Errorf("list nft chain: %s: %w", out, err)
	}
	if strings.Contains(string(out), "ip saddr @blocked drop") {
		return nil
	}
	return runOK(exec.Command("nft", "add", "rule", "inet", "banhack233", "input", "ip", "saddr", "@blocked", "drop"))
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
