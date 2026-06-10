package notify

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"

	"github.com/neko233-com/banhack233/internal/config"
)

type SMTPPreset struct {
	Host string
	Port int
}

var smtpPresets = map[string]SMTPPreset{
	"qq.com":         {Host: "smtp.qq.com", Port: 465},
	"163.com":        {Host: "smtp.163.com", Port: 465},
	"126.com":        {Host: "smtp.126.com", Port: 465},
	"gmail.com":      {Host: "smtp.gmail.com", Port: 587},
	"googlemail.com": {Host: "smtp.gmail.com", Port: 587},
	"outlook.com":    {Host: "smtp-mail.outlook.com", Port: 587},
	"hotmail.com":    {Host: "smtp-mail.outlook.com", Port: 587},
	"live.com":       {Host: "smtp-mail.outlook.com", Port: 587},
}

func InferSMTP(addr string) (SMTPPreset, bool) {
	_, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(addr)), "@")
	if !ok {
		return SMTPPreset{}, false
	}
	preset, found := smtpPresets[domain]
	return preset, found
}

func SendEmail(cfg config.EmailConfig, subject, textBody, htmlBody string) error {
	if strings.TrimSpace(cfg.From) == "" || strings.TrimSpace(cfg.To) == "" {
		return fmt.Errorf("email from/to required")
	}
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 {
		preset, ok := InferSMTP(cfg.From)
		if !ok {
			return fmt.Errorf("unknown smtp preset for %s; set smtp_host and smtp_port", cfg.From)
		}
		cfg.SMTPHost = preset.Host
		cfg.SMTPPort = preset.Port
	}
	msg := buildEmailMessage(cfg.From, cfg.To, subject, textBody, htmlBody)
	auth := smtp.PlainAuth("", cfg.From, cfg.Password, cfg.SMTPHost)
	if cfg.SMTPPort == 465 {
		return sendImplicitTLS(cfg, auth, msg)
	}
	return smtp.SendMail(net.JoinHostPort(cfg.SMTPHost, fmt.Sprint(cfg.SMTPPort)), auth, cfg.From, recipients(cfg.To), msg)
}

func buildEmailMessage(from, to, subject, textBody, htmlBody string) []byte {
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + encodedSubject,
		"MIME-Version: 1.0",
	}
	if strings.TrimSpace(htmlBody) == "" {
		headers = append(headers, "Content-Type: text/plain; charset=utf-8")
		return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + textBody)
	}
	boundary := "banhack233-" + fmt.Sprint(len(textBody)+len(htmlBody))
	headers = append(headers, "Content-Type: multipart/alternative; boundary="+boundary)
	var body strings.Builder
	body.WriteString(strings.Join(headers, "\r\n"))
	body.WriteString("\r\n\r\n")
	writePart := func(contentType, content string) {
		body.WriteString("--")
		body.WriteString(boundary)
		body.WriteString("\r\nContent-Type: ")
		body.WriteString(contentType)
		body.WriteString("\r\n\r\n")
		body.WriteString(content)
		body.WriteString("\r\n")
	}
	writePart("text/plain; charset=utf-8", textBody)
	writePart("text/html; charset=utf-8", htmlBody)
	body.WriteString("--")
	body.WriteString(boundary)
	body.WriteString("--\r\n")
	return []byte(body.String())
}

func sendImplicitTLS(cfg config.EmailConfig, auth smtp.Auth, msg []byte) error {
	conn, err := tls.Dial("tcp", net.JoinHostPort(cfg.SMTPHost, fmt.Sprint(cfg.SMTPPort)), &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(cfg.From); err != nil {
		return err
	}
	for _, to := range recipients(cfg.To) {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func recipients(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
