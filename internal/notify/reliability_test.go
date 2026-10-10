package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/neko233-com/banhack233/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelegramPayloadAndAPIErrors(t *testing.T) {
	var fail atomic.Bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if p["chat_id"] != "-123" || p["message_thread_id"] != float64(42) || p["parse_mode"] != nil {
			t.Errorf("unexpected payload: %v", p)
		}
		if len([]rune(p["text"].(string))) > 3500 {
			t.Error("message too long")
		}
		if fail.Load() {
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"secret-must-not-leak"}`)
		} else {
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	defer s.Close()
	cfg := config.TelegramConfig{ChatID: "-123", MessageThreadID: 42}
	if err := telegramRequest(context.Background(), s.URL, cfg, alertFromTest(strings.Repeat("🙂", 5000))); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	err := telegramRequest(context.Background(), s.URL, cfg, testAlert())
	if err == nil || strings.Contains(err.Error(), "secret-must-not-leak") {
		t.Fatal(err)
	}
}
func TestFailedChannelPersistsAndOnlyItRetries(t *testing.T) {
	var good, bad atomic.Int32
	var recover atomic.Bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good" {
			good.Add(1)
			w.WriteHeader(204)
			return
		}
		bad.Add(1)
		if !recover.Load() {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer s.Close()
	cfg := config.NotificationSet{QueuePath: filepath.Join(t.TempDir(), "queue.json"), Webhooks: []config.WebhookTarget{{Name: "bad", Enabled: true, URL: s.URL + "/bad", Format: "text"}, {Name: "good", Enabled: true, URL: s.URL + "/good", Format: "text"}}}
	d := NewDispatcher(cfg, config.GeoIPConfig{})
	if err := d.NotifyBan(context.Background(), Event{Rule: "ssh", IP: "192.0.2.3", Action: "notify", When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := d.Flush(context.Background()); err == nil {
		t.Fatal("expected delivery failure")
	}
	_ = d.Close()
	d = NewDispatcher(cfg, config.GeoIPConfig{})
	defer d.Close()
	if len(d.queue.Alerts) != 1 || len(d.queue.Alerts[0].Pending) != 1 {
		t.Fatal("failed channel not persisted")
	}
	recover.Store(true)
	d.queue.Alerts[0].Next = time.Time{}
	if err := d.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if good.Load() != 1 || bad.Load() != 2 || len(d.queue.Alerts) != 0 {
		t.Fatalf("duplicate delivery: good=%d bad=%d", good.Load(), bad.Load())
	}
	data, _ := os.ReadFile(cfg.QueuePath)
	if strings.Contains(string(data), s.URL) {
		t.Fatal("endpoint persisted with queue")
	}
}
func TestProviderResponsesAndDeadline(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feishu":
			fmt.Fprint(w, `{"code":19021,"msg":"private"}`)
		case "/rate":
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
		case "/slow":
			<-r.Context().Done()
		}
	}))
	defer s.Close()
	if err := sendWebhook(context.Background(), config.WebhookTarget{URL: s.URL + "/feishu", Format: "feishu"}, testAlert()); err == nil {
		t.Fatal("API error accepted")
	}
	_, err := post(context.Background(), s.URL+"/rate", nil, "text/plain", nil)
	if e, ok := err.(*deliveryError); !ok || e.RetryAfter != 2*time.Minute {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = post(ctx, s.URL+"/slow", nil, "text/plain", nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("deadline ignored")
	}
}
func TestDiscordDisablesMentionsAndBoundsEmbeds(t *testing.T) {
	b, _, err := webhookBody("discord", alertFromTest(strings.Repeat("@everyone ", 1000)), "")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Allowed struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
		Embeds []struct {
			Description string `json:"description"`
		} `json:"embeds"`
	}
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Allowed.Parse) != 0 || len([]rune(p.Embeds[0].Description)) > 3000 {
		t.Fatal("Discord limits not respected")
	}
}
