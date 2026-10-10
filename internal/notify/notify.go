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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
)

type Event struct {
	Rule        string
	IP          string
	Location    string
	Action      string
	Count       int
	BanDuration time.Duration
	When        time.Time
	DryRun      bool
}

func Send(ctx context.Context, cfg config.NotificationSet, ev Event) error {
	return sendAlert(ctx, cfg, alertFromEvent(ev))
}

func sendAlert(ctx context.Context, cfg config.NotificationSet, alert Alert) error {
	_, err := deliver(ctx, cfg, alert, nil)
	return err
}

func sendWebhook(ctx context.Context, target config.WebhookTarget, alert Alert) error {
	alert = alert.WithLocationLanguage(target.LocationLanguage)
	body, contentType, err := webhookBody(target.Format, alert, target.Secret)
	if err != nil {
		return err
	}
	endpoint := target.URL
	if strings.EqualFold(target.Format, "discord") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("invalid webhook URL")
		}
		q := u.Query()
		q.Set("wait", "true")
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}
	response, err := post(ctx, endpoint, body, contentType, target.Headers)
	if err != nil {
		return err
	}
	if target.Format == "feishu" || target.Format == "lark" {
		var result struct {
			Code       int `json:"code"`
			StatusCode int `json:"StatusCode"`
		}
		if len(response) > 0 && json.Unmarshal(response, &result) != nil {
			return fmt.Errorf("invalid Feishu/Lark response")
		}
		if result.Code != 0 || result.StatusCode != 0 {
			return fmt.Errorf("Feishu/Lark rejected message (code %d/%d)", result.Code, result.StatusCode)
		}
	}
	return nil
}

type deliveryError struct {
	Status     int
	RetryAfter time.Duration
}

func (e *deliveryError) Error() string { return fmt.Sprintf("notification HTTP status %d", e.Status) }

func post(ctx context.Context, endpoint string, body []byte, contentType string, headers map[string]string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid notification URL")
	}
	req.Header.Set("Content-Type", contentType)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		// net/url errors contain webhook credentials; never log them.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("notification network request failed")
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("reading notification response failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		delay := time.Duration(0)
		if seconds, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64); err == nil {
			delay = time.Duration(seconds * float64(time.Second))
		} else if date, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil {
			delay = time.Until(date)
		}
		var data struct {
			RetryAfter float64 `json:"retry_after"`
			Parameters struct {
				RetryAfter int `json:"retry_after"`
			} `json:"parameters"`
		}
		_ = json.Unmarshal(response, &data)
		if n := time.Duration(data.RetryAfter * float64(time.Second)); n > delay {
			delay = n
		}
		if n := time.Duration(data.Parameters.RetryAfter) * time.Second; n > delay {
			delay = n
		}
		return nil, &deliveryError{Status: resp.StatusCode, RetryAfter: delay}
	}
	return response, nil
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
		body, err := json.Marshal(map[string]any{"embeds": []map[string]any{alert.discordEmbed()}, "allowed_mentions": map[string]any{"parse": []string{}}})
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
