package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/neko233-com/banhack233/internal/config"
	"os"
	"strings"
	"time"
)

type channel struct {
	name string
	send func(context.Context, Alert) error
}

func channels(cfg config.NotificationSet) []channel {
	var out []channel
	if cfg.Console {
		out = append(out, channel{"console", func(_ context.Context, a Alert) error { fmt.Println(a.ConsoleText()); return nil }})
	}
	if cfg.Telegram.Enabled {
		out = append(out, channel{"telegram", func(ctx context.Context, a Alert) error { return sendTelegram(ctx, cfg.Telegram, a) }})
	}
	for _, entry := range []struct {
		name string
		cfg  config.WebhookConfig
	}{{"feishu", cfg.Feishu}, {"discord", cfg.Discord}, {"slack", cfg.Slack}} {
		if !entry.cfg.Enabled {
			continue
		}
		target := config.WebhookTarget{Name: entry.name, URL: entry.cfg.URL, Format: entry.name, Secret: entry.cfg.Secret, LocationLanguage: entry.cfg.LocationLanguage}
		out = append(out, channel{entry.name, func(ctx context.Context, a Alert) error { return sendWebhook(ctx, target, a) }})
	}
	for i, target := range cfg.Webhooks {
		if !target.Enabled {
			continue
		}
		target := target
		out = append(out, channel{fmt.Sprintf("webhook:%d:%s", i, target.Name), func(ctx context.Context, a Alert) error { return sendWebhook(ctx, target, a) }})
	}
	if cfg.Email.Enabled {
		out = append(out, channel{"email", func(ctx context.Context, a Alert) error {
			a = a.WithLocationLanguage(cfg.Email.LocationLanguage)
			if err := sendEmailContext(ctx, cfg.Email, a.EmailSubject(), a.EmailBody(), a.EmailHTML()); err != nil {
				return fmt.Errorf("SMTP delivery failed (check host, TLS and credentials)")
			}
			return nil
		}})
	}
	return out
}

// Channels are independent, bounded by their own deadline; only failed IDs are retried.
func deliver(ctx context.Context, cfg config.NotificationSet, alert Alert, pending []string) ([]string, error) {
	targets := channels(cfg)
	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(targets))
	n := 0
	for _, target := range targets {
		if pending != nil {
			found := false
			for _, id := range pending {
				if id == target.name {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		n++
		go func(c channel) {
			bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			results <- result{c.name, c.send(bounded, alert)}
		}(target)
	}
	var failed []string
	var errs []error
	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			failed = append(failed, r.name)
			errs = append(errs, fmt.Errorf("%s: %w", r.name, r.err))
		}
	}
	return failed, errors.Join(errs...)
}

func sendTelegram(ctx context.Context, cfg config.TelegramConfig, alert Alert) error {
	token := cfg.BotToken
	if cfg.BotTokenEnv != "" {
		token = os.Getenv(cfg.BotTokenEnv)
	}
	if token == "" || strings.ContainsAny(token, "/\r\n ?#") || cfg.ChatID == "" {
		return fmt.Errorf("Telegram requires bot_token (or bot_token_env) and chat_id")
	}
	return telegramRequest(ctx, "https://api.telegram.org/bot"+token+"/sendMessage", cfg, alert)
}

func telegramRequest(ctx context.Context, endpoint string, cfg config.TelegramConfig, alert Alert) error {
	payload := map[string]any{"chat_id": cfg.ChatID, "text": truncate(alert.PlainText(), 3500), "disable_notification": cfg.DisableNotification, "link_preview_options": map[string]bool{"is_disabled": true}}
	if cfg.MessageThreadID > 0 {
		payload["message_thread_id"] = cfg.MessageThreadID
	}
	body, _ := json.Marshal(payload)
	response, err := post(ctx, endpoint, body, "application/json", nil)
	if err != nil {
		return err
	}
	var result struct {
		OK        bool `json:"ok"`
		ErrorCode int  `json:"error_code"`
	}
	if json.Unmarshal(response, &result) != nil {
		return fmt.Errorf("invalid Telegram response")
	}
	if !result.OK {
		return fmt.Errorf("Telegram rejected message (code %d)", result.ErrorCode)
	}
	return nil
}

// Count UTF-16 units, so emoji do not exceed provider character limits.
func truncate(s string, limit int) string {
	n := 0
	for i, r := range s {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if n+width > limit-1 {
			return s[:i] + "…"
		}
		n += width
	}
	return s
}
