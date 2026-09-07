package daemon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

// reconcileBans releases expired or newly whitelisted automatic bans before
// reading logs. Shared firewall entries stay until every active rule releases them.
func reconcileBans(cfg config.Config, st *state, now time.Time, remove func(string, string) error) error {
	rules := make(map[string]config.Rule, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		rules[rule.Name] = rule
	}
	type entry struct{ key, ip, backend string }
	var releases []entry
	active := map[string]bool{}
	for key, until := range st.Bans {
		idx := strings.LastIndex(key, "|")
		if idx < 0 {
			continue
		}
		ip := key[idx+1:]
		rule, exists := rules[key[:idx]]
		backend := st.BanActions[key]
		if backend == "" {
			// Pre-upgrade state did not record the firewall backend.
			backend = rule.Action
			if backend == "" || backend == "notify" {
				backend = "auto"
			}
		}
		ignored := config.IsIgnoredIP(cfg.IgnoreIPs, ip)
		release := ignored || !until.After(now) || (exists && rule.Action == "notify" && backend != "notify")
		if !release {
			if backend != "notify" && backend != "dry-run" {
				active[ip] = true
			}
			continue
		}
		if cfg.DryRun && backend != "notify" && backend != "dry-run" {
			// dry_run never changes existing firewall rules.
			continue
		}
		switch backend {
		case "auto", "nft", "iptables", "pf", "netsh", "dry-run", "notify":
			releases = append(releases, entry{key, ip, backend})
		default:
			// Custom actions own their reversal; do not guess an undo command.
			active[ip] = true
		}
	}
	var result error
	removed := map[string]bool{}
	for _, item := range releases {
		if active[item.ip] && item.backend != "dry-run" && item.backend != "notify" {
			// Retain every backend's record until no real ban still needs this IP.
			continue
		}
		id := item.backend + "|" + item.ip
		if !active[item.ip] && !removed[id] && item.backend != "dry-run" && item.backend != "notify" {
			if err := remove(item.ip, item.backend); err != nil {
				result = errors.Join(result, fmt.Errorf("release %s: %w", item.key, err))
				continue // Keep state so the next cycle retries the release.
			}
			removed[id] = true
		}
		delete(st.Bans, item.key)
		delete(st.BanActions, item.key)
		delete(st.Hits, item.key)
	}
	for key, until := range st.Cooldowns {
		idx := strings.LastIndex(key, "|")
		if idx >= 0 && (!until.After(now) || config.IsIgnoredIP(cfg.IgnoreIPs, key[idx+1:])) {
			delete(st.Cooldowns, key)
			delete(st.Hits, key)
		}
	}
	for key, hits := range st.Hits {
		idx := strings.LastIndex(key, "|")
		if idx < 0 {
			continue
		}
		rule, exists := rules[key[:idx]]
		if !exists || config.IsIgnoredIP(cfg.IgnoreIPs, key[idx+1:]) {
			delete(st.Hits, key)
			continue
		}
		var recent []time.Time
		for _, hit := range hits {
			if hit.After(now.Add(-rule.FindTime.Duration)) {
				recent = append(recent, hit)
			}
		}
		if len(recent) == 0 {
			delete(st.Hits, key)
		} else {
			st.Hits[key] = recent
		}
	}
	return result
}
