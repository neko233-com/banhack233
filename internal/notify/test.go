package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/neko233-com/banhack233/internal/config"
)

type TestOptions struct {
	Channels []string
	Message  string
}

type TestResult struct {
	Channel string
	Skipped bool
	Reason  string
	Err     error
}

func Test(ctx context.Context, cfg config.NotificationSet, opts TestOptions) ([]TestResult, error) {
	alert := alertFromTest(opts.Message)
	filter := parseChannelFilter(opts.Channels)

	var results []TestResult
	var failures []string

	appendResult := func(channel string, skipped bool, reason string, err error) {
		results = append(results, TestResult{Channel: channel, Skipped: skipped, Reason: reason, Err: err})
		if skipped {
			return
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", channel, err))
			return
		}
		fmt.Printf("notify-test %s: ok\n", channel)
	}

	if filter.match("console") {
		if cfg.Console {
			fmt.Println(alert.ConsoleText())
			appendResult("console", false, "", nil)
		} else {
			appendResult("console", true, "disabled", nil)
		}
	}

	if filter.match("feishu", "lark") {
		if !cfg.Feishu.Enabled {
			appendResult("feishu", true, "disabled", nil)
		} else if strings.TrimSpace(cfg.Feishu.URL) == "" {
			appendResult("feishu", true, "missing url", nil)
		} else {
			err := sendWebhook(ctx, config.WebhookTarget{Name: "feishu", Enabled: true, URL: cfg.Feishu.URL, Format: "feishu", Secret: cfg.Feishu.Secret}, alert)
			appendResult("feishu", false, "", err)
		}
	}

	if filter.match("discord") {
		if !cfg.Discord.Enabled {
			appendResult("discord", true, "disabled", nil)
		} else if strings.TrimSpace(cfg.Discord.URL) == "" {
			appendResult("discord", true, "missing url", nil)
		} else {
			err := sendWebhook(ctx, config.WebhookTarget{Name: "discord", Enabled: true, URL: cfg.Discord.URL, Format: "discord"}, alert)
			appendResult("discord", false, "", err)
		}
	}

	if filter.match("slack") {
		if !cfg.Slack.Enabled {
			appendResult("slack", true, "disabled", nil)
		} else if strings.TrimSpace(cfg.Slack.URL) == "" {
			appendResult("slack", true, "missing url", nil)
		} else {
			err := sendWebhook(ctx, config.WebhookTarget{Name: "slack", Enabled: true, URL: cfg.Slack.URL, Format: "slack"}, alert)
			appendResult("slack", false, "", err)
		}
	}

	for _, target := range cfg.Webhooks {
		name := strings.TrimSpace(target.Name)
		if name == "" {
			name = "webhook"
		}
		channel := "webhook:" + name
		if !filter.match(channel, name, "webhook") {
			continue
		}
		if !target.Enabled {
			appendResult(channel, true, "disabled", nil)
			continue
		}
		if strings.TrimSpace(target.URL) == "" {
			appendResult(channel, true, "missing url", nil)
			continue
		}
		err := sendWebhook(ctx, target, alert)
		appendResult(channel, false, "", err)
	}

	if filter.match("email") {
		if !cfg.Email.Enabled {
			appendResult("email", true, "disabled", nil)
		} else {
			err := SendEmail(cfg.Email, alert.EmailSubject(), alert.EmailBody(), alert.EmailHTML())
			appendResult("email", false, "", err)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no matching notification channels; use -channel console,feishu,discord,slack,email,webhook")
	}

	attempted := 0
	for _, result := range results {
		if !result.Skipped {
			attempted++
		} else if len(filter.names) > 0 {
			fmt.Printf("notify-test %s: skipped (%s)\n", result.Channel, result.Reason)
		}
	}
	if attempted == 0 {
		return results, fmt.Errorf("no enabled notification channels matched; enable channels in config or pass -channel")
	}
	if len(failures) > 0 {
		return results, fmt.Errorf("notify-test failed: %s", strings.Join(failures, "; "))
	}
	return results, nil
}

type channelFilter struct {
	all   bool
	names map[string]struct{}
}

func parseChannelFilter(channels []string) channelFilter {
	filter := channelFilter{names: make(map[string]struct{})}
	for _, raw := range channels {
		for _, item := range strings.Split(raw, ",") {
			name := strings.ToLower(strings.TrimSpace(item))
			if name == "" {
				continue
			}
			if name == "all" {
				filter.all = true
				continue
			}
			filter.names[name] = struct{}{}
		}
	}
	if len(filter.names) == 0 {
		filter.all = true
	}
	return filter
}

func (f channelFilter) match(names ...string) bool {
	if f.all {
		return true
	}
	for _, name := range names {
		if _, ok := f.names[strings.ToLower(strings.TrimSpace(name))]; ok {
			return true
		}
	}
	return false
}
