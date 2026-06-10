package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	Interval      Duration        `json:"interval"`
	AuditInterval Duration        `json:"audit_interval"`
	StatePath     string          `json:"state_path"`
	DryRun        bool            `json:"dry_run"`
	StartAtEnd    bool            `json:"start_at_end"`
	IgnoreIPs     []string        `json:"ignore_ips"`
	Rules         []Rule          `json:"rules"`
	Hardening     Hardening       `json:"hardening"`
	Malware       MalwareConfig   `json:"malware"`
	Notifications NotificationSet `json:"notifications"`
}

type Rule struct {
	Name        string   `json:"name"`
	LogPaths    []string `json:"log_paths"`
	Patterns    []string `json:"patterns"`
	MaxAttempts int      `json:"max_attempts"`
	FindTime    Duration `json:"find_time"`
	BanTime     Duration `json:"ban_time"`
	Action      string   `json:"action"`
}

type NotificationSet struct {
	Feishu   WebhookConfig   `json:"feishu"`
	Discord  WebhookConfig   `json:"discord"`
	Slack    WebhookConfig   `json:"slack"`
	Webhooks []WebhookTarget `json:"webhooks"`
	Email    EmailConfig     `json:"email"`
	Console  bool            `json:"console"`
}

type Hardening struct {
	SSH SSHHardening `json:"ssh"`
}

type MalwareConfig struct {
	Enabled    bool   `json:"enabled"`
	DirectKill bool   `json:"direct_kill"`
	ReportDir  string `json:"report_dir"`
	ReportKeep int    `json:"report_keep"`
}

type SSHHardening struct {
	Enabled                  bool     `json:"enabled"`
	PasswordAuthentication   bool     `json:"password_authentication"`
	PermitRootLogin          string   `json:"permit_root_login"`
	AllowedUsers             []string `json:"allowed_users"`
	MaxAuthTries             int      `json:"max_auth_tries"`
	LoginGraceTime           string   `json:"login_grace_time"`
	ClientAliveInterval      int      `json:"client_alive_interval"`
	ClientAliveCountMax      int      `json:"client_alive_count_max"`
	DisableEmptyPasswords    bool     `json:"disable_empty_passwords"`
	DisableChallengeResponse bool     `json:"disable_challenge_response"`
}

type WebhookConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

type WebhookTarget struct {
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	URL     string            `json:"url"`
	Format  string            `json:"format"`
	Headers map[string]string `json:"headers"`
}

type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	From     string `json:"from"`
	To       string `json:"to"`
	Password string `json:"password"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func DefaultPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "banhack233", "config.json")
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/etc/banhack233/config.json"
	}
	return "/etc/banhack233/config.json"
}

func Default() Config {
	return Config{
		Interval:      Duration{30 * time.Second},
		AuditInterval: Duration{time.Hour},
		StatePath:     defaultStatePath(),
		DryRun:        true,
		StartAtEnd:    true,
		IgnoreIPs:     []string{"127.0.0.1", "::1"},
		Rules: []Rule{
			{
				Name:        "ssh-auth-failure",
				LogPaths:    defaultAuthLogs(),
				Patterns:    []string{`Failed password.*from (?P<ip>\d+\.\d+\.\d+\.\d+)`, `Invalid user .* from (?P<ip>\d+\.\d+\.\d+\.\d+)`},
				MaxAttempts: 5,
				FindTime:    Duration{10 * time.Minute},
				BanTime:     Duration{1 * time.Hour},
				Action:      "auto",
			},
		},
		Hardening: Hardening{SSH: SSHHardening{
			Enabled:                  true,
			PasswordAuthentication:   true,
			PermitRootLogin:          "yes",
			MaxAuthTries:             3,
			LoginGraceTime:           "20s",
			ClientAliveInterval:      300,
			ClientAliveCountMax:      2,
			DisableEmptyPasswords:    true,
			DisableChallengeResponse: true,
		}},
		Malware: MalwareConfig{
			Enabled:    true,
			DirectKill: false,
			ReportDir:  defaultReportPath(),
			ReportKeep: 50,
		},
		Notifications: NotificationSet{Console: true},
	}
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	cfg := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Normalize(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) Normalize() error {
	if c.Interval.Duration <= 0 {
		c.Interval = Duration{30 * time.Second}
	}
	if c.AuditInterval.Duration <= 0 {
		c.AuditInterval = Duration{time.Hour}
	}
	if strings.TrimSpace(c.StatePath) == "" {
		c.StatePath = defaultStatePath()
	}
	if strings.TrimSpace(c.Malware.ReportDir) == "" {
		c.Malware.ReportDir = defaultReportPath()
	}
	if c.Malware.ReportKeep <= 0 {
		c.Malware.ReportKeep = 50
	}
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Name == "" {
			r.Name = "rule"
		}
		if r.MaxAttempts <= 0 {
			r.MaxAttempts = 5
		}
		if r.FindTime.Duration <= 0 {
			r.FindTime = Duration{10 * time.Minute}
		}
		if r.BanTime.Duration <= 0 {
			r.BanTime = Duration{1 * time.Hour}
		}
		if r.Action == "" {
			r.Action = "auto"
		}
	}
	return nil
}

func defaultReportPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "banhack233", "reports")
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/var/banhack233/reports"
	}
	return "/var/lib/banhack233/reports"
}

func defaultStatePath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "banhack233", "state.json")
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/var/banhack233/state.json"
	}
	return "/var/lib/banhack233/state.json"
}

func defaultAuthLogs() []string {
	if runtime.GOOS == "windows" {
		return []string{"eventlog:OpenSSH/Operational"}
	}
	if _, err := os.Stat("/var/log/auth.log"); err == nil {
		return []string{"/var/log/auth.log"}
	}
	return []string{"/var/log/secure"}
}
