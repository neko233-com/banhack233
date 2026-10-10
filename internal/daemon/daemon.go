package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/applog"
	"github.com/neko233-com/banhack233/internal/audit"
	"github.com/neko233-com/banhack233/internal/ban"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/geoip"
	"github.com/neko233-com/banhack233/internal/lock"
	"github.com/neko233-com/banhack233/internal/notify"
)

// hostMacro 是 fail2ban 风格的 <HOST> 宏：展开为带 (?P<ip>...) 命名组的
// IPv4|IPv6 字面地址（fail2ban 写法 `from <HOST>` 可直接使用）。
// 同一条 pattern 内只应出现一次 <HOST>（重复命名组会导致编译失败）。
const hostMacro = `(?P<ip>\S+)`

func Run(ctx context.Context, cfg config.Config) error {
	unlock, err := lock.Acquire(cfg.StatePath + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	dispatcher := notify.NewDispatcher(cfg.Notifications, cfg.GeoIP)
	defer dispatcher.Close()
	logger, err := applog.New(cfg.Logging)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(cfg.Interval.Duration)
	defer ticker.Stop()
	runCycle := func() error {
		return runOnce(ctx, cfg, dispatcher, logger)
	}
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-t.C:
				if err := dispatcher.FlushIfDue(workerCtx); err != nil {
					logger.Error(time.Now(), err.Error())
				}
			}
		}
	}()
	defer func() { stopWorker(); <-workerDone }()
	if err := runCycle(); err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		logger.Error(time.Now(), err.Error())
	}
	for {
		select {
		case <-ctx.Done():
			return nil // Pending notifications are persisted; next start resumes them.
		case <-ticker.C:
			if err := runCycle(); err != nil {
				fmt.Fprintln(os.Stderr, "scan error:", err)
				logger.Error(time.Now(), err.Error())
			}
		}
	}
}

func RunOnce(ctx context.Context, cfg config.Config) error {
	unlock, err := lock.Acquire(cfg.StatePath + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	dispatcher := notify.NewDispatcher(cfg.Notifications, cfg.GeoIP)
	logger, err := applog.New(cfg.Logging)
	if err != nil {
		return err
	}
	defer dispatcher.Close()
	return errors.Join(runOnce(ctx, cfg, dispatcher, logger), dispatcher.Flush(ctx))
}

func runOnce(ctx context.Context, cfg config.Config, dispatcher *notify.Dispatcher, logger *applog.Logger) (result error) {
	st, err := loadState(cfg.StatePath)
	if err != nil {
		return err
	}
	// Persist completed firewall operations even when a later scan or notification fails.
	defer func() { result = errors.Join(result, saveState(cfg.StatePath, st)) }()
	now := time.Now()
	releaseErr := reconcileBans(cfg, &st, now, func(ip, backend string) error {
		ports := []int{22}
		ports = append(ports, cfg.SSHPorts...)
		for key, recorded := range st.BanPorts {
			_, entryIP, ok := splitBanKey(key)
			if ok && entryIP == ip {
				ports = append(ports, recorded...)
			}
		}
		return ban.RemovePorts(ip, backend, ports)
	})
	if releaseErr != nil {
		logger.Error(now, releaseErr.Error())
	}
	var lookup *geoip.Lookup
	if cfg.GeoIP.Enabled {
		lookup = geoip.New(cfg.GeoIP.DBPath)
		defer func() { _ = lookup.Close() }()
	}
	for _, rule := range cfg.Rules {
		if err := scanRule(ctx, cfg, rule, dispatcher, logger, &st, now, lookup); err != nil {
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
	return releaseErr
}

func scanRule(ctx context.Context, cfg config.Config, rule config.Rule, dispatcher *notify.Dispatcher, logger *applog.Logger, st *state, now time.Time, lookup *geoip.Lookup) error {
	for _, path := range rule.LogPaths {
		cursorKey := rule.Name + "|" + path
		if strings.HasPrefix(path, "eventlog:") {
			cursor, seen := st.Offsets[cursorKey]
			lines, next, err := readWindowsEvents(strings.TrimPrefix(path, "eventlog:"), cursor, cfg.StartAtEnd && !seen)
			if err != nil {
				return err
			}
			if err := scanLines(ctx, cfg, rule, dispatcher, logger, st, now, lines, lookup); err != nil {
				return err
			}
			st.Offsets[cursorKey] = next
			continue
		}
		id, err := fileID(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if st.FileIDs == nil {
			st.FileIDs = map[string]string{}
		}
		if previous := st.FileIDs[cursorKey]; previous != "" && previous != id {
			st.Offsets[cursorKey] = 0
		}
		st.FileIDs[cursorKey] = id
		if _, seen := st.Offsets[cursorKey]; !seen {
			if legacy, ok := st.Offsets[path]; ok {
				st.Offsets[cursorKey] = legacy
			}
		}
		if _, seen := st.Offsets[cursorKey]; !seen && cfg.StartAtEnd {
			offset, err := fileSize(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			st.Offsets[cursorKey] = offset
			continue
		}
		lines, offset, err := readNewLines(path, st.Offsets[cursorKey])
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		st.Offsets[cursorKey] = offset
		if err := scanLines(ctx, cfg, rule, dispatcher, logger, st, now, lines, lookup); err != nil {
			return err
		}
	}
	return nil
}

func scanLines(ctx context.Context, cfg config.Config, rule config.Rule, dispatcher *notify.Dispatcher, logger *applog.Logger, st *state, now time.Time, lines []string, lookup *geoip.Lookup) error {
	matchers, err := compilePatterns(rule.Patterns)
	if err != nil {
		return err
	}
	resetMatchers, err := compilePatterns(rule.ResetPatterns)
	if err != nil {
		return err
	}
	if st.SuccessUntil == nil {
		st.SuccessUntil = map[string]time.Time{}
	}
	for key, until := range st.SuccessUntil {
		if !until.After(now) {
			delete(st.SuccessUntil, key)
		}
	}
	// Preprocess successes in this read batch before applying any firewall action.
	// A normal user's successful retry must not be banned by earlier failures.
	succeeded := map[string]bool{}
	for _, line := range lines {
		ip, _ := matchIPUser(resetMatchers, line)
		if addr, err := netip.ParseAddr(ip); err == nil {
			ip = addr.Unmap().String()
			succeeded[ip] = true
			clearHits(st, rule, ip)
			st.SuccessUntil[rule.Name+"|"+ip] = now.Add(rule.SuccessGrace.Duration)
		}
	}
	for _, line := range lines {
		// 成功登录（reset_patterns）优先：清零该 IP 的失败计数后跳过失败匹配。
		if resetIP, _ := matchIPUser(resetMatchers, line); resetIP != "" {
			if !config.IsIgnoredIP(cfg.IgnoreIPs, resetIP) {
				clearHits(st, rule, resetIP)
			}
			continue
		}
		ip, user := matchIPUser(matchers, line)
		if ip == "" {
			continue
		}
		// 半截匹配/伪 IP 行直接跳过，绝不让一条坏日志打断整轮扫描。
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		ip = addr.Unmap().String()
		if succeeded[ip] || st.SuccessUntil[rule.Name+"|"+ip].After(now) {
			continue
		}
		if config.IsIgnoredIP(cfg.IgnoreIPs, ip) {
			continue
		}
		maxAttempts := rule.MaxAttempts
		if rule.RegionRules != nil && lookup != nil {
			if loc, err := lookup.Lookup(ip); err == nil {
				if max, ok := config.MatchRegion(rule.RegionRules, loc); ok {
					maxAttempts = max
				}
			}
		}
		key := banKey(rule, ip, user)
		if st.Bans[key].After(now) || ((cfg.DryRun || rule.Action == "notify") && st.Cooldowns[key].After(now)) {
			continue
		}
		st.Hits[key] = appendRecent(st.Hits[key], now, rule.FindTime.Duration)
		if len(st.Hits[key]) < maxAttempts {
			continue
		}
		if until, banned := st.Bans[key]; banned && until.After(now) {
			continue
		}
		if cfg.DryRun || rule.Action == "notify" {
			if until := st.Cooldowns[key]; until.After(now) {
				continue
			}
		} else {
			delete(st.Cooldowns, key)
		}
		action, err := ban.ApplyPorts(ip, rule.Action, cfg.DryRun, cfg.SSHPorts)
		if err != nil {
			return err
		}
		if action == "notify" || action == "dry-run" {
			if st.Cooldowns == nil {
				st.Cooldowns = map[string]time.Time{}
			}
			st.Cooldowns[key] = now.Add(rule.BanTime.Duration)
		} else {
			st.Bans[key] = now.Add(rule.BanTime.Duration)
			if st.BanActions == nil {
				st.BanActions = map[string]string{}
			}
			st.BanActions[key] = action
			if st.BanPorts == nil {
				st.BanPorts = map[string][]int{}
			}
			st.BanPorts[key] = append([]int(nil), cfg.SSHPorts...)
		}
		logger.Ban(now, rule.Name, ip, action, len(st.Hits[key]), cfg.DryRun)
		if err := dispatcher.NotifyBan(ctx, notify.Event{Rule: rule.Name, IP: ip, Action: action, Count: len(st.Hits[key]), BanDuration: rule.BanTime.Duration, When: now, DryRun: cfg.DryRun}); err != nil {
			logger.Error(now, err.Error())
		}
	}
	return nil
}

func banKey(rule config.Rule, ip, user string) string {
	if rule.CountByUser && user != "" {
		return rule.Name + "|" + ip + "|" + user
	}
	return rule.Name + "|" + ip
}

// clearHits 删除某 IP 在该规则下的全部失败计数（含 count_by_user 的各用户键）。
func clearHits(st *state, rule config.Rule, ip string) {
	prefix := rule.Name + "|" + ip
	for key := range st.Hits {
		if key == prefix || strings.HasPrefix(key, prefix+"|") {
			delete(st.Hits, key)
		}
	}
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, pattern := range patterns {
		re, err := regexp.Compile(strings.ReplaceAll(pattern, "<HOST>", hostMacro))
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func matchIPUser(matchers []*regexp.Regexp, line string) (ip, user string) {
	for _, re := range matchers {
		m := re.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		for i, name := range re.SubexpNames() {
			if i >= len(m) {
				continue
			}
			switch name {
			case "ip":
				ip = m[i]
			case "user":
				user = m[i]
			}
		}
		if ip != "" {
			return ip, user
		}
		if len(m) > 1 {
			return m[1], user
		}
	}
	return "", ""
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
	reader := bufio.NewReaderSize(file, 64*1024)
	start := offset
	for offset-start < 4*1024*1024 {
		line, err := reader.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			// Bound memory even for malicious oversized records; discard to next newline.
			offset += int64(len(line))
			for err == bufio.ErrBufferFull {
				line, err = reader.ReadSlice('\n')
				offset += int64(len(line))
			}
			if err != nil && err != io.EOF {
				return lines, offset, err
			}
			continue
		}
		if err == io.EOF {
			break
		} // Keep partial records for the next poll.
		if err != nil {
			return lines, offset, err
		}
		offset += int64(len(line))
		if len(line) <= 64*1024 && strings.TrimSpace(string(line)) != "" {
			lines = append(lines, strings.TrimSpace(string(line)))
		}
	}
	return lines, offset, nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
