package notify

import (
	"encoding/json"
	"testing"
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
		body, contentType, err := webhookBody(tt.format, "alert")
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
	body, contentType, err := webhookBody("feishu", "alert")
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

func TestWebhookBodyTextAndUnknown(t *testing.T) {
	body, contentType, err := webhookBody("text", "alert")
	if err != nil {
		t.Fatalf("webhookBody(text) error: %v", err)
	}
	if string(body) != "alert" || contentType != "text/plain; charset=utf-8" {
		t.Fatalf("body=%q contentType=%q", string(body), contentType)
	}
	if _, _, err := webhookBody("unknown", "alert"); err == nil {
		t.Fatal("expected unknown format error")
	}
}
