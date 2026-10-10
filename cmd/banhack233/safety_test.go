package main

import (
	"encoding/json"
	"github.com/neko233-com/banhack233/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProductionAndSafeMigrationPreservePreferences(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.IgnoreIPs = append(cfg.IgnoreIPs, "192.0.2.3")
	cfg.Logging.Enabled = false
	cfg.GeoIP.Enabled = false
	cfg.Notifications.Batch.Enabled = false
	cfg.Notifications.Audit = true
	cfg.Rules[0].Patterns = []string{`Invalid user (?P<user>\S+) from <HOST>`}
	cfg.Rules[0].MaxAttempts = 23
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(p, b, 0600)
	if err := runEnableProduction([]string{"-config", p}); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.DryRun || got.Logging.Enabled || got.GeoIP.Enabled || got.Notifications.Batch.Enabled || !got.Notifications.Audit {
		t.Fatal("preferences overwritten")
	}
	if err := runSafeSSH([]string{"-config", p, "-write"}); err != nil {
		t.Fatal(err)
	}
	got, err = config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.IgnoreIPs, cfg.IgnoreIPs) || got.Rules[0].MaxAttempts != 23 || len(got.Rules[0].Patterns) != 1 || got.Rules[0].Patterns[0] != config.Default().Rules[0].Patterns[0] {
		t.Fatal("migration did not preserve policy")
	}
}
