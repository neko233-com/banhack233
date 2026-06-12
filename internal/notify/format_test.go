package notify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAlertFromEventBan(t *testing.T) {
	alert := alertFromEvent(Event{
		Rule:        "ssh-auth-failure",
		IP:          "203.0.113.10",
		Location:    "United States, Arizona",
		Action:      "nft",
		Count:       5,
		BanDuration: time.Hour,
		DryRun:      true,
		When:        time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	})
	if alert.Kind != alertBan || alert.Title != "banhack233 模拟封禁" {
		t.Fatalf("alert=%+v", alert)
	}
	body, _, err := webhookBody("feishu", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		MsgType string         `json:"msg_type"`
		Card    map[string]any `json:"card"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MsgType != "interactive" {
		t.Fatalf("msg_type=%q", payload.MsgType)
	}
	if payload.Card["schema"] != "2.0" {
		t.Fatalf("card schema=%v", payload.Card["schema"])
	}
	text := alert.WithLocationLanguage("zh-CN").PlainText()
	if !containsAll(text, "处理方式", "封禁原因", "尝试 SSH 密码登录达到最大 5 次", "封禁持续时间", "1 小时", "归属地中文", "美国") {
		t.Fatalf("text=%q", text)
	}
}

func TestAlertFromEventAudit(t *testing.T) {
	alert := alertFromEvent(Event{
		Rule:   "audit",
		Action: "[warn] ssh root login enabled",
		Count:  2,
		When:   time.Now(),
	})
	if alert.Kind != alertAudit {
		t.Fatalf("kind=%q", alert.Kind)
	}
	body, _, err := webhookBody("discord", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Embeds []map[string]any `json:"embeds"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("embeds=%d", len(payload.Embeds))
	}
}

func TestAlertEmailHTML(t *testing.T) {
	alert := alertFromEvent(Event{
		Rule: "ssh-auth-failure",
		IP:   "203.0.113.10",
		When: time.Now(),
	})
	html := alert.EmailHTML()
	if !strings.Contains(html, "banhack233") || !strings.Contains(html, "203.0.113.10") || !strings.Contains(html, "<html>") {
		t.Fatalf("html=%q", html)
	}
}

func TestAlertJSONPayload(t *testing.T) {
	alert := alertFromTest("hello")
	payload := alert.JSONPayload()
	if payload["kind"] != "test" || payload["detail"] != "hello" {
		t.Fatalf("payload=%+v", payload)
	}
	body, _, err := webhookBody("json", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["title"] == nil {
		t.Fatalf("decoded=%+v", decoded)
	}
}

func TestAlertConsoleText(t *testing.T) {
	text := alertFromTest("ok").ConsoleText()
	if !strings.Contains(text, "═") || !strings.Contains(text, "通知测试") {
		t.Fatalf("text=%q", text)
	}
}

func TestAlertSlackBlocks(t *testing.T) {
	alert := alertFromTest("hello")
	body, _, err := webhookBody("slack", alert, "")
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Blocks []map[string]any `json:"blocks"`
		Text   string           `json:"text"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Blocks) < 2 {
		t.Fatalf("blocks=%d", len(payload.Blocks))
	}
	if payload.Text == "" {
		t.Fatal("expected fallback text")
	}
}
