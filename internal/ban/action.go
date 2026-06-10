package ban

import (
	"fmt"
	"os/exec"
	"runtime"
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
			return "nft", exec.Command("nft", "add", "element", "inet", "banhack233", "blocked", "{", ip, "}").Run()
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
