package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestLoggerAuditAndBan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banhack233.log")
	logger, err := New(config.LogConfig{
		Enabled:    true,
		Path:       path,
		MaxSizeMB:  10,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	when := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	logger.Audit(when, "[info] ssh password enabled", 1)
	logger.Ban(when, "ssh-auth-failure", "1.2.3.4", "nft", 5, false)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "audit count=1") || !strings.Contains(text, "ban rule=ssh-auth-failure") {
		t.Fatalf("log=%q", text)
	}
}

func TestLoggerDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banhack233.log")
	logger, err := New(config.LogConfig{Enabled: false, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	logger.Audit(time.Now(), "x", 1)
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no log file")
	}
}
