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
			var out strings.Builder
			missing := 0
			for _, set := range []string{"blocked", "blocked6"} {
				buf, err := exec.Command("nft", "list", "set", "inet", "banhack233", set).CombinedOutput()
				if err != nil && strings.Contains(string(buf), "No such") {
					missing++
					continue
				}
				if err != nil {
					return string(buf), err
				}
				out.Write(buf)
			}
			if missing == 2 {
				return "", nil
			}
			return out.String(), nil
		}
		var out strings.Builder
		for _, bin := range []string{"iptables", "ip6tables"} {
			if _, err := exec.LookPath(bin); err != nil {
				continue
			}
			buf, err := exec.Command(bin, "-S", "INPUT").CombinedOutput()
			if err != nil {
				return string(buf), err
			}
			out.Write(buf)
		}
		return out.String(), nil
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

// deleteIPTRules 删除指定 iptables/ip6tables INPUT 中匹配 ` -s ip <extra...>` 的全部规则（处理旧版本重复插入）。
func deleteIPTRules(bin, ip string, extra ...string) error {
	spec := append([]string{"-s", ip}, extra...)
	for {
		args := append([]string{"-w", "5", "-C", "INPUT"}, spec...)
		out, err := exec.Command(bin, args...).CombinedOutput()
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s check %s: %s: %w", bin, ip, out, err)
		}
		del := append([]string{"-w", "5", "-D", "INPUT"}, spec...)
		if err := exec.Command(bin, del...).Run(); err != nil {
			return err
		}
	}
}

// Remove is idempotent and uses the backend recorded when the ban was applied.
func Remove(ip, backend string) error {
	return RemovePorts(ip, backend, []int{22})
}

func RemovePorts(ip, backend string, ports []int) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	addr = addr.Unmap()
	target := addr.String()
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
		set := "blocked"
		if !addr.Is4() {
			set = "blocked6"
		}
		out, err := exec.Command("nft", "delete", "element", "inet", "banhack233", set, "{", target, "}").CombinedOutput()
		if err != nil && !strings.Contains(string(out), "No such file or directory") && !strings.Contains(string(out), "No such element") {
			return fmt.Errorf("nft unban %s: %s: %w", ip, out, err)
		}
		return nil
	case "iptables":
		bin := "iptables"
		if !addr.Is4() {
			bin = "ip6tables"
		}
		// 移除当前「仅 SSH 端口」规则，以及旧版本插入的全端口规则（可能重复）。
		for _, port := range portStrings(ports) {
			if err := deleteIPTRules(bin, target, "-p", "tcp", "--dport", port, "-j", "DROP"); err != nil {
				return err
			}
		}
		return deleteIPTRules(bin, target, "-j", "DROP")
	case "pf":
		return exec.Command("pfctl", "-t", "banhack233", "-T", "delete", target).Run()
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
