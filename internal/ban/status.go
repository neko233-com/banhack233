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

// deleteIPTRules 删除 INPUT 中匹配 ` -s ip <extra...>` 的全部规则（处理旧版本重复插入）。
func deleteIPTRules(ip string, extra ...string) error {
	spec := append([]string{"-s", ip}, extra...)
	for {
		args := append([]string{"-w", "5", "-C", "INPUT"}, spec...)
		out, err := exec.Command("iptables", args...).CombinedOutput()
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("iptables check %s: %s: %w", ip, out, err)
		}
		del := append([]string{"-w", "5", "-D", "INPUT"}, spec...)
		if err := exec.Command("iptables", del...).Run(); err != nil {
			return err
		}
	}
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
		// 移除当前「仅 SSH 端口」规则，以及旧版本插入的全端口规则（可能重复）。
		if err := deleteIPTRules(ip, "-p", "tcp", "--dport", "22", "-j", "DROP"); err != nil {
			return err
		}
		return deleteIPTRules(ip, "-j", "DROP")
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
