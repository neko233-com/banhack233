package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

type Event struct {
	Rule   string
	IP     string
	Action string
	Count  int
	When   time.Time
	DryRun bool
}

func Send(ctx context.Context, cfg config.NotificationSet, ev Event) error {
	text := fmt.Sprintf("banhack233: rule=%s ip=%s action=%s count=%d dry_run=%t time=%s", ev.Rule, ev.IP, ev.Action, ev.Count, ev.DryRun, ev.When.Format(time.RFC3339))
	if cfg.Console {
		fmt.Println(text)
	}
	if cfg.Feishu.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "feishu", Enabled: true, URL: cfg.Feishu.URL, Format: "feishu"}, text); err != nil {
			return err
		}
	}
	if cfg.Discord.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "discord", Enabled: true, URL: cfg.Discord.URL, Format: "discord"}, text); err != nil {
			return err
		}
	}
	if cfg.Slack.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "slack", Enabled: true, URL: cfg.Slack.URL, Format: "slack"}, text); err != nil {
			return err
		}
	}
	for _, target := range cfg.Webhooks {
		if !target.Enabled {
			continue
		}
		if err := sendWebhook(ctx, target, text); err != nil {
			return err
		}
	}
	if cfg.Email.Enabled {
		if err := SendEmail(cfg.Email, "banhack233 alert", text); err != nil {
			return err
		}
	}
	return nil
}

func sendWebhook(ctx context.Context, target config.WebhookTarget, text string) error {
	body, contentType, err := webhookBody(target.Format, text)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	for key, value := range target.Headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		name := strings.TrimSpace(target.Name)
		if name == "" {
			name = "webhook"
		}
		return fmt.Errorf("%s webhook status %d: %s", name, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func webhookBody(format, text string) ([]byte, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return []byte(text), "text/plain; charset=utf-8", nil
	case "json":
		body, err := json.Marshal(map[string]string{"text": text})
		return body, "application/json", err
	case "discord":
		body, err := json.Marshal(map[string]string{"content": text})
		return body, "application/json", err
	case "slack":
		body, err := json.Marshal(map[string]string{"text": text})
		return body, "application/json", err
	case "feishu", "lark":
		body, err := json.Marshal(map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		})
		return body, "application/json", err
	default:
		return nil, "", fmt.Errorf("unknown webhook format %q", format)
	}
}
