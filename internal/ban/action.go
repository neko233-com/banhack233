package ban

import (
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// 封禁规则只匹配 SSH(22) 端口：全端口 DROP 会在封禁攻击源时误伤
// 共享出口上的 HTTPS 等正常业务访问。IPv4/IPv6 使用独立 set 与规则。
var handlePattern = regexp.MustCompile(`# handle (\d+)`)

func Apply(ip, action string, dryRun bool) (string, error) {
	return ApplyPorts(ip, action, dryRun, []int{22})
}

func ApplyPorts(ip, action string, dryRun bool, ports []int) (string, error) {
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
		return applyAuto(ip, ports)
	default:
		return action, exec.Command(action, ip).Run()
	}
}

func applyAuto(ip string, ports []int) (string, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", fmt.Errorf("invalid IP %q: %w", ip, err)
	}
	addr = addr.Unmap()
	target := addr.String()
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("nft"); err == nil {
			if err := ensureNFTPorts(ports); err != nil {
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
			for _, port := range portStrings(ports) {
				if exec.Command(bin, "-w", "5", "-C", "INPUT", "-s", target, "-p", "tcp", "--dport", port, "-j", "DROP").Run() == nil {
					continue
				}
				if err := exec.Command(bin, "-w", "5", "-I", "INPUT", "-s", target, "-p", "tcp", "--dport", port, "-j", "DROP").Run(); err != nil {
					_ = RemovePorts(ip, "iptables", ports)
					return "iptables", err
				}
			}
			return "iptables", nil
		}
		return "", fmt.Errorf("no nft or iptables found")
	case "darwin":
		if err := ensurePF(); err != nil {
			return "pf", err
		}
		return "pf", runOK(exec.Command("pfctl", "-t", "banhack233", "-T", "add", target))
	case "windows":
		name := "banhack233-" + ip
		_ = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name).Run()
		return "netsh", exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name="+name, "dir=in", "action=block", "remoteip="+target, "localport="+strings.Join(portStrings(ports), ","), "protocol=TCP").Run()
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

func ensurePF() error {
	out, err := exec.Command("pfctl", "-sr").Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "<banhack233>") {
		return fmt.Errorf("PF requires an explicitly configured SSH-only block rule using <banhack233>; no firewall change applied")
	}
	_ = runOK(exec.Command("pfctl", "-E"))
	return runOK(exec.Command("pfctl", "-t", "banhack233", "-T", "show"))
}

func ensureNFT() error {
	return ensureNFTPorts([]int{22})
}

func ensureNFTPorts(ports []int) error {
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
	plan := planChainRulesPorts(string(out), ports)
	for _, handle := range plan.DeleteHandles {
		if err := runOK(exec.Command("nft", "delete", "rule", "inet", "banhack233", "input", "handle", handle)); err != nil {
			return err
		}
	}
	if plan.MissingHandle {
		return fmt.Errorf("nft rule without handle cannot be removed: %s", out)
	}
	if plan.NeedV4 {
		if err := runOK(exec.Command("nft", "add", "rule", "inet", "banhack233", "input", "ip", "saddr", "@blocked", "tcp", "dport", portExpression(ports), "drop")); err != nil {
			return err
		}
	}
	if plan.NeedV6 {
		if err := runOK(exec.Command("nft", "add", "rule", "inet", "banhack233", "input", "ip6", "saddr", "@blocked6", "tcp", "dport", portExpression(ports), "drop")); err != nil {
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
	return planChainRulesPorts(listOutput, []int{22})
}

func planChainRulesPorts(listOutput string, ports []int) chainRulesPlan {
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
		scoped := strings.Contains(line, "tcp dport "+portExpression(ports)+" drop")
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

func portStrings(ports []int) []string {
	if len(ports) == 0 {
		ports = []int{22}
	}
	unique := map[int]bool{}
	for _, port := range ports {
		unique[port] = true
	}
	ordered := []int{}
	for port := range unique {
		ordered = append(ordered, port)
	}
	sort.Ints(ordered)
	result := []string{}
	for _, port := range ordered {
		result = append(result, strconv.Itoa(port))
	}
	return result
}
func portExpression(ports []int) string {
	items := portStrings(ports)
	if len(items) == 1 {
		return items[0]
	}
	return "{ " + strings.Join(items, ", ") + " }"
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
