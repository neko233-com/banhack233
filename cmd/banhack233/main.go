package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/neko233-com/banhack233/internal/audit"
	"github.com/neko233-com/banhack233/internal/autostart"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/daemon"
	"github.com/neko233-com/banhack233/internal/hardening"
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
	case "doctor", "audit":
		return runDoctor(args[1:])
	case "secure-ssh":
		return runSecureSSH(args[1:])
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

func runSecureSSH(args []string) error {
	fs := flag.NewFlagSet("banhack233 secure-ssh", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	write := fs.Bool("write", false, "write managed sshd_config block and reload ssh")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	out, err := hardening.SecureSSH(cfg, *write)
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
  banhack233 test [-config path]             scan once and send configured test notification
  banhack233 doctor [-config path]           audit local security posture
  banhack233 secure-ssh [-config path]       preview SSH hardening block
  banhack233 secure-ssh -write               apply SSH hardening after backup + sshd -t
  banhack233 install-autostart [-config path] install systemd or Windows startup task
  banhack233 uninstall-autostart             remove autostart
  banhack233 autostart-status                show autostart status
  banhack233 version                         print version

Safe password-SSH baseline:
  keep PasswordAuthentication yes only for normal users
  set PermitRootLogin no, PermitEmptyPasswords no, MaxAuthTries 3
  enable banhack233 autostart, alerts, hourly doctor audit, and firewall bans`)
}
