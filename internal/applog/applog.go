package applog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

func New(cfg config.LogConfig) (*Logger, error) {
	if !cfg.Enabled {
		return &Logger{w: io.Discard}, nil
	}
	path := cfg.Path
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &Logger{w: &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxSizeMB,
		MaxAge:     cfg.MaxAgeDays,
		MaxBackups: 0,
		LocalTime:  true,
	}}, nil
}

func (l *Logger) Audit(when time.Time, detail string, count int) {
	if detail == "" {
		return
	}
	l.write(when, "audit", fmt.Sprintf("count=%d %s", count, detail))
}

func (l *Logger) Ban(when time.Time, rule, ip, action string, count int, dryRun bool) {
	mode := "ban"
	if dryRun {
		mode = "ban-dry-run"
	}
	if action == "notify" {
		mode = "notify-only"
	}
	l.write(when, mode, fmt.Sprintf("rule=%s ip=%s action=%s count=%d", rule, ip, action, count))
}

func (l *Logger) Error(when time.Time, msg string) {
	l.write(when, "error", msg)
}

func (l *Logger) Close() error {
	if lj, ok := l.w.(*lumberjack.Logger); ok {
		return lj.Close()
	}
	return nil
}

func (l *Logger) write(when time.Time, kind, msg string) {
	if l == nil || l.w == nil {
		return
	}
	if when.IsZero() {
		when = time.Now()
	}
	line := fmt.Sprintf("%s %s %s\n", when.Format(time.RFC3339), kind, msg)
	l.mu.Lock()
	_, _ = io.WriteString(l.w, line)
	l.mu.Unlock()
}
