package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestReconcileBans(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name, backend, action string
		ignore                []string
		expired, dryRun, fail bool
		wantCalls             int
		wantKept              bool
	}{
		{name: "expiry", backend: "nft", expired: true, wantCalls: 1},
		{name: "legacy expiry", action: "auto", expired: true, wantCalls: 1},
		{name: "whitelisted", backend: "nft", ignore: []string{"192.0.2.0/24"}, wantCalls: 1},
		{name: "active", backend: "nft", wantKept: true},
		{name: "dry run preserves firewall", backend: "nft", expired: true, dryRun: true, wantKept: true},
		{name: "simulated expiry", backend: "dry-run", expired: true},
		{name: "notify cooldown expiry", backend: "notify", action: "notify", expired: true},
		{name: "notify cooldown active", backend: "notify", action: "notify", wantKept: true},
		{name: "switch to notify", backend: "nft", action: "notify", wantCalls: 1},
		{name: "failed removal retries", backend: "nft", expired: true, fail: true, wantCalls: 1, wantKept: true},
		{name: "custom reversal not guessed", backend: "/opt/custom-ban", expired: true, wantKept: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "ssh|192.0.2.3"
			until := now.Add(time.Hour)
			if tc.expired {
				until = now
			}
			st := state{Bans: map[string]time.Time{key: until}, BanActions: map[string]string{key: tc.backend}, Hits: map[string][]time.Time{key: {now}}}
			cfg := config.Config{DryRun: tc.dryRun, IgnoreIPs: tc.ignore, Rules: []config.Rule{{Name: "ssh", Action: tc.action, FindTime: config.Duration{Duration: time.Minute}}}}
			calls := 0
			err := reconcileBans(cfg, &st, now, func(ip, backend string) error {
				calls++
				if ip != "192.0.2.3" {
					t.Fatalf("ip=%s", ip)
				}
				if tc.fail {
					return errors.New("permission denied")
				}
				return nil
			})
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v", err)
			}
			_, kept := st.Bans[key]
			if calls != tc.wantCalls || kept != tc.wantKept {
				t.Fatalf("calls=%d kept=%t", calls, kept)
			}
			if !kept && (len(st.Hits) != 0 || len(st.BanActions) != 0) {
				t.Fatalf("stale state: %+v", st)
			}
		})
	}
}

func TestReconcileSharedIPWaitsForLastRule(t *testing.T) {
	now := time.Now()
	st := state{Bans: map[string]time.Time{"one|192.0.2.3": now, "two|192.0.2.3": now.Add(time.Hour)}, BanActions: map[string]string{"one|192.0.2.3": "nft", "two|192.0.2.3": "nft"}, Hits: map[string][]time.Time{}}
	calls := 0
	remove := func(ip, backend string) error { calls++; return nil }
	if err := reconcileBans(config.Config{}, &st, now, remove); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || len(st.Bans) != 2 {
		t.Fatalf("premature removal: calls=%d bans=%v", calls, st.Bans)
	}
	if err := reconcileBans(config.Config{}, &st, now.Add(time.Hour), remove); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(st.Bans) != 0 {
		t.Fatalf("calls=%d bans=%v", calls, st.Bans)
	}
}

func TestReconcileWhitelistClearsHitsWithoutBan(t *testing.T) {
	st := state{Hits: map[string][]time.Time{"ssh|192.0.2.3": {time.Now()}}}
	if err := reconcileBans(config.Config{IgnoreIPs: []string{"192.0.2.3"}}, &st, time.Now(), func(string, string) error { t.Fatal("unexpected firewall call"); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(st.Hits) != 0 {
		t.Fatal("whitelisted hits retained")
	}
}
