package ban

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func List() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("nft"); err == nil {
			out, err := exec.Command("nft", "list", "set", "inet", "banhack233", "blocked").CombinedOutput()
			if err != nil && (strings.Contains(string(out), "No such file") || strings.Contains(string(out), "No such table")) {
				return "", nil
			}
			return string(out), err
		}
		out, err := exec.Command("iptables", "-S", "INPUT").CombinedOutput()
		return string(out), err
	case "windows":
		out, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all").CombinedOutput()
		return string(out), err
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func Unban(ip string) error {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("nft"); err == nil {
			return exec.Command("nft", "delete", "element", "inet", "banhack233", "blocked", "{", ip, "}").Run()
		}
		return exec.Command("iptables", "-D", "INPUT", "-s", ip, "-j", "DROP").Run()
	case "windows":
		return exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=banhack233-"+ip).Run()
	default:
		return fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}
