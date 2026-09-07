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

func TestDefaultRuleCountsOnlyPasswordFailures(t *testing.T) {
	matchers, err := compilePatterns(config.Default().Rules[0].Patterns)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Invalid user admin from 192.0.2.3 port 12345",
		"Accepted password for root from 192.0.2.3 port 12345 ssh2",
		"Accepted publickey for root from 192.0.2.3 port 12345 ssh2",
		"Failed publickey for root from 192.0.2.3 port 12345 ssh2",
		"Connection closed by 192.0.2.3 port 12345 [preauth]",
	} {
		if ip := matchIP(matchers, line); ip != "" {
			t.Errorf("unexpected failure: %s", line)
		}
	}
	for _, line := range []string{
		"Failed password for root from 192.0.2.3 port 12345 ssh2",
		"Failed password for invalid user admin from 192.0.2.3 port 12345 ssh2",
	} {
		if ip := matchIP(matchers, line); ip != "192.0.2.3" {
			t.Errorf("missed password failure: %s", line)
		}
	}
}
