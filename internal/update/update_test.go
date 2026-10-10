package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStableVersionAndAssetPolicy(t *testing.T) {
	for _, tc := range []struct {
		next, current string
		want          bool
		invalid       bool
	}{{"v0.2.0", "v0.1.22", true, false}, {"v0.1.22", "v0.2.0", false, false}, {"v1.2.3", "v1.2.3", false, false}, {"v1.2.3-rc1", "v1.2.2", false, true}, {"v01.2.3", "v1.2.2", false, true}, {"v1.2.3", "dev", false, true}} {
		got, err := newer(tc.next, tc.current)
		if got != tc.want || (err != nil) != tc.invalid {
			t.Errorf("%+v: %t %v", tc, got, err)
		}
	}
	r := Release{Tag: "v1.2.3"}
	r.Assets = append(r.Assets, struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{"SHA256SUMS.txt", "https://github.com/" + repository + "/releases/download/v1.2.3/SHA256SUMS.txt"})
	if _, err := releaseAsset(r, "SHA256SUMS.txt"); err != nil {
		t.Fatal(err)
	}
	r.Draft = true
	if _, err := releaseAsset(r, "SHA256SUMS.txt"); err == nil {
		t.Fatal("draft accepted")
	}
	r.Draft = false
	r.Assets[0].URL = "https://other.example/asset"
	if _, err := releaseAsset(r, "SHA256SUMS.txt"); err == nil {
		t.Fatal("foreign asset accepted")
	}
}
func TestChecksumAndBoundedDownload(t *testing.T) {
	name := "binary"
	hash := digest([]byte("verified"))
	if got, err := checksum([]byte(hash+"  "+name), name); err != nil || got != hash {
		t.Fatal(got, err)
	}
	for _, manifest := range []string{"bad binary", hash + " other", hash + " binary\n" + hash + " binary"} {
		if _, err := checksum([]byte(manifest), name); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", 100)) }))
	defer s.Close()
	if _, err := fetch(context.Background(), s.Client(), s.URL, 10); err == nil {
		t.Fatal("oversized asset accepted")
	}
}

type fakeService struct {
	healthCalls, starts, stops int
	failNew                    bool
	cancelNew                  context.CancelFunc
}

func (s *fakeService) Stop(context.Context) error  { s.stops++; return nil }
func (s *fakeService) Start(context.Context) error { s.starts++; return nil }
func (s *fakeService) Healthy(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.healthCalls++
	if s.failNew && s.healthCalls == 2 {
		if s.cancelNew != nil {
			s.cancelNew()
		}
		return fmt.Errorf("new process exited")
	}
	return nil
}
func TestInstallRollbackPreservesConfigAndState(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "binary")
			cfg := filepath.Join(dir, "config.json")
			state := filepath.Join(dir, "state.json")
			for p, data := range map[string]string{target: "old", cfg: "whitelist and credentials", state: "offsets and bans"} {
				if err := os.WriteFile(p, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			stage, err := writeStage(target, []byte("new"))
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(stage)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &fakeService{failNew: fail, cancelNew: cancel}
			err = install(ctx, Options{Target: target, Config: cfg, Restart: true}, stage, []byte("old"), s)
			if (err != nil) != fail {
				t.Fatal(err)
			}
			if fail && s.healthCalls != 3 {
				t.Fatal("rollback recovery inherited the cancelled update context")
			}
			want := "new"
			if fail {
				want = "old"
			}
			data, _ := os.ReadFile(target)
			if string(data) != want {
				t.Fatal("incorrect installed binary", string(data))
			}
			data, _ = os.ReadFile(cfg)
			if string(data) != "whitelist and credentials" {
				t.Fatal("config modified")
			}
			data, _ = os.ReadFile(state)
			if string(data) != "offsets and bans" {
				t.Fatal("state modified")
			}
		})
	}
}
func TestRollbackRejectsTamperedBackup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "binary")
	_ = os.WriteFile(target, []byte("new"), 0600)
	_ = os.WriteFile(target+".previous", []byte("tampered"), 0600)
	_ = os.WriteFile(target+".update.json", []byte(`{"backup_sha256":"`+digest([]byte("old"))+`"}`), 0600)
	if _, err := rollback(context.Background(), Options{Target: target}); err == nil {
		t.Fatal("tampered backup accepted")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Fatal("target changed before verification")
	}
}
func TestScheduleQuotesPaths(t *testing.T) {
	if psQuote("a'b") != "'a''b'" {
		t.Fatal("PowerShell quote")
	}
	if !strings.Contains(systemdQuote("/srv/100%/$literal"), "100%%/$$literal") {
		t.Fatal("systemd expansion not escaped")
	}
}
