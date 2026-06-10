package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestSendBatchSingleUsesBanAlert(t *testing.T) {
	alert := alertFromEvent(Event{
		Rule:   "ssh-auth-failure",
		IP:     "138.99.80.102",
		Action: "nft",
		Count:  5,
		When:   time.Now(),
	})
	if alert.Title != "banhack233 IP 已封禁" {
		t.Fatalf("title=%q", alert.Title)
	}
}

func TestAlertFromBanBatch(t *testing.T) {
	alert := alertFromBanBatch([]Event{
		{Rule: "ssh-auth-failure", IP: "138.99.80.102", Location: "巴西, 圣保罗", Action: "nft", Count: 5, When: time.Now()},
		{Rule: "ssh-auth-failure", IP: "203.0.113.10", Location: "United States, California", Action: "nft", Count: 6, When: time.Now()},
	})
	if alert.Kind != alertBanBatch || alert.Title != "banhack233 批量封禁" {
		t.Fatalf("alert=%+v", alert)
	}
	text := alert.PlainText()
	if !containsAll(text, "138.99.80.102", "巴西", "203.0.113.10") {
		t.Fatalf("text=%q", text)
	}
}

func TestDispatcherBatchFlush(t *testing.T) {
	d := NewDispatcher(config.NotificationSet{
		Console: true,
		Batch: config.NotifyBatchConfig{
			Enabled:  true,
			Interval: config.Duration{time.Minute},
			MaxItems: 10,
		},
	}, config.GeoIPConfig{Enabled: false})
	defer d.Close()
	ctx := context.Background()
	if err := d.NotifyBan(ctx, Event{Rule: "ssh", IP: "1.2.3.4", Action: "nft", Count: 5, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := d.NotifyBan(ctx, Event{Rule: "ssh", IP: "5.6.7.8", Action: "nft", Count: 5, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := d.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
