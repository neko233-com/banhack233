// tools/integration: banhack233 的 Linux 防火墙集成自检（原 dist/integration_linux.py 的 Go 实现）。
//
// 需要 root 与 nft；直接读写本机共享的 nft 表 inet banhack233，
// 运行前请停止 banhack233 守护进程并保证 set blocked 为空。
//
// 用法: integration [-bin /usr/local/bin/banhack233]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
)

var binPath = flag.String("bin", "/usr/local/bin/banhack233", "banhack233 binary to test")

func run(args ...string) string {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL command %v\nerror: %v\n%s\n", args, err, out)
		os.Exit(1)
	}
	return string(out)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func mustf(cond bool, format string, a ...any) {
	if !cond {
		fmt.Fprintf(os.Stderr, "FAIL "+format+"\n", a...)
		os.Exit(1)
	}
}

func readJSON(path string, v any) {
	b, err := os.ReadFile(path)
	must(err)
	must(json.Unmarshal(b, v))
}

func writeJSON(path string, v any) {
	b, err := json.Marshal(v)
	must(err)
	must(os.WriteFile(path, b, 0o644))
}

// blocked 返回 nft set inet banhack233 blocked 中的全部 IP。
func blocked() map[string]bool {
	out := run("nft", "-j", "list", "set", "inet", "banhack233", "blocked")
	var doc struct {
		Nftables []map[string]json.RawMessage `json:"nftables"`
	}
	must(json.Unmarshal([]byte(out), &doc))
	got := map[string]bool{}
	for _, item := range doc.Nftables {
		raw, ok := item["set"]
		if !ok {
			continue
		}
		var set struct {
			Elem []json.RawMessage `json:"elem"`
		}
		must(json.Unmarshal(raw, &set))
		for _, e := range set.Elem {
			var ip string
			if err := json.Unmarshal(e, &ip); err == nil {
				got[ip] = true
				continue
			}
			var obj struct {
				Addr string `json:"addr"`
			}
			if err := json.Unmarshal(e, &obj); err == nil && obj.Addr != "" {
				got[obj.Addr] = true
			}
		}
	}
	return got
}

func expectBlocked(want ...string) {
	got := blocked()
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	mustf(reflect.DeepEqual(got, wantSet), "blocked set = %v, want %v", got, wantSet)
}

func chainRuleCount() int {
	out := run("nft", "-j", "list", "chain", "inet", "banhack233", "input")
	var doc struct {
		Nftables []map[string]json.RawMessage `json:"nftables"`
	}
	must(json.Unmarshal([]byte(out), &doc))
	n := 0
	for _, item := range doc.Nftables {
		if _, ok := item["rule"]; ok {
			n++
		}
	}
	return n
}

func chainRuleText() string {
	return run("nft", "list", "chain", "inet", "banhack233", "input")
}

func nested(m map[string]any, key string) map[string]any {
	sub, _ := m[key].(map[string]any)
	if sub == nil {
		sub = map[string]any{}
		m[key] = sub
	}
	return sub
}

func main() {
	flag.Parse()
	tmp, err := os.MkdirTemp("", "banhack233-test-")
	must(err)
	defer os.RemoveAll(tmp)

	cfgPath := filepath.Join(tmp, "config.json")
	logPath := filepath.Join(tmp, "auth.log")
	statePath := filepath.Join(tmp, "state.json")

	cfg := map[string]any{
		"dry_run":       false,
		"start_at_end":  false,
		"state_path":    statePath,
		"ignore_ips":    []string{"192.0.2.0/24"},
		"audit_interval": "1h",
		"rules": []any{map[string]any{
			"name":        "ssh",
			"log_paths":   []string{logPath},
			"patterns":    []string{`from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			"reset_patterns": []string{`Accepted \w+ for (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			"max_attempts": 5,
			"find_time":    "10m",
			"ban_time":     "1h",
			"action":       "auto",
		}},
		"malware":       map[string]any{"enabled": false},
		"geoip":         map[string]any{"enabled": false},
		"logging":       map[string]any{"enabled": false},
		"notifications": map[string]any{"console": true, "audit": false, "batch": map[string]any{"enabled": false}},
	}
	saveCfg := func() { writeJSON(cfgPath, cfg) }
	rule0 := cfg["rules"].([]any)[0].(map[string]any)
	scan := func() string { return run(*binPath, "test", "-config", cfgPath) }
	loadState := func() map[string]any {
		var st map[string]any
		readJSON(statePath, &st)
		return st
	}
	saveState := func(st map[string]any) { writeJSON(statePath, st) }
	appendFails := func(ip string, count int) {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		must(err)
		defer f.Close()
		for i := 0; i < count; i++ {
			_, err := f.WriteString("Failed password for root from " + ip + " port 22 ssh2\n")
			must(err)
		}
	}

	// 1. CIDR 白名单不计数、真实阈值封禁进 nft set。
	saveCfg()
	appendFails("192.0.2.3", 10)
	appendFails("203.0.113.9", 5)
	scan()
	expectBlocked("203.0.113.9")
	st := loadState()
	mustf(len(nested(st, "hits")) == 1, "hits = %v", st["hits"])
	fmt.Println("PASS CIDR whitelist bypass and real nft threshold ban")

	// 2. 重复封禁不产生重复规则，且规则只匹配 SSH 端口。
	appendFails("203.0.113.10", 5)
	scan()
	mustf(chainRuleCount() == 1, "chain rules = %d, want 1", chainRuleCount())
	mustf(strings.Contains(chainRuleText(), "tcp dport 22"),
		"ban rule is not scoped to ssh port:\n%s", chainRuleText())
	fmt.Println("PASS repeated bans do not duplicate nft rules")

	// 3. whitelist 命令同时解除防火墙与状态封禁。
	run(*binPath, "whitelist", "-config", cfgPath, "203.0.113.9")
	readJSON(cfgPath, &cfg)
	rule0 = cfg["rules"].([]any)[0].(map[string]any)
	scan()
	expectBlocked("203.0.113.10")
	st = loadState()
	mustf(nested(st, "bans")["ssh|203.0.113.9"] == nil, "whitelist left ban in state: %v", st["bans"])
	fmt.Println("PASS whitelist command releases existing firewall and state ban")

	// 4. 过期封禁清理与幂等 unban。
	st = loadState()
	nested(st, "bans")["ssh|203.0.113.10"] = "2000-01-01T00:00:00Z"
	saveState(st)
	scan()
	expectBlocked()
	st = loadState()
	mustf(len(nested(st, "bans")) == 0, "expired bans remain: %v", st["bans"])
	run(*binPath, "unban", "203.0.113.10")
	fmt.Println("PASS expired ban removal and idempotent unban")

	// 5. notify 模式不落防火墙，且告警有冷却。
	rule0["action"] = "notify"
	saveCfg()
	appendFails("203.0.113.11", 5)
	out := scan()
	expectBlocked()
	st = loadState()
	mustf(len(nested(st, "bans")) == 0, "notify mode wrote bans: %v", st["bans"])
	mustf(len(nested(st, "cooldowns")) == 1, "cooldowns = %v", st["cooldowns"])
	mustf(strings.Contains(out, "未封禁") && !strings.Contains(out, "已封禁"), "notify output: %s", out)
	fmt.Println("PASS notify mode has no firewall bans and enforces alert cooldown")

	// 6. 冷却期内重复事件不刷新冷却；切回 auto 不被冷却压制。
	cooldownsBefore := nested(loadState(), "cooldowns")
	appendFails("203.0.113.11", 5)
	out = scan()
	st = loadState()
	mustf(reflect.DeepEqual(cooldownsBefore, nested(st, "cooldowns")), "cooldowns refreshed during cooldown")
	mustf(!strings.Contains(out, "未封禁"), "auto output suppressed by cooldown: %s", out)
	rule0["action"] = "auto"
	saveCfg()
	appendFails("203.0.113.11", 5)
	scan()
	expectBlocked("203.0.113.11")
	fmt.Println("PASS notify to production transition is not suppressed by alert cooldown")

	// 7. 切回 notify 会释放已有自动封禁。
	rule0["action"] = "notify"
	saveCfg()
	scan()
	expectBlocked()
	st = loadState()
	mustf(len(nested(st, "bans")) == 0, "switching to notify left bans: %v", st["bans"])
	fmt.Println("PASS switching to notify releases existing automatic bans")

	// 8. dry_run 不动防火墙；生产模式清理遗留过期封禁。
	run("nft", "add", "element", "inet", "banhack233", "blocked", "{", "203.0.113.12", "}")
	st = loadState()
	nested(st, "bans")["ssh|203.0.113.12"] = "2000-01-01T00:00:00Z"
	delete(st, "ban_actions")
	saveState(st)
	rule0["action"] = "auto"
	cfg["dry_run"] = true
	saveCfg()
	scan()
	expectBlocked("203.0.113.12")
	st = loadState()
	mustf(nested(st, "bans")["ssh|203.0.113.12"] != nil, "dry_run dropped state ban: %v", st["bans"])
	cfg["dry_run"] = false
	saveCfg()
	scan()
	expectBlocked()
	st = loadState()
	mustf(len(nested(st, "bans")) == 0, "legacy expired bans remain: %v", st["bans"])
	fmt.Println("PASS dry_run preserves firewall; production releases legacy expired bans")

	// 9. 成功登录清零失败计数（fail2ban MLFGAINED 对应语义）。
	appendFails("203.0.113.13", 3)
	scan()
	st = loadState()
	mustf(nested(st, "hits")["ssh|203.0.113.13"] != nil, "hits before reset = %v", st["hits"])
	af, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	must(err)
	_, err = af.WriteString("Accepted password for root from 203.0.113.13 port 22 ssh2\n")
	must(err)
	must(af.Close())
	scan()
	st = loadState()
	mustf(nested(st, "hits")["ssh|203.0.113.13"] == nil, "hits after accepted login = %v", st["hits"])
	appendFails("203.0.113.13", 5)
	scan()
	expectBlocked("203.0.113.13")
	st = loadState()
	nested(st, "bans")["ssh|203.0.113.13"] = "2000-01-01T00:00:00Z"
	saveState(st)
	scan()
	expectBlocked()
	st = loadState()
	mustf(len(nested(st, "bans")) == 0, "final bans = %v", st["bans"])
	fmt.Println("PASS accepted login resets failure counter")

	fmt.Println("ALL 9 LINUX INTEGRATION CHECKS PASSED")
}
