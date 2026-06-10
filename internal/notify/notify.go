package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		if err := sendFeishu(ctx, cfg.Feishu.URL, text); err != nil {
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

func sendFeishu(ctx context.Context, url, text string) error {
	body, err := json.Marshal(map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": text},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("feishu webhook status %d", resp.StatusCode)
	}
	return nil
}
