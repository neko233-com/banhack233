package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestRunOnceBansAfterThreshold(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "auth.log")
	statePath := filepath.Join(dir, "state.json")
	content := "Failed password for root from 1.2.3.4 port 22 ssh2\n" +
		"Invalid user admin from 1.2.3.4 port 22\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Interval:   config.Duration{Duration: time.Second},
		StatePath:  statePath,
		DryRun:     true,
		StartAtEnd: false,
		Rules: []config.Rule{{
			Name:        "ssh",
			LogPaths:    []string{logPath},
			Patterns:    []string{`from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			MaxAttempts: 2,
			FindTime:    config.Duration{Duration: time.Minute},
			BanTime:     config.Duration{Duration: time.Hour},
			Action:      "auto",
		}},
		Notifications: config.NotificationSet{Console: false},
	}
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, err := loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Bans) != 0 || len(st.Cooldowns) != 1 {
		t.Fatalf("bans=%d cooldowns=%d want 0, 1 for dry run", len(st.Bans), len(st.Cooldowns))
	}
}

func TestRunOnceStartAtEndSkipsHistory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "auth.log")
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(logPath, []byte("Failed password for root from 1.2.3.4 port 22 ssh2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Interval:      config.Duration{Duration: time.Second},
		AuditInterval: config.Duration{Duration: time.Hour},
		StatePath:     statePath,
		DryRun:        true,
		StartAtEnd:    true,
		Rules: []config.Rule{{
			Name:        "ssh",
			LogPaths:    []string{logPath},
			Patterns:    []string{`from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			MaxAttempts: 1,
			FindTime:    config.Duration{Duration: time.Minute},
			BanTime:     config.Duration{Duration: time.Hour},
			Action:      "auto",
		}},
	}
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, err := loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Bans) != 0 || len(st.Cooldowns) != 0 {
		t.Fatalf("bans=%d cooldowns=%d want 0, 0", len(st.Bans), len(st.Cooldowns))
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("Failed password for root from 1.2.3.4 port 22 ssh2\n"); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, err = loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Bans) != 0 || len(st.Cooldowns) != 1 {
		t.Fatalf("bans=%d cooldowns=%d want 0, 1 for dry run", len(st.Bans), len(st.Cooldowns))
	}
}

func TestReadNewLinesHandlesTruncate(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, offset, err := readNewLines(logPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, _, err := readNewLines(logPath, offset+100)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "two" {
		t.Fatalf("lines=%v want [two]", lines)
	}
}

func TestNotifyOnlyAndWhitelistAcrossScans(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "auth.log")
	cfg := config.Config{
		StatePath:     filepath.Join(dir, "state.json"),
		IgnoreIPs:     []string{"192.0.2.0/24"},
		AuditInterval: config.Duration{Duration: time.Hour},
		Rules:         []config.Rule{{Name: "ssh", LogPaths: []string{logPath}, Patterns: []string{`from (?P<ip>\d+\.\d+\.\d+\.\d+)`}, MaxAttempts: 2, FindTime: config.Duration{Duration: time.Minute}, BanTime: config.Duration{Duration: time.Hour}, Action: "notify"}},
	}
	lines := "Failed password for root from 192.0.2.3\nFailed password for root from 192.0.2.3\nFailed password for root from 203.0.113.3\nFailed password for root from 203.0.113.3\n"
	if err := os.WriteFile(logPath, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, err := loadState(cfg.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Bans) != 0 || len(st.Cooldowns) != 1 || len(st.Hits) != 1 {
		t.Fatalf("unexpected state: %+v", st)
	}
	until := st.Cooldowns["ssh|203.0.113.3"]
	if err := os.WriteFile(logPath, []byte(lines+lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, err = loadState(cfg.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Cooldowns["ssh|203.0.113.3"] != until || len(st.Bans) != 0 {
		t.Fatalf("cooldown not respected: %+v", st)
	}
}

func TestDefaultRuleMatchesFail2banNormalMode(t *testing.T) {
	matchers, err := compilePatterns(config.Default().Rules[0].Patterns)
	if err != nil {
		t.Fatal(err)
	}
	// 计入：fail2ban normal 模式同样计数的真实认证失败事件。
	for _, tc := range []struct{ line, user string }{
		{"Failed password for root from 192.0.2.3 port 12345 ssh2", "root"},
		{"Failed password for invalid user admin from 192.0.2.3 port 12345 ssh2", "admin"},
		{"Failed publickey for invalid user admin from 192.0.2.3 port 12345 ssh2", "admin"},
		{"error: maximum authentication attempts exceeded for root from 192.0.2.3 port 12345 ssh2 [preauth]", "root"},
		{"Invalid user admin from 192.0.2.3 port 12345", "admin"},
		{"ROOT LOGIN REFUSED FROM 192.0.2.3", ""},
		{"Received disconnect from 192.0.2.3 port 54321:3: Auth fail", ""},
	} {
		ip, user := matchIPUser(matchers, tc.line)
		if ip != "192.0.2.3" || user != tc.user {
			t.Errorf("missed failure: %s ip=%q user=%q want user=%q", tc.line, ip, user, tc.user)
		}
	}
	// 不计入：正常登录、断开、有效用户公钥失败、握手层事件（防误封底线）。
	for _, line := range []string{
		"Accepted password for root from 192.0.2.3 port 12345 ssh2",
		"Accepted publickey for root from 192.0.2.3 port 12345 ssh2",
		"Failed publickey for root from 192.0.2.3 port 12345 ssh2",
		"Connection closed by authenticating user root 192.0.2.3 port 12345 [preauth]",
		"Connection closed by 192.0.2.3 port 12345 [preauth]",
		"kex_exchange_identification: Connection closed by remote host",
		"Did not receive identification string from 192.0.2.3",
	} {
		if ip, _ := matchIPUser(matchers, line); ip != "" {
			t.Errorf("unexpected failure: %s", line)
		}
	}
	// reset_patterns 必须命中成功登录。
	resets, err := compilePatterns(config.Default().Rules[0].ResetPatterns)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Accepted password for root from 192.0.2.3 port 12345 ssh2",
		"Accepted publickey for root from 192.0.2.3 port 12345 ssh2",
		"Accepted keyboard-interactive for root from 192.0.2.3 port 12345 ssh2",
	} {
		if ip, _ := matchIPUser(resets, line); ip != "192.0.2.3" {
			t.Errorf("reset pattern missed: %s", line)
		}
	}
	for _, line := range []string{
		"Failed password for root from 192.0.2.3 port 12345 ssh2",
		"Connection closed by 192.0.2.3 port 12345 [preauth]",
	} {
		if ip, _ := matchIPUser(resets, line); ip != "" {
			t.Errorf("reset pattern false positive: %s", line)
		}
	}
}

func TestAcceptedLoginResetsFailureCount(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "auth.log")
	statePath := filepath.Join(dir, "state.json")
	cfg := config.Config{
		Interval:      config.Duration{Duration: time.Second},
		AuditInterval: config.Duration{Duration: time.Hour},
		StatePath:     statePath,
		DryRun:        true,
		StartAtEnd:    false,
		Rules: []config.Rule{{
			Name:          "ssh",
			LogPaths:      []string{logPath},
			Patterns:      []string{`from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			ResetPatterns: []string{`Accepted \w+ for (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
			MaxAttempts:   3,
			FindTime:      config.Duration{Duration: time.Minute},
			BanTime:       config.Duration{Duration: time.Hour},
			Action:        "auto",
		}},
		Notifications: config.NotificationSet{Console: false},
	}
	writeLog := func(content string) {
		if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	appendLog := func(content string) {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
	}
	runAndLoad := func() state {
		if err := RunOnce(context.Background(), cfg); err != nil {
			t.Fatal(err)
		}
		st, err := loadState(statePath)
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	fails := "Failed password for root from 192.0.2.3 port 22 ssh2\n"

	// 阶段1：2 次失败（阈值 3），只计数不封禁。日志必须追加写入（守护进程按 offset 只读新增尾部）。
	writeLog(fails + fails)
	st := runAndLoad()
	if len(st.Hits) != 1 || len(st.Hits["ssh|192.0.2.3"]) != 2 || len(st.Bans) != 0 {
		t.Fatalf("phase1 hits=%v bans=%d want 2 timestamps, 0 bans", st.Hits, len(st.Bans))
	}

	// 阶段2：成功登录清零该 IP 计数。
	appendLog("Accepted password for root from 192.0.2.3 port 22 ssh2\n")
	st = runAndLoad()
	if len(st.Hits) != 0 {
		t.Fatalf("phase2 hits=%v want empty after accepted login", st.Hits)
	}

	// 阶段3：再 2 次失败 —— 计数应只有这 2 条（未清零则累计 4 条，会触发阈值）。
	appendLog(fails + fails)
	st = runAndLoad()
	if len(st.Hits) != 1 || len(st.Hits["ssh|192.0.2.3"]) != 2 || len(st.Bans) != 0 || len(st.Cooldowns) != 0 {
		t.Fatalf("phase3 hits=%v bans=%d cooldowns=%d want fresh 2, no ban (reset must clear)", st.Hits, len(st.Bans), len(st.Cooldowns))
	}

	// 阶段4：第 3 条新失败达到阈值。
	appendLog(fails)
	st = runAndLoad()
	if len(st.Cooldowns) != 1 {
		t.Fatalf("phase4 cooldowns=%d want 1 after reaching threshold", len(st.Cooldowns))
	}
}

func TestBanKeyCountsByUser(t *testing.T) {
	rule := config.Rule{Name: "ssh", CountByUser: true}
	if got := banKey(rule, "192.0.2.3", "root"); got != "ssh|192.0.2.3|root" {
		t.Fatalf("banKey=%q", got)
	}
	rule.CountByUser = false
	if got := banKey(rule, "192.0.2.3", "root"); got != "ssh|192.0.2.3" {
		t.Fatalf("banKey=%q", got)
	}
}

func TestSplitBanKey(t *testing.T) {
	rule, ip, ok := splitBanKey("ssh|192.0.2.3|root")
	if !ok || rule != "ssh" || ip != "192.0.2.3" {
		t.Fatalf("rule=%q ip=%q ok=%v", rule, ip, ok)
	}
	rule, ip, ok = splitBanKey("ssh|192.0.2.3")
	if !ok || rule != "ssh" || ip != "192.0.2.3" {
		t.Fatalf("rule=%q ip=%q ok=%v", rule, ip, ok)
	}
}
