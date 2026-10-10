package daemon

import (
	"context"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/lock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func safetyConfig(t *testing.T) (config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.log")
	cfg := config.Default()
	cfg.StatePath = filepath.Join(dir, "state.json")
	cfg.StartAtEnd = false
	cfg.Logging.Enabled = false
	cfg.GeoIP.Enabled = false
	cfg.Malware.Enabled = false
	cfg.Notifications = config.NotificationSet{}
	cfg.Rules[0].LogPaths = []string{path}
	cfg.Rules[0].MaxAttempts = 2
	return cfg, path
}
func TestSuccessfulRetryBeforeBanAndGrace(t *testing.T) {
	cfg, path := safetyConfig(t)
	fail := "Failed password for root from 203.0.113.8 port 40000 ssh2\n"
	content := strings.Repeat(fail, 5) + "Accepted password for root from 203.0.113.8 port 40000 ssh2\n" + strings.Repeat(fail, 3)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ := loadState(cfg.StatePath)
	if len(st.Bans)+len(st.Cooldowns)+len(st.Hits) != 0 {
		t.Fatalf("normal login penalized: %+v", st)
	}
	if !st.SuccessUntil["ssh-auth-failure|203.0.113.8"].After(time.Now()) {
		t.Fatal("grace not persisted")
	}
	_ = os.WriteFile(path, []byte(content+strings.Repeat(fail, 3)), 0600)
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ = loadState(cfg.StatePath)
	if len(st.Cooldowns) != 0 {
		t.Fatal("success grace ignored on next scan")
	}
}
func TestRuleCursorsPartialRecordsAndRotation(t *testing.T) {
	cfg, path := safetyConfig(t)
	second := cfg.Rules[0]
	second.Name = "second"
	cfg.Rules = append(cfg.Rules, second)
	line := "Failed password for root from 203.0.113.9 port 444 ssh2"
	_ = os.WriteFile(path, []byte(line), 0600)
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ := loadState(cfg.StatePath)
	if len(st.Hits) != 0 {
		t.Fatal("partial record counted")
	}
	_ = os.WriteFile(path, []byte(line+"\n"), 0600)
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ = loadState(cfg.StatePath)
	if len(st.Hits) != 2 {
		t.Fatalf("shared log rules lost events: %+v", st.Hits)
	}
	_ = os.Rename(path, path+".1")
	_ = os.WriteFile(path, []byte(line+"\n"+line+"\n"), 0600)
	if err := RunOnce(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ = loadState(cfg.StatePath)
	if len(st.Cooldowns) != 2 {
		t.Fatal("replacement file not read from start")
	}
}
func TestStateSingleWriter(t *testing.T) {
	cfg, _ := safetyConfig(t)
	unlock, err := lock.Acquire(cfg.StatePath + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err := RunOnce(context.Background(), cfg); err == nil {
		t.Fatal("second writer accepted")
	}
}
func TestEventRecordCursorDeduplicatesAndOrders(t *testing.T) {
	data := []byte(`<Events><Event><System><EventRecordID>12</EventRecordID></System><EventData><Data>second</Data></EventData></Event><Event><System><EventRecordID>11</EventRecordID></System><EventData><Data>first</Data></EventData><RenderingInfo><Message>first</Message></RenderingInfo></Event><Event><System><EventRecordID>12</EventRecordID></System><EventData><Data>duplicate</Data></EventData></Event></Events>`)
	lines, cursor, err := parseWindowsEvents(data, 10)
	if err != nil || cursor != 12 || len(lines) != 2 || lines[0] != "first" {
		t.Fatalf("%v %d %v", lines, cursor, err)
	}
	lines, cursor, err = parseWindowsEvents(data, cursor)
	if err != nil || len(lines) != 0 || cursor != 12 {
		t.Fatal("events replayed")
	}
}
