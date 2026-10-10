package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/update"
	"github.com/neko233-com/banhack233/internal/version"
	"os"
	"time"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	path := fs.String("config", config.DefaultPath(), "existing configuration")
	apply := fs.Bool("apply", false, "install latest stable release after SHA256 and configuration checks")
	restart := fs.Bool("restart", false, "restart managed daemon, rollback on failed health check")
	rollback := fs.Bool("rollback", false, "restore verified previous binary")
	target := fs.String("target", "", "installed executable (update helper only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *apply && *rollback {
		return fmt.Errorf("choose -apply or -rollback")
	}
	if _, err := config.Load(*path); err != nil {
		return err
	}
	opts := update.Options{Current: version.Version, Config: *path, Apply: *apply, Restart: *restart, Rollback: *rollback, Target: *target}
	started, err := update.Launch(opts)
	if err != nil {
		return err
	}
	if started {
		fmt.Println("Windows update helper started; completion/failure is recorded beside the installed executable in .update.log")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	out, err := update.Run(ctx, opts)
	if out != "" {
		fmt.Println(out)
	}
	return err
}
func runAutoUpdate(args []string) error {
	fs := flag.NewFlagSet("auto-update", flag.ContinueOnError)
	path := fs.String("config", config.DefaultPath(), "existing configuration")
	enable := fs.Bool("enable", false, "install daily updater and restart managed daemon on successful upgrades")
	disable := fs.Bool("disable", false, "remove daily update schedule")
	status := fs.Bool("status", false, "show daily update schedule")
	if err := fs.Parse(args); err != nil {
		return err
	}
	n := 0
	for _, v := range []bool{*enable, *disable, *status} {
		if v {
			n++
		}
	}
	if n > 1 {
		return fmt.Errorf("choose one auto-update action")
	}
	action := "status"
	if *enable {
		action = "enable"
		if _, err := config.Load(*path); err != nil {
			return err
		}
	}
	if *disable {
		action = "disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := update.Schedule(ctx, action, *path)
	if out != "" {
		fmt.Println(out)
	}
	return err
}
func runSafeSSH(args []string) error {
	fs := flag.NewFlagSet("safe-ssh", flag.ContinueOnError)
	path := fs.String("config", config.DefaultPath(), "configuration")
	write := fs.Bool("write", false, "save password-only default SSH rule with 10m successful-login grace; keeps thresholds and whitelist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := config.Load(*path); err != nil {
		return err
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(data, &doc); err != nil {
		return err
	}
	var rules []map[string]json.RawMessage
	if err = json.Unmarshal(doc["rules"], &rules); err != nil {
		return err
	}
	found := false
	defaults := config.Default().Rules[0]
	for _, r := range rules {
		var name string
		_ = json.Unmarshal(r["name"], &name)
		if name != "ssh-auth-failure" {
			continue
		}
		found = true
		r["patterns"], _ = json.Marshal(defaults.Patterns)
		r["reset_patterns"], _ = json.Marshal(defaults.ResetPatterns)
		r["count_by_user"] = json.RawMessage("true")
		r["success_grace"] = json.RawMessage(`"10m"`)
	}
	if !found {
		return fmt.Errorf("safe-ssh only migrates the named ssh-auth-failure rule; review custom rules manually")
	}
	doc["rules"], _ = json.Marshal(rules)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if !*write {
		fmt.Println("Will set ssh-auth-failure to password-only, count_by_user=true, reset on success, success_grace=10m; thresholds, paths, whitelist and other settings stay unchanged. Use -write to apply.")
		return nil
	}
	if err = os.WriteFile(*path+".before-safe-ssh", data, 0600); err != nil {
		return err
	}
	if err = os.WriteFile(*path, append(out, '\n'), 0600); err != nil {
		return err
	}
	fmt.Println("saved safe SSH policy; restart daemon to apply; backup:", *path+".before-safe-ssh")
	return nil
}
