package ban

import (
	"fmt"
	"net/netip"
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
	case "darwin":
		out, err := exec.Command("pfctl", "-t", "banhack233", "-T", "show").CombinedOutput()
		if err != nil && strings.Contains(string(out), "No ALTQ support") {
			return string(out), nil
		}
		return string(out), err
	case "windows":
		out, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all").CombinedOutput()
		return string(out), err
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func Unban(ip string) error {
	return Remove(ip, "auto")
}

// Remove is idempotent and uses the backend recorded when the ban was applied.
func Remove(ip, backend string) error {
	if _, err := netip.ParseAddr(ip); err != nil {
		return fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	if backend == "" || backend == "auto" {
		switch runtime.GOOS {
		case "linux":
			backend = "iptables"
			if _, err := exec.LookPath("nft"); err == nil {
				backend = "nft"
			}
		case "darwin":
			backend = "pf"
		case "windows":
			backend = "netsh"
		}
	}
	switch backend {
	case "nft":
		out, err := exec.Command("nft", "delete", "element", "inet", "banhack233", "blocked", "{", ip, "}").CombinedOutput()
		if err != nil && !strings.Contains(string(out), "No such file or directory") && !strings.Contains(string(out), "No such element") {
			return fmt.Errorf("nft unban %s: %s: %w", ip, out, err)
		}
		return nil
	case "iptables":
		// Older versions could insert duplicate rules for the same IP.
		for {
			out, err := exec.Command("iptables", "-w", "5", "-C", "INPUT", "-s", ip, "-j", "DROP").CombinedOutput()
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
				return nil
			}
			if err != nil {
				return fmt.Errorf("iptables check %s: %s: %w", ip, out, err)
			}
			if err := exec.Command("iptables", "-w", "5", "-D", "INPUT", "-s", ip, "-j", "DROP").Run(); err != nil {
				return err
			}
		}
	case "pf":
		return exec.Command("pfctl", "-t", "banhack233", "-T", "delete", ip).Run()
	case "netsh":
		out, err := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=banhack233-"+ip).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "No rules match") {
			return fmt.Errorf("netsh unban %s: %s: %w", ip, out, err)
		}
		return nil
	case "dry-run", "notify":
		return nil
	default:
		return fmt.Errorf("automatic unban unsupported for backend %q", backend)
	}
}
