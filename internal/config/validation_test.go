package config

import (
	"github.com/neko233-com/banhack233/internal/geoip"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidationFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if _, err := Load(p); err == nil {
		t.Fatal("missing config silently accepted")
	}
	for _, s := range []string{`null`, `{"dry_rnu":false}`, `{} {}`, `{"ssh_ports":[0]}`, `{"rules":[{"name":"same"},{"name":"same"}]}`, `{"rules":[{"patterns":["["]}]}`, `{"notifications":{"telegram":{"enabled":true}}}`} {
		_ = os.WriteFile(p, []byte(s), 0600)
		if _, err := Load(p); err == nil {
			t.Errorf("invalid config accepted: %s", s)
		}
	}
}
func TestOverlappingRegionsAreDeterministic(t *testing.T) {
	r := &RegionRules{MaxAttempts: map[string]int{"China": 10, "Guangzhou": 100}}
	for i := 0; i < 100; i++ {
		if n, ok := MatchRegion(r, geoip.Location{Country: "China", City: "Guangzhou"}); !ok || n != 100 {
			t.Fatal(n)
		}
	}
}

func TestExplicitRulesDoNotInheritDefaultTemplate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	_ = os.WriteFile(p, []byte(`{"rules":[{"name":"custom","patterns":["from <HOST>"]}]}`), 0600)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Rules[0]
	if r.CountByUser || r.SuccessGrace.Duration != 0 || len(r.ResetPatterns) != 0 || r.RegionRules != nil {
		t.Fatalf("inherited default policy: %+v", r)
	}
}
