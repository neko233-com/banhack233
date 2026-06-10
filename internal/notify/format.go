package notify

import (
	"fmt"
	"html"
	"strings"
	"time"
)

type alertKind string

const (
	alertBan   alertKind = "ban"
	alertAudit alertKind = "audit"
	alertTest  alertKind = "test"
)

type alertRow struct {
	Label string
	Value string
}

type Alert struct {
	Kind   alertKind
	Title  string
	Rule   string
	IP     string
	Action string
	Count  int
	DryRun bool
	When   time.Time
	Detail string
}

func alertFromEvent(ev Event) Alert {
	when := ev.When
	if when.IsZero() {
		when = time.Now()
	}
	if ev.Rule == "audit" {
		return Alert{
			Kind:   alertAudit,
			Title:  "banhack233 巡检告警",
			Rule:   ev.Rule,
			Count:  ev.Count,
			DryRun: ev.DryRun,
			When:   when,
			Detail: ev.Action,
		}
	}
	title := "banhack233 安全告警"
	if ev.DryRun {
		title = "banhack233 模拟封禁"
	}
	return Alert{
		Kind:   alertBan,
		Title:  title,
		Rule:   ev.Rule,
		IP:     ev.IP,
		Action: ev.Action,
		Count:  ev.Count,
		DryRun: ev.DryRun,
		When:   when,
	}
}

func alertFromTest(message string) Alert {
	detail := strings.TrimSpace(message)
	if detail == "" {
		detail = "通知渠道联通性测试成功。"
	}
	return Alert{
		Kind:   alertTest,
		Title:  "banhack233 通知测试",
		Detail: detail,
		When:   time.Now(),
	}
}

func (a Alert) accentColor() string {
	switch a.Kind {
	case alertTest:
		return "#3498DB"
	case alertAudit:
		return "#F39C12"
	case alertBan:
		if a.DryRun {
			return "#F39C12"
		}
		return "#E74C3C"
	default:
		return "#3498DB"
	}
}

func (a Alert) rows() []alertRow {
	switch a.Kind {
	case alertTest:
		return nil
	case alertAudit:
		rows := []alertRow{{Label: "发现项", Value: fmt.Sprint(a.Count)}}
		if a.DryRun {
			rows = append(rows, alertRow{Label: "模式", Value: "dry_run"})
		}
		return rows
	default:
		var rows []alertRow
		if a.Rule != "" {
			rows = append(rows, alertRow{Label: "规则", Value: a.Rule})
		}
		if a.IP != "" {
			rows = append(rows, alertRow{Label: "来源 IP", Value: a.IP})
		}
		if a.Action != "" {
			rows = append(rows, alertRow{Label: "处置", Value: a.Action})
		}
		if a.Count > 0 {
			rows = append(rows, alertRow{Label: "触发次数", Value: fmt.Sprint(a.Count)})
		}
		if a.DryRun {
			rows = append(rows, alertRow{Label: "模式", Value: "dry_run（仅通知，不封禁）"})
		}
		return rows
	}
}

func (a Alert) PlainText() string {
	var b strings.Builder
	b.WriteString(a.Title)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 32))
	b.WriteByte('\n')
	if a.Kind == alertTest {
		fmt.Fprintf(&b, "%s\n", a.Detail)
	} else {
		for _, row := range a.rows() {
			fmt.Fprintf(&b, "%-10s %s\n", row.Label+":", row.Value)
		}
		if a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "" {
			b.WriteByte('\n')
			b.WriteString(a.Detail)
			b.WriteByte('\n')
		}
	}
	b.WriteString(strings.Repeat("─", 32))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "时间: %s", a.When.Format("2006-01-02 15:04:05 MST"))
	return b.String()
}

func (a Alert) ConsoleText() string {
	var b strings.Builder
	line := strings.Repeat("═", 36)
	b.WriteString(line)
	b.WriteByte('\n')
	fmt.Fprintf(&b, " %s\n", a.Title)
	b.WriteString(strings.Repeat("─", 36))
	b.WriteByte('\n')
	if a.Kind == alertTest {
		fmt.Fprintf(&b, " %s\n", a.Detail)
	} else {
		for _, row := range a.rows() {
			fmt.Fprintf(&b, " %-10s %s\n", row.Label+":", row.Value)
		}
		if a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "" {
			b.WriteByte('\n')
			for _, line := range strings.Split(strings.TrimSpace(a.Detail), "\n") {
				fmt.Fprintf(&b, " %s\n", line)
			}
		}
	}
	b.WriteString(line)
	b.WriteByte('\n')
	fmt.Fprintf(&b, " %s\n", a.When.Format("2006-01-02 15:04:05 MST"))
	return b.String()
}

func (a Alert) EmailSubject() string {
	switch a.Kind {
	case alertTest:
		return "banhack233 通知测试"
	case alertAudit:
		return "banhack233 巡检告警"
	default:
		if a.DryRun {
			return "banhack233 模拟封禁告警"
		}
		return "banhack233 安全告警"
	}
}

func (a Alert) EmailBody() string {
	return a.PlainText()
}

func (a Alert) EmailHTML() string {
	var body strings.Builder
	body.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;margin:8px 0;">`)
	if a.Kind == alertTest {
		fmt.Fprintf(&body, `<tr><td style="padding:6px 0;color:#333;line-height:1.6;">%s</td></tr>`, html.EscapeString(a.Detail))
	} else {
		for _, row := range a.rows() {
			fmt.Fprintf(&body, `<tr><td style="padding:8px 0;border-bottom:1px solid #f0f0f0;"><span style="display:inline-block;min-width:88px;color:#666;">%s</span><span style="color:#111;font-weight:600;">%s</span></td></tr>`,
				html.EscapeString(row.Label), html.EscapeString(row.Value))
		}
		if a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "" {
			fmt.Fprintf(&body, `<tr><td style="padding:12px 0;color:#333;white-space:pre-wrap;line-height:1.6;">%s</td></tr>`, html.EscapeString(a.Detail))
		}
	}
	body.WriteString(`</table>`)
	footer := html.EscapeString(a.When.Format("2006-01-02 15:04:05 MST"))
	return fmt.Sprintf(`<!DOCTYPE html><html><body style="margin:0;padding:24px;background:#f4f6f8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;"><table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center"><table role="presentation" width="560" cellpadding="0" cellspacing="0" style="max-width:560px;width:100%%;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 16px rgba(0,0,0,.08);"><tr><td style="background:%s;color:#fff;padding:18px 24px;font-size:18px;font-weight:700;">%s</td></tr><tr><td style="padding:8px 24px 4px;">%s</td></tr><tr><td style="padding:12px 24px 18px;color:#999;font-size:12px;border-top:1px solid #eee;">banhack233 · %s</td></tr></table></td></tr></table></body></html>`,
		a.accentColor(), html.EscapeString(a.Title), body.String(), footer)
}

func (a Alert) JSONPayload() map[string]any {
	fields := make([]map[string]string, 0, len(a.rows()))
	for _, row := range a.rows() {
		fields = append(fields, map[string]string{"label": row.Label, "value": row.Value})
	}
	payload := map[string]any{
		"title":  a.Title,
		"kind":   string(a.Kind),
		"time":   a.When.Format(time.RFC3339),
		"fields": fields,
		"text":   a.PlainText(),
	}
	if a.DryRun {
		payload["dry_run"] = true
	}
	if strings.TrimSpace(a.Detail) != "" {
		payload["detail"] = a.Detail
	}
	return payload
}

func (a Alert) feishuHeaderTemplate() string {
	switch a.Kind {
	case alertTest:
		return "blue"
	case alertAudit:
		return "orange"
	case alertBan:
		if a.DryRun {
			return "orange"
		}
		return "red"
	default:
		return "blue"
	}
}

func (a Alert) feishuMarkdown() string {
	var lines []string
	if a.Kind == alertTest {
		lines = append(lines, a.Detail)
	} else {
		for _, row := range a.rows() {
			if row.Label == "来源 IP" {
				lines = append(lines, fmt.Sprintf("**%s**：`%s`", row.Label, row.Value))
				continue
			}
			lines = append(lines, fmt.Sprintf("**%s**：%s", row.Label, row.Value))
		}
		if a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "" {
			lines = append(lines, "", a.Detail)
		}
	}
	return strings.Join(lines, "\n")
}

func (a Alert) feishuCard() map[string]any {
	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{"update_multi": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": a.Title,
			},
			"template": a.feishuHeaderTemplate(),
		},
		"body": map[string]any{
			"direction": "vertical",
			"padding":   "12px 12px 12px 12px",
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"content":    a.feishuMarkdown(),
					"text_align": "left",
					"text_size":  "normal_v2",
				},
				{"tag": "hr"},
				{
					"tag":     "markdown",
					"content": fmt.Sprintf("<font color='grey'>%s</font>", a.When.Format("2006-01-02 15:04:05 MST")),
				},
			},
		},
	}
}

func (a Alert) discordColor() int {
	switch a.accentColor() {
	case "#E74C3C":
		return 0xE74C3C
	case "#F39C12":
		return 0xF39C12
	default:
		return 0x3498DB
	}
}

func (a Alert) discordFields() []map[string]any {
	field := func(name, value string, inline bool) map[string]any {
		return map[string]any{"name": name, "value": value, "inline": inline}
	}
	if a.Kind == alertTest {
		return nil
	}
	fields := make([]map[string]any, 0, len(a.rows()))
	inline := true
	for _, row := range a.rows() {
		if row.Label == "模式" {
			inline = false
		}
		fields = append(fields, field(row.Label, row.Value, inline))
	}
	return fields
}

func (a Alert) discordEmbed() map[string]any {
	embed := map[string]any{
		"title":       a.Title,
		"color":       a.discordColor(),
		"timestamp":   a.When.UTC().Format(time.RFC3339),
		"author":      map[string]any{"name": "banhack233"},
		"footer":      map[string]any{"text": a.When.Format("2006-01-02 15:04:05 MST")},
	}
	if a.Kind == alertTest || (a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "") {
		embed["description"] = a.Detail
	}
	if fields := a.discordFields(); len(fields) > 0 {
		embed["fields"] = fields
	}
	return embed
}

func (a Alert) slackBlocks() []map[string]any {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": a.Title, "emoji": true},
		},
	}
	if a.Kind == alertTest {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": a.Detail},
		})
	} else {
		var fields []map[string]any
		for _, row := range a.rows() {
			value := row.Value
			if row.Label == "来源 IP" {
				value = "`" + value + "`"
			}
			fields = append(fields, map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", row.Label, value),
			})
		}
		if len(fields) > 0 {
			blocks = append(blocks, map[string]any{"type": "section", "fields": fields})
		}
		if a.Kind == alertAudit && strings.TrimSpace(a.Detail) != "" {
			blocks = append(blocks, map[string]any{
				"type": "section",
				"text": map[string]any{"type": "mrkdwn", "text": a.Detail},
			})
		}
	}
	blocks = append(blocks,
		map[string]any{"type": "divider"},
		map[string]any{
			"type": "context",
			"elements": []map[string]any{
				{"type": "mrkdwn", "text": "banhack233 · " + a.When.Format("2006-01-02 15:04:05 MST")},
			},
		},
	)
	return blocks
}

func applyFeishuSign(payload map[string]any, secret string) {
	if strings.TrimSpace(secret) == "" {
		return
	}
	timestamp := fmt.Sprint(time.Now().Unix())
	payload["timestamp"] = timestamp
	payload["sign"] = feishuSign(timestamp, secret)
}
