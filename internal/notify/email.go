package notify

import (
	"crypto/tls"
	"fmt"
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

func SendEmail(cfg config.EmailConfig, subject, body string) error {
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
	msg := "From: " + cfg.From + "\r\n" +
		"To: " + cfg.To + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" + body
	auth := smtp.PlainAuth("", cfg.From, cfg.Password, cfg.SMTPHost)
	if cfg.SMTPPort == 465 {
		return sendImplicitTLS(cfg, auth, []byte(msg))
	}
	return smtp.SendMail(net.JoinHostPort(cfg.SMTPHost, fmt.Sprint(cfg.SMTPPort)), auth, cfg.From, recipients(cfg.To), []byte(msg))
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
