package daemon

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/applog"
	"github.com/neko233-com/banhack233/internal/audit"
	"github.com/neko233-com/banhack233/internal/ban"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/notify"
)

func Run(ctx context.Context, cfg config.Config) error {
	dispatcher := notify.NewDispatcher(cfg.Notifications, cfg.GeoIP)
	defer dispatcher.Close()
	logger, err := applog.New(cfg.Logging)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(cfg.Interval.Duration)
	defer ticker.Stop()
	runCycle := func() error {
		if err := dispatcher.FlushIfDue(ctx); err != nil {
			return err
		}
		return runOnce(ctx, cfg, dispatcher, logger)
	}
	if err := runCycle(); err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		logger.Error(time.Now(), err.Error())
	}
	for {
		select {
		case <-ctx.Done():
			if err := dispatcher.Flush(ctx); err != nil {
				return err
			}
			return ctx.Err()
		case <-ticker.C:
			if err := runCycle(); err != nil {
				fmt.Fprintln(os.Stderr, "scan error:", err)
				logger.Error(time.Now(), err.Error())
			}
		}
	}
}

func RunOnce(ctx context.Context, cfg config.Config) error {
	dispatcher := notify.NewDispatcher(cfg.Notifications, cfg.GeoIP)
	logger, err := applog.New(cfg.Logging)
	if err != nil {
		return err
	}
	defer func() {
		_ = dispatcher.Flush(ctx)
		_ = dispatcher.Close()
	}()
	return runOnce(ctx, cfg, dispatcher, logger)
}

func runOnce(ctx context.Context, cfg config.Config, dispatcher *notify.Dispatcher, logger *applog.Logger) error {
	st, err := loadState(cfg.StatePath)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, rule := range cfg.Rules {
		if err := scanRule(ctx, cfg, rule, dispatcher, logger, &st, now); err != nil {
			return err
		}
	}
	if st.LastAudit.IsZero() || now.Sub(st.LastAudit) >= cfg.AuditInterval.Duration {
		findings := audit.Run(cfg)
		if len(findings) > 0 {
			detail := audit.Format(findings)
			logger.Audit(now, detail, len(findings))
			if cfg.Notifications.Audit {
				if err := dispatcher.NotifyAudit(ctx, notify.Event{Rule: "audit", IP: "-", Action: detail, Count: len(findings), When: now, DryRun: cfg.DryRun}); err != nil {
					return err
				}
			}
		}
		st.LastAudit = now
	}
	return saveState(cfg.StatePath, st)
}

func scanRule(ctx context.Context, cfg config.Config, rule config.Rule, dispatcher *notify.Dispatcher, logger *applog.Logger, st *state, now time.Time) error {
	for _, path := range rule.LogPaths {
		if strings.HasPrefix(path, "eventlog:") {
			lines, err := readWindowsEvents(strings.TrimPrefix(path, "eventlog:"))
			if err != nil {
				return err
			}
			if err := scanLines(ctx, cfg, rule, dispatcher, logger, st, now, lines); err != nil {
				return err
			}
			continue
		}
		if _, seen := st.Offsets[path]; !seen && cfg.StartAtEnd {
			offset, err := fileSize(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			st.Offsets[path] = offset
			continue
		}
		lines, offset, err := readNewLines(path, st.Offsets[path])
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		st.Offsets[path] = offset
		if err := scanLines(ctx, cfg, rule, dispatcher, logger, st, now, lines); err != nil {
			return err
		}
	}
	return nil
}

func scanLines(ctx context.Context, cfg config.Config, rule config.Rule, dispatcher *notify.Dispatcher, logger *applog.Logger, st *state, now time.Time, lines []string) error {
	matchers, err := compilePatterns(rule.Patterns)
	if err != nil {
		return err
	}
	for _, line := range lines {
		ip := matchIP(matchers, line)
		if ip == "" {
			continue
		}
		if ignoredIP(cfg.IgnoreIPs, ip) {
			continue
		}
		key := rule.Name + "|" + ip
		st.Hits[key] = appendRecent(st.Hits[key], now, rule.FindTime.Duration)
		if len(st.Hits[key]) < rule.MaxAttempts {
			continue
		}
		if until, banned := st.Bans[key]; banned && until.After(now) {
			continue
		}
		action, err := ban.Apply(ip, rule.Action, cfg.DryRun)
		if err != nil {
			return err
		}
		st.Bans[key] = now.Add(rule.BanTime.Duration)
		logger.Ban(now, rule.Name, ip, action, len(st.Hits[key]), cfg.DryRun)
		if err := dispatcher.NotifyBan(ctx, notify.Event{Rule: rule.Name, IP: ip, Action: action, Count: len(st.Hits[key]), BanDuration: rule.BanTime.Duration, When: now, DryRun: cfg.DryRun}); err != nil {
			return err
		}
	}
	return nil
}

func ignoredIP(ignore []string, ip string) bool {
	for _, item := range ignore {
		if strings.TrimSpace(item) == ip {
			return true
		}
	}
	return false
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func matchIP(matchers []*regexp.Regexp, line string) string {
	for _, re := range matchers {
		m := re.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		for i, name := range re.SubexpNames() {
			if name == "ip" && i < len(m) {
				return m[i]
			}
		}
		if len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func appendRecent(items []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	var out []time.Time
	for _, item := range items {
		if item.After(cutoff) {
			out = append(out, item)
		}
	}
	out = append(out, now)
	return out
}

func readNewLines(path string, offset int64) ([]string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}
	if offset > info.Size() {
		offset = 0
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, offset, err
	}
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, offset, err
	}
	pos, err := file.Seek(0, 1)
	return lines, pos, err
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
