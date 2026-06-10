package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testAlert() Alert {
	return alertFromTest("alert")
}

func TestWebhookBodyFormats(t *testing.T) {
	alert := testAlert()
	body, contentType, err := webhookBody("json", alert, "")
	if err != nil {
		t.Fatalf("webhookBody(json) error: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type=%q", contentType)
	}
	var jsonPayload map[string]any
	if err := json.Unmarshal(body, &jsonPayload); err != nil {
		t.Fatal(err)
	}
	if jsonPayload["title"] == nil || jsonPayload["fields"] == nil {
		t.Fatalf("payload=%+v", jsonPayload)
	}

	body, _, err = webhookBody("discord", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var discordPayload struct {
		Embeds []map[string]any `json:"embeds"`
	}
	if err := json.Unmarshal(body, &discordPayload); err != nil {
		t.Fatal(err)
	}
	if len(discordPayload.Embeds) != 1 {
		t.Fatalf("embeds=%d", len(discordPayload.Embeds))
	}

	body, _, err = webhookBody("slack", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var slackPayload struct {
		Blocks []map[string]any `json:"blocks"`
	}
	if err := json.Unmarshal(body, &slackPayload); err != nil {
		t.Fatal(err)
	}
	if len(slackPayload.Blocks) == 0 {
		t.Fatal("expected slack blocks")
	}
}

func TestWebhookBodyFeishu(t *testing.T) {
	body, contentType, err := webhookBody("feishu", testAlert(), "")
	if err != nil {
		t.Fatalf("webhookBody(feishu) error: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type=%q", contentType)
	}
	var payload struct {
		MessageType string         `json:"msg_type"`
		Card        map[string]any `json:"card"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json error: %v", err)
	}
	if payload.MessageType != "interactive" {
		t.Fatalf("msg_type=%q", payload.MessageType)
	}
	if payload.Card["schema"] != "2.0" {
		t.Fatalf("card=%+v", payload.Card)
	}
}

func TestWebhookBodyFeishuWithSecret(t *testing.T) {
	const secret = "test-secret"
	body, _, err := webhookBody("feishu", testAlert(), secret)
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
	body, contentType, err := webhookBody("text", testAlert(), "")
	if err != nil {
		t.Fatalf("webhookBody(text) error: %v", err)
	}
	if !strings.Contains(string(body), "banhack233") || contentType != "text/plain; charset=utf-8" {
		t.Fatalf("body=%q contentType=%q", string(body), contentType)
	}
	if _, _, err := webhookBody("unknown", testAlert(), ""); err == nil {
		t.Fatal("expected unknown format error")
	}
}
