package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestWebhookBodyFormats(t *testing.T) {
	tests := []struct {
		format string
		key    string
	}{
		{format: "json", key: "text"},
		{format: "discord", key: "content"},
		{format: "slack", key: "text"},
	}
	for _, tt := range tests {
		body, contentType, err := webhookBody(tt.format, "alert", "")
		if err != nil {
			t.Fatalf("webhookBody(%q) error: %v", tt.format, err)
		}
		if contentType != "application/json" {
			t.Fatalf("webhookBody(%q) content type=%q", tt.format, contentType)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("webhookBody(%q) json error: %v", tt.format, err)
		}
		if payload[tt.key] != "alert" {
			t.Fatalf("webhookBody(%q)[%q]=%q", tt.format, tt.key, payload[tt.key])
		}
	}
}

func TestWebhookBodyFeishu(t *testing.T) {
	body, contentType, err := webhookBody("feishu", "alert", "")
	if err != nil {
		t.Fatalf("webhookBody(feishu) error: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type=%q", contentType)
	}
	var payload struct {
		MessageType string `json:"msg_type"`
		Content     struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json error: %v", err)
	}
	if payload.MessageType != "text" || payload.Content.Text != "alert" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestWebhookBodyFeishuWithSecret(t *testing.T) {
	const secret = "test-secret"
	body, _, err := webhookBody("feishu", "alert", secret)
	if err != nil {
		t.Fatalf("webhookBody(feishu, secret) error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json error: %v", err)
	}
	timestamp, ok := payload["timestamp"].(string)
	if !ok || timestamp == "" {
		t.Fatalf("missing timestamp: %+v", payload)
	}
	sign, ok := payload["sign"].(string)
	if !ok || sign == "" {
		t.Fatalf("missing sign: %+v", payload)
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		t.Fatalf("timestamp parse error: %v", err)
	}
	if delta := time.Now().Unix() - ts; delta < 0 || delta > 5 {
		t.Fatalf("timestamp out of range: %s", timestamp)
	}
	if sign != feishuSign(timestamp, secret) {
		t.Fatalf("sign mismatch: got %q want %q", sign, feishuSign(timestamp, secret))
	}
}

func TestFeishuSign(t *testing.T) {
	const timestamp = "1599360473"
	const secret = "demo-secret"
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	mac.Write(nil)
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if got := feishuSign(timestamp, secret); got != want {
		t.Fatalf("feishuSign()=%q want %q", got, want)
	}
}

func TestWebhookBodyTextAndUnknown(t *testing.T) {
	body, contentType, err := webhookBody("text", "alert", "")
	if err != nil {
		t.Fatalf("webhookBody(text) error: %v", err)
	}
	if string(body) != "alert" || contentType != "text/plain; charset=utf-8" {
		t.Fatalf("body=%q contentType=%q", string(body), contentType)
	}
	if _, _, err := webhookBody("unknown", "alert", ""); err == nil {
		t.Fatal("expected unknown format error")
	}
}
