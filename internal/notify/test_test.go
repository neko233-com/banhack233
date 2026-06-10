package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestParseChannelFilterAll(t *testing.T) {
	filter := parseChannelFilter(nil)
	if !filter.match("feishu") || !filter.match("email") {
		t.Fatal("empty filter should match all")
	}
}

func TestParseChannelFilterSelected(t *testing.T) {
	filter := parseChannelFilter([]string{"feishu,email"})
	if !filter.match("feishu") || !filter.match("email") {
		t.Fatal("expected feishu and email")
	}
	if filter.match("discord") {
		t.Fatal("discord should not match")
	}
}

func TestNotifyTestMultipleChannels(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.NotificationSet{
		Console: true,
		Feishu: config.WebhookConfig{
			Enabled: true,
			URL:     server.URL + "/feishu",
		},
		Discord: config.WebhookConfig{
			Enabled: true,
			URL:     server.URL + "/discord",
		},
	}
	_, err := Test(context.Background(), cfg, TestOptions{Message: "hello"})
	if err != nil {
		t.Fatalf("Test() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("webhook calls=%d want 2", len(got))
	}
}

func TestNotifyTestChannelFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.NotificationSet{
		Feishu: config.WebhookConfig{Enabled: true, URL: server.URL},
		Slack:  config.WebhookConfig{Enabled: true, URL: server.URL + "/slack"},
	}
	_, err := Test(context.Background(), cfg, TestOptions{Channels: []string{"feishu"}})
	if err != nil {
		t.Fatalf("Test(feishu only) error: %v", err)
	}
}

func TestNotifyTestNoEnabledChannels(t *testing.T) {
	_, err := Test(context.Background(), config.NotificationSet{}, TestOptions{})
	if err == nil || !strings.Contains(err.Error(), "no enabled notification channels") {
		t.Fatalf("expected no enabled channels error, got %v", err)
	}
}

func TestNotifyTestWebhookFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	cfg := config.NotificationSet{
		Feishu: config.WebhookConfig{Enabled: true, URL: server.URL},
	}
	_, err := Test(context.Background(), cfg, TestOptions{})
	if err == nil || !strings.Contains(err.Error(), "feishu") {
		t.Fatalf("expected feishu failure, got %v", err)
	}
}
