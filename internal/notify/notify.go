package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	alert := alertFromEvent(ev)
	if cfg.Console {
		fmt.Println(alert.ConsoleText())
	}
	if cfg.Feishu.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "feishu", Enabled: true, URL: cfg.Feishu.URL, Format: "feishu", Secret: cfg.Feishu.Secret}, alert); err != nil {
			return err
		}
	}
	if cfg.Discord.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "discord", Enabled: true, URL: cfg.Discord.URL, Format: "discord"}, alert); err != nil {
			return err
		}
	}
	if cfg.Slack.Enabled {
		if err := sendWebhook(ctx, config.WebhookTarget{Name: "slack", Enabled: true, URL: cfg.Slack.URL, Format: "slack"}, alert); err != nil {
			return err
		}
	}
	for _, target := range cfg.Webhooks {
		if !target.Enabled {
			continue
		}
		if err := sendWebhook(ctx, target, alert); err != nil {
			return err
		}
	}
	if cfg.Email.Enabled {
		if err := SendEmail(cfg.Email, alert.EmailSubject(), alert.EmailBody(), alert.EmailHTML()); err != nil {
			return err
		}
	}
	return nil
}

func sendWebhook(ctx context.Context, target config.WebhookTarget, alert Alert) error {
	body, contentType, err := webhookBody(target.Format, alert, target.Secret)
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

func feishuSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	mac.Write(nil)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func webhookBody(format string, alert Alert, secret string) ([]byte, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return []byte(alert.PlainText()), "text/plain; charset=utf-8", nil
	case "json":
		body, err := json.Marshal(alert.JSONPayload())
		return body, "application/json", err
	case "discord":
		body, err := json.Marshal(map[string]any{"embeds": []map[string]any{alert.discordEmbed()}})
		return body, "application/json", err
	case "slack":
		body, err := json.Marshal(map[string]any{
			"text":   alert.Title,
			"blocks": alert.slackBlocks(),
		})
		return body, "application/json", err
	case "feishu", "lark":
		payload := map[string]any{
			"msg_type": "interactive",
			"card":     alert.feishuCard(),
		}
		applyFeishuSign(payload, secret)
		body, err := json.Marshal(payload)
		return body, "application/json", err
	default:
		return nil, "", fmt.Errorf("unknown webhook format %q", format)
	}
}
