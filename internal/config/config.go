package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/geoip"
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
	GeoIP         GeoIPConfig     `json:"geoip"`
	Logging       LogConfig       `json:"logging"`
	Notifications NotificationSet `json:"notifications"`
}

type LogConfig struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`
	MaxSizeMB  int    `json:"max_size_mb"`
	MaxAgeDays int    `json:"max_age_days"`
}

type Rule struct {
	Name        string   `json:"name"`
	LogPaths    []string `json:"log_paths"`
	Patterns    []string `json:"patterns"`
	MaxAttempts int      `json:"max_attempts"`
	FindTime    Duration `json:"find_time"`
	BanTime     Duration `json:"ban_time"`
	Action      string   `json:"action"`
	// CountByUser: 按 IP+用户名 计失败次数，避免同一出口 IP 上多用户共享阈值导致误封。
	CountByUser bool `json:"count_by_user,omitempty"`
	// ResetPatterns: 命中后清零该 IP 的全部失败计数（成功登录），
	// 对应 fail2ban 的 MLFGAINED 语义，避免正常登录者被历史失败连坐。
	ResetPatterns []string     `json:"reset_patterns,omitempty"`
	RegionRules   *RegionRules `json:"region_rules,omitempty"`
}

// RegionRules 按 GeoIP 地区覆盖 max_attempts。
// key 匹配 Country/Region/City，大小写不敏感，支持中英文（广州/Guangzhou）。
type RegionRules struct {
	MaxAttempts map[string]int `json:"max_attempts"`
}

type NotificationSet struct {
	Feishu   WebhookConfig     `json:"feishu"`
	Discord  WebhookConfig     `json:"discord"`
	Slack    WebhookConfig     `json:"slack"`
	Webhooks []WebhookTarget   `json:"webhooks"`
	Email    EmailConfig       `json:"email"`
	Console  bool              `json:"console"`
	Audit    bool              `json:"audit"`
	Batch    NotifyBatchConfig `json:"batch"`
}

type GeoIPConfig struct {
	Enabled bool   `json:"enabled"`
	DBPath  string `json:"db_path"`
}

type NotifyBatchConfig struct {
	Enabled  bool     `json:"enabled"`
	Interval Duration `json:"interval"`
	MaxItems int      `json:"max_items"`
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
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url"`
	Secret           string `json:"secret,omitempty"`
	LocationLanguage string `json:"location_language,omitempty"`
}

type WebhookTarget struct {
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	URL              string            `json:"url"`
	Format           string            `json:"format"`
	Secret           string            `json:"secret,omitempty"`
	LocationLanguage string            `json:"location_language,omitempty"`
	Headers          map[string]string `json:"headers"`
}

type EmailConfig struct {
	Enabled          bool   `json:"enabled"`
	From             string `json:"from"`
	To               string `json:"to"`
	Password         string `json:"password"`
	SMTPHost         string `json:"smtp_host"`
	SMTPPort         int    `json:"smtp_port"`
	LocationLanguage string `json:"location_language,omitempty"`
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
				Name: "ssh-auth-failure",
				LogPaths: defaultAuthLogs(),
				// 与 fail2ban normal 模式对齐：覆盖密码/无效用户公钥/超限认证/
				// 裸 Invalid user/ROOT LOGIN REFUSED/Auth fail 断开等真实认证失败。
				Patterns: []string{
					`Failed password for(?: invalid user)? (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`,
					`Failed publickey for invalid user (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`,
					`maximum authentication attempts exceeded for (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`,
					`Invalid user (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`,
					`ROOT LOGIN REFUSED FROM (?P<ip>\d+\.\d+\.\d+\.\d+)`,
					`Received disconnect from (?P<ip>\d+\.\d+\.\d+\.\d+) port \d+:3: Auth fail`,
				},
				// 成功登录清零该 IP 的失败计数（fail2ban MLFGAINED 对应语义）。
				ResetPatterns: []string{
					`Accepted (?:password|publickey|keyboard-interactive) for (?P<user>\S+) from (?P<ip>\d+\.\d+\.\d+\.\d+)`,
				},
				MaxAttempts: 5,
				FindTime:    Duration{10 * time.Minute},
				BanTime:     Duration{1 * time.Hour},
				Action:      "auto",
				CountByUser: true,
				RegionRules: &RegionRules{
					MaxAttempts: map[string]int{
						"广州":       100,
						"Guangzhou": 100,
					},
				},
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
		GeoIP: GeoIPConfig{
			Enabled: true,
			DBPath:  defaultGeoIPPath(),
		},
		Logging: LogConfig{
			Enabled:    true,
			Path:       defaultLogPath(),
			MaxSizeMB:  10,
			MaxAgeDays: 30,
		},
		Notifications: NotificationSet{
			Console: true,
			Feishu:  WebhookConfig{LocationLanguage: "zh-CN"},
			Batch: NotifyBatchConfig{
				Enabled:  true,
				Interval: Duration{60 * time.Second},
				MaxItems: 20,
			},
		},
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
	for i, item := range c.IgnoreIPs {
		item = strings.TrimSpace(item)
		if err := validateIgnoreIP(item); err != nil {
			return err
		}
		c.IgnoreIPs[i] = item
	}
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
	if strings.TrimSpace(c.GeoIP.DBPath) == "" {
		c.GeoIP.DBPath = defaultGeoIPPath()
	}
	if c.Logging.Enabled {
		if strings.TrimSpace(c.Logging.Path) == "" {
			c.Logging.Path = defaultLogPath()
		}
		if c.Logging.MaxSizeMB <= 0 {
			c.Logging.MaxSizeMB = 10
		}
		if c.Logging.MaxAgeDays <= 0 {
			c.Logging.MaxAgeDays = 30
		}
	}
	if c.Notifications.Batch.Enabled {
		if c.Notifications.Batch.Interval.Duration <= 0 {
			c.Notifications.Batch.Interval = Duration{60 * time.Second}
		}
		if c.Notifications.Batch.MaxItems <= 0 {
			c.Notifications.Batch.MaxItems = 20
		}
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
		if r.RegionRules != nil {
			for k, v := range r.RegionRules.MaxAttempts {
				key := strings.TrimSpace(k)
				if key == "" || v <= 0 {
					delete(r.RegionRules.MaxAttempts, k)
					continue
				}
				if key != k {
					delete(r.RegionRules.MaxAttempts, k)
					r.RegionRules.MaxAttempts[key] = v
				}
			}
			if len(r.RegionRules.MaxAttempts) == 0 {
				r.RegionRules = nil
			}
		}
	}
	return nil
}

// MatchRegion 在 region_rules 中查找命中地区，返回覆盖后的 max_attempts。
func MatchRegion(rules *RegionRules, loc geoip.Location) (int, bool) {
	if rules == nil || len(rules.MaxAttempts) == 0 {
		return 0, false
	}
	places := []string{loc.City, loc.Region, loc.Country}
	for key, max := range rules.MaxAttempts {
		nk := normalizePlace(key)
		if nk == "" {
			continue
		}
		for _, place := range places {
			np := normalizePlace(place)
			if np == "" {
				continue
			}
			if np == nk || strings.Contains(np, nk) || strings.Contains(nk, np) {
				return max, true
			}
		}
	}
	return 0, false
}

func normalizePlace(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, suffix := range []string{"省", "市", "自治区", "特别行政区", " province", " city", " region"} {
		s = strings.TrimSpace(strings.TrimSuffix(s, suffix))
	}
	return s
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

func defaultLogPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "banhack233", "banhack233.log")
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/var/banhack233/banhack233.log"
	}
	return "/var/lib/banhack233/banhack233.log"
}

func defaultGeoIPPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "banhack233", "ip2region_v4.xdb")
	}
	if runtime.GOOS == "darwin" {
		return "/usr/local/var/banhack233/ip2region_v4.xdb"
	}
	return "/var/lib/banhack233/ip2region_v4.xdb"
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
