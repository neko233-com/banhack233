package ban

import (
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// 封禁规则只匹配 SSH(22) 端口：全端口 DROP 会在封禁攻击源时误伤
// 共享出口上的 HTTPS 等正常业务访问。IPv4/IPv6 使用独立 set 与规则。
var handlePattern = regexp.MustCompile(`# handle (\d+)`)

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
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	addr = addr.Unmap()
	target := addr.String()
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("nft"); err == nil {
			if err := ensureNFT(); err != nil {
				return "nft", err
			}
			set := "blocked"
			if !addr.Is4() {
				set = "blocked6"
			}
			return "nft", runOK(exec.Command("nft", "add", "element", "inet", "banhack233", set, "{", target, "}"))
		}
		bin := "iptables"
		if !addr.Is4() {
			bin = "ip6tables"
		}
		if _, err := exec.LookPath(bin); err == nil {
			return "iptables", exec.Command(bin, "-I", "INPUT", "-s", target, "-p", "tcp", "--dport", "22", "-j", "DROP").Run()
		}
		return "", fmt.Errorf("no nft or iptables found")
	case "darwin":
		if err := ensurePF(); err != nil {
			return "pf", err
		}
		return "pf", runOK(exec.Command("pfctl", "-t", "banhack233", "-T", "add", target))
	case "windows":
		name := "banhack233-" + ip
		return "netsh", exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=block", "remoteip="+target, "remoteport=22", "protocol=TCP").Run()
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
add set inet banhack233 blocked6 { type ipv6_addr; flags timeout; }
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
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", "banhack233", "input").CombinedOutput()
	if err != nil {
		return fmt.Errorf("list nft chain: %s: %w", out, err)
	}
	plan := planChainRules(string(out))
	for _, handle := range plan.DeleteHandles {
		if err := runOK(exec.Command("nft", "delete", "rule", "inet", "banhack233", "input", "handle", handle)); err != nil {
			return err
		}
	}
	if plan.MissingHandle {
		return fmt.Errorf("nft rule without handle cannot be removed: %s", out)
	}
	if plan.NeedV4 {
		if err := runOK(exec.Command("nft", "add", "rule", "inet", "banhack233", "input", "ip", "saddr", "@blocked", "tcp", "dport", "22", "drop")); err != nil {
			return err
		}
	}
	if plan.NeedV6 {
		if err := runOK(exec.Command("nft", "add", "rule", "inet", "banhack233", "input", "ip6", "saddr", "@blocked6", "tcp", "dport", "22", "drop")); err != nil {
			return err
		}
	}
	return nil
}

type chainRulesPlan struct {
	DeleteHandles []string
	MissingHandle bool
	NeedV4        bool
	NeedV6        bool
}

// planChainRules 解析 `nft -a list chain` 输出：删除旧版全端口规则和重复的
// 当前规则，并判断 v4/v6 两条「只拦 SSH 端口」规则是否缺失。
func planChainRules(listOutput string) chainRulesPlan {
	var plan chainRulesPlan
	seenV4, seenV6 := false, false
	for _, line := range strings.Split(listOutput, "\n") {
		if !strings.Contains(line, "saddr @blocked") {
			continue
		}
		handle := ""
		if m := handlePattern.FindStringSubmatch(line); m != nil {
			handle = m[1]
		}
		isV6 := strings.Contains(line, "@blocked6")
		scoped := strings.Contains(line, "tcp dport 22")
		seen := &seenV4
		if isV6 {
			seen = &seenV6
		}
		if scoped && !*seen {
			*seen = true
			continue
		}
		// 旧版全端口规则、重复的当前规则：都要删除。
		if handle == "" {
			plan.MissingHandle = true
			continue
		}
		plan.DeleteHandles = append(plan.DeleteHandles, handle)
	}
	plan.NeedV4 = !seenV4
	plan.NeedV6 = !seenV6
	return plan
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
