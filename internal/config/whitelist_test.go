package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
