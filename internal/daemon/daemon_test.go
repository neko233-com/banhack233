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
	if len(st.Bans) != 1 {
		t.Fatalf("bans=%d want 1", len(st.Bans))
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
	if len(st.Bans) != 0 {
		t.Fatalf("bans=%d want 0", len(st.Bans))
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
	if len(st.Bans) != 1 {
		t.Fatalf("bans=%d want 1", len(st.Bans))
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
