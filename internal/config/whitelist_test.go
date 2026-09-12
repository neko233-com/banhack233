package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/neko233-com/banhack233/internal/geoip"
)

func TestIsIgnoredIP(t *testing.T) {
	ignore := []string{" 192.0.2.3 ", "198.51.100.0/24", "2001:db8::/32", "::1"}
	for _, tc := range []struct {
		ip   string
		want bool
	}{
		{"192.0.2.3", true}, {"192.0.2.30", false}, {"198.51.100.255", true},
		{"198.51.101.1", false}, {"2001:db8:1::2", true}, {"2001:db9::1", false},
		{"0:0:0:0:0:0:0:1", true}, {"::ffff:192.0.2.3", true}, {"garbage", false},
	} {
		if got := IsIgnoredIP(ignore, tc.ip); got != tc.want {
			t.Errorf("%s: %t want %t", tc.ip, got, tc.want)
		}
	}
}

func TestAddIgnoreIPsPreservesConfigAndRejectsInvalidInput(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"ignore_ips":["127.0.0.1"],"dry_run":false,"future_option":{"value":7},"notifications":{"feishu":{"secret":"test-placeholder"}}}`)
	if err := os.WriteFile(p, original, 0o600); err != nil {
		t.Fatal(err)
	}
	ips, err := AddIgnoreIPs(p, []string{"192.0.2.3", "192.0.2.3", "198.51.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 3 {
		t.Fatalf("ips=%v", ips)
	}
	updated, _ := os.ReadFile(p)
	var before, after map[string]any
	_ = json.Unmarshal(original, &before)
	_ = json.Unmarshal(updated, &after)
	delete(before, "ignore_ips")
	delete(after, "ignore_ips")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unrelated fields changed")
	}
	if _, err := AddIgnoreIPs(p, []string{"203.0.113.1", "not-an-ip"}); err == nil {
		t.Fatal("expected invalid IP error")
	}
	still, _ := os.ReadFile(p)
	if string(still) != string(updated) {
		t.Fatal("failed update modified file")
	}
}

func TestNormalizeRejectsInvalidWhitelist(t *testing.T) {
	cfg := Default()
	cfg.IgnoreIPs = []string{"192.0.2.1/99"}
	if err := cfg.Normalize(); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
}

func TestMatchRegion(t *testing.T) {
	rules := &RegionRules{MaxAttempts: map[string]int{"广州": 100, "Guangzhou": 100}}
	if max, ok := MatchRegion(rules, geoip.Location{Country: "中国", Region: "广东省", City: "广州市"}); !ok || max != 100 {
		t.Fatalf("max=%d ok=%v", max, ok)
	}
	if max, ok := MatchRegion(rules, geoip.Location{Country: "China", Region: "Guangdong", City: "Guangzhou"}); !ok || max != 100 {
		t.Fatalf("max=%d ok=%v", max, ok)
	}
	if _, ok := MatchRegion(rules, geoip.Location{Country: "中国", Region: "北京市", City: "北京市"}); ok {
		t.Fatal("unexpected region match")
	}
	if _, ok := MatchRegion(nil, geoip.Location{City: "广州市"}); ok {
		t.Fatal("nil rules matched")
	}
}

func TestNormalizeRegionRules(t *testing.T) {
	cfg := Default()
	cfg.Rules[0].RegionRules = &RegionRules{MaxAttempts: map[string]int{" 广州 ": 100, "": 1, "上海": 0}}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Rules[0].RegionRules.MaxAttempts["广州"]; got != 100 {
		t.Fatalf("广州=%d", got)
	}
	if len(cfg.Rules[0].RegionRules.MaxAttempts) != 1 {
		t.Fatalf("rules=%v", cfg.Rules[0].RegionRules.MaxAttempts)
	}
	cfg.Rules[0].RegionRules = &RegionRules{MaxAttempts: map[string]int{"": 1}}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.Rules[0].RegionRules != nil {
		t.Fatal("empty region rules should be dropped")
	}
}
