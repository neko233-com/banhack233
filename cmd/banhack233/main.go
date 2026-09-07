package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/audit"
	"github.com/neko233-com/banhack233/internal/autostart"
	"github.com/neko233-com/banhack233/internal/ban"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/daemon"
	"github.com/neko233-com/banhack233/internal/hardening"
	"github.com/neko233-com/banhack233/internal/malware"
	"github.com/neko233-com/banhack233/internal/notify"
	"github.com/neko233-com/banhack233/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		args = []string{"help"}
	}
	switch args[0] {
	case "init-config":
		return runInitConfig(args[1:])
	case "run":
		return runDaemon(args[1:])
	case "test":
		return runOnce(args[1:])
	case "notify-test":
		return runNotifyTest(args[1:])
	case "doctor", "audit":
		return runDoctor(args[1:])
	case "malware-scan":
		return runMalwareScan(args[1:])
	case "status":
		return runStatus(args[1:])
	case "secure-ssh":
		return runSecureSSH(args[1:])
	case "keepalive":
		return runKeepalive(args[1:])
	case "ban-list":
		out, err := ban.List()
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "whitelist":
		return runWhitelist(args[1:])
	case "unban":
		if len(args) < 2 {
			return fmt.Errorf("usage: banhack233 unban <ip>")
		}
		return ban.Unban(args[1])
	case "install-autostart":
		return runInstallAutostart(args[1:])
	case "uninstall-autostart":
		return autostart.Disable()
	case "autostart-status":
		status, err := autostart.Status()
		if err != nil {
			return err
		}
		fmt.Printf("backend=%s\nenabled=%t\nactive=%t\ndetail=%s\n", status.Backend, status.Enabled, status.Active, status.Detail)
		return nil
	case "version":
		fmt.Println(version.String())
		return nil
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runWhitelist(args []string) error {
	fs := flag.NewFlagSet("banhack233 whitelist", flag.ContinueOnError)
	path := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		cfg, err := config.Load(*path)
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(cfg.IgnoreIPs, "\n"))
		return nil
	}
	ips, err := config.AddIgnoreIPs(*path, fs.Args())
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(ips, "\n"))
	fmt.Println("Saved. Restart the daemon to apply the whitelist and release its existing automatic bans (production mode).")
	return nil
}

func runMalwareScan(args []string) error {
	fs := flag.NewFlagSet("banhack233 malware-scan", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	kill := fs.Bool("kill", false, "kill suspicious miner/intrusion processes for this manual scan")
	reportName := fs.String("name", "malware-scan", "report file name prefix")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if cfg.Malware.DirectKill {
		*kill = true
	}
	findings := malware.Scan(cfg.Malware, malware.Options{Kill: *kill})
	fmt.Println(malware.Format(findings))
	report, err := malware.WriteReport(cfg.Malware, *reportName, findings)
	if err != nil {
		return err
	}
	fmt.Println("report:", report)
	return nil
}

func runInitConfig(args []string) error {
	fs := flag.NewFlagSet("banhack233 init-config", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	force := fs.Bool("force", false, "overwrite existing config")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*force {
		if _, err := os.Stat(*cfgPath); err == nil {
			return fmt.Errorf("config exists: %s", *cfgPath)
		}
	}
	if err := os.MkdirAll(filepath.Dir(*cfgPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(config.Default(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*cfgPath, append(b, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println("created config:", *cfgPath)
	return nil
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("banhack233 run", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	return daemon.Run(context.Background(), cfg)
}

func runOnce(args []string) error {
	fs := flag.NewFlagSet("banhack233 test", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return daemon.RunOnce(ctx, cfg)
}

func runNotifyTest(args []string) error {
	fs := flag.NewFlagSet("banhack233 notify-test", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	channels := fs.String("channel", "", "comma-separated channels: console,feishu,discord,slack,email,webhook or webhook:<name>")
	message := fs.String("message", "", "custom test message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var selected []string
	if strings.TrimSpace(*channels) != "" {
		selected = []string{*channels}
	}
	_, err = notify.Test(ctx, cfg.Notifications, notify.TestOptions{Channels: selected, Message: *message})
	return err
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("banhack233 doctor", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	fmt.Println(audit.Format(audit.Run(cfg)))
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("banhack233 status", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	auto, _ := autostart.Status()
	fmt.Printf("version=%s\nconfig=%s\ndry_run=%t\nstart_at_end=%t\nstate=%s\nautostart_backend=%s\nautostart_enabled=%t\nautostart_active=%t\nrules=%d\n", version.String(), *cfgPath, cfg.DryRun, cfg.StartAtEnd, cfg.StatePath, auto.Backend, auto.Enabled, auto.Active, len(cfg.Rules))
	fmt.Println("audit:")
	fmt.Println(audit.Format(audit.Run(cfg)))
	return nil
}

func runSecureSSH(args []string) error {
	fs := flag.NewFlagSet("banhack233 secure-ssh", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	write := fs.Bool("write", false, "write managed sshd_config block and reload ssh")
	force := fs.Bool("force", false, "allow writing sshd_config without allowed_users")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	out, err := hardening.SecureSSH(cfg, *write, *force)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func runKeepalive(args []string) error {
	fs := flag.NewFlagSet("banhack233 keepalive", flag.ContinueOnError)
	write := fs.Bool("write", false, "write SSH keepalive config")
	hours := fs.Int("ssh-hours", 24, "SSH idle keepalive window in hours")
	tcp := fs.Bool("tcp", false, "also write system TCP keepalive/conntrack sysctl; opt-in only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	out, err := hardening.ApplyKeepalive(*write, *hours, *tcp)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func runInstallAutostart(args []string) error {
	fs := flag.NewFlagSet("banhack233 install-autostart", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return autostart.Enable(*cfgPath)
}

func printHelp() {
	fmt.Println(`banhack233 - adaptive host attack blocker and notifier

Usage:
  banhack233 init-config [-config path] [-force] create safe default config
  banhack233 run [-config path]              run daemon
  banhack233 test [-config path]             scan logs once (ban/audit notifications on events)
  banhack233 notify-test [-config path]      send test notification to enabled channels
  banhack233 notify-test -channel feishu,email
  banhack233 notify-test -message "custom text"
  banhack233 doctor [-config path]           audit local security posture
  banhack233 malware-scan [-kill] [-name prefix] scan linux miners and write report
  banhack233 status [-config path]           show version, config, autostart, audit summary
  banhack233 secure-ssh [-config path]       preview SSH hardening block
  banhack233 secure-ssh -write               apply SSH hardening; requires allowed_users unless -force
  banhack233 keepalive [-write] [-tcp]       keep SSH alive; TCP sysctl is opt-in
  banhack233 ban-list                        list active ban backend entries
  banhack233 whitelist [-config path] [ip/cidr ...] list or add never-ban addresses; restart daemon after changes
  banhack233 unban <ip>                      remove a blocked IP
  banhack233 install-autostart [-config path] install systemd or Windows startup task
  banhack233 uninstall-autostart             remove autostart
  banhack233 autostart-status                show autostart status
  banhack233 version                         print version

Safe password-SSH baseline:
  keep PasswordAuthentication yes when needed
  root login is supported; keep strong password, PermitEmptyPasswords no, MaxAuthTries 3
  enable banhack233 autostart, alerts, hourly doctor audit, and firewall bans`)
}
