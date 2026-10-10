// Package update installs only verified assets from this project's stable GitHub releases.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/lock"
)

const repository = "neko233-com/banhack233"
const latestURL = "https://api.github.com/repos/" + repository + "/releases/latest"

var stable = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}
type Options struct {
	Current, Target, Config  string
	Apply, Restart, Rollback bool
}
type receipt struct {
	Version      string    `json:"version"`
	BackupSHA256 string    `json:"backup_sha256"`
	Updated      time.Time `json:"updated"`
}

func newer(candidate, current string) (bool, error) {
	a, b := stable.FindStringSubmatch(candidate), stable.FindStringSubmatch(current)
	if a == nil || b == nil {
		return false, fmt.Errorf("automatic updates require stable vMAJOR.MINOR.PATCH versions (current=%s, latest=%s)", current, candidate)
	}
	for i := 1; i <= 3; i++ {
		x, e := strconv.ParseUint(a[i], 10, 32)
		if e != nil {
			return false, e
		}
		y, e := strconv.ParseUint(b[i], 10, 32)
		if e != nil {
			return false, e
		}
		if x != y {
			return x > y, nil
		}
	}
	return false, nil
}

func fetch(ctx context.Context, client *http.Client, endpoint string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "banhack233-updater")
	if req.URL.Scheme == "https" && req.URL.Host == "api.github.com" {
		if token := os.Getenv("BANHACK233_GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("release response exceeds size limit")
	}
	return data, nil
}

func releaseAsset(r Release, name string) (string, error) {
	if r.Draft || r.Prerelease || !stable.MatchString(r.Tag) {
		return "", fmt.Errorf("release is not a published stable version")
	}
	want := "https://github.com/" + repository + "/releases/download/" + r.Tag + "/" + name
	url := ""
	count := 0
	for _, a := range r.Assets {
		if a.Name == name {
			count++
			url = a.URL
		}
	}
	if count != 1 || url != want {
		return "", fmt.Errorf("missing, duplicate or unexpected release asset %s", name)
	}
	return url, nil
}

func checksum(manifest []byte, name string) (string, error) {
	found := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("duplicate checksum for %s", name)
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size {
			return "", fmt.Errorf("invalid SHA256 for %s", name)
		}
		found = strings.ToLower(fields[0])
	}
	if found == "" {
		return "", fmt.Errorf("checksum missing for %s", name)
	}
	return found, nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func Run(ctx context.Context, opts Options) (string, error) {
	if opts.Target == "" {
		var err error
		opts.Target, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	target, err := filepath.Abs(opts.Target)
	if err != nil {
		return "", err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	opts.Target = target
	if _, err := config.Load(opts.Config); err != nil {
		return "", fmt.Errorf("existing config: %w", err)
	}
	if opts.Rollback {
		return rollback(ctx, opts)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	defer transport.CloseIdleConnections()
	// Release assets must also finish on slow international links. Connection/TLS
	// and response-header timeouts stay short; the body has a bounded ten minutes.
	client := &http.Client{Timeout: 10 * time.Minute, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" {
			return fmt.Errorf("unsafe release redirect")
		}
		return nil
	}}
	data, err := fetch(ctx, client, latestURL, 2*1024*1024)
	if err != nil {
		return "", err
	}
	var release Release
	if err = json.Unmarshal(data, &release); err != nil {
		return "", err
	}
	if release.Draft || release.Prerelease {
		return "", fmt.Errorf("latest release is not stable")
	}
	ok, err := newer(release.Tag, opts.Current)
	if err != nil {
		return "", err
	}
	if !ok {
		return fmt.Sprintf("up to date: %s (latest %s)", opts.Current, release.Tag), nil
	}
	if !opts.Apply {
		return fmt.Sprintf("update available: %s -> %s", opts.Current, release.Tag), nil
	}
	unlock, err := lock.Acquire(target + ".update.lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	// Recheck after taking the lock: another updater may have finished during the download check.
	installed, err := exec.CommandContext(ctx, target, "version").Output()
	if err != nil {
		return "", fmt.Errorf("cannot verify installed version: %w", err)
	}
	fields := strings.Fields(string(installed))
	if len(fields) < 2 || fields[0] != "banhack233" {
		return "", fmt.Errorf("unexpected installed binary")
	}
	if ok, err := newer(release.Tag, fields[1]); err != nil {
		return "", err
	} else if !ok {
		return "already updated: " + fields[1], nil
	}
	asset := "banhack233-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	assetURL, err := releaseAsset(release, asset)
	if err != nil {
		return "", err
	}
	sumsURL, err := releaseAsset(release, "SHA256SUMS.txt")
	if err != nil {
		return "", err
	}
	sums, err := fetch(ctx, client, sumsURL, 128*1024)
	if err != nil {
		return "", err
	}
	want, err := checksum(sums, asset)
	if err != nil {
		return "", err
	}
	binary, err := fetch(ctx, client, assetURL, 64*1024*1024)
	if err != nil {
		return "", err
	}
	if digest(binary) != want {
		return "", fmt.Errorf("SHA256 mismatch; installation refused")
	}
	stage, err := writeStage(target, binary)
	if err != nil {
		return "", err
	}
	defer os.Remove(stage)
	validationCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(validationCtx, stage, "version").Output()
	if err != nil || !strings.HasPrefix(string(out), "banhack233 "+release.Tag+" (") {
		return "", fmt.Errorf("downloaded binary version check failed")
	}
	if err := exec.CommandContext(validationCtx, stage, "config-check", "-config", opts.Config).Run(); err != nil {
		return "", fmt.Errorf("new binary rejected the existing config; installation refused")
	}
	previous, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	if err = writeAtomic(target+".previous", previous, 0755); err != nil {
		return "", err
	}
	r := receipt{Version: release.Tag, BackupSHA256: digest(previous), Updated: time.Now()}
	metadata, _ := json.MarshalIndent(r, "", "  ")
	if err = writeAtomic(target+".update.json", metadata, 0600); err != nil {
		return "", err
	}
	if err = install(ctx, opts, stage, previous, systemService{}); err != nil {
		return "", err
	}
	return fmt.Sprintf("installed %s; config/state preserved; rollback: banhack233 update -rollback -config %s", release.Tag, opts.Config), nil
}

type service interface {
	Stop(context.Context) error
	Start(context.Context) error
	Healthy(context.Context) error
}

func install(ctx context.Context, opts Options, stage string, previous []byte, s service) error {
	if opts.Restart {
		if err := s.Healthy(ctx); err != nil {
			return fmt.Errorf("managed daemon must be running before a restart update: %w", err)
		}
		if err := s.Stop(ctx); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, opts.Target); err != nil {
		if opts.Restart {
			recoverCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			return errors.Join(err, s.Start(recoverCtx))
		}
		return err
	}
	if !opts.Restart {
		return nil
	}
	err := s.Start(ctx)
	if err == nil {
		err = s.Healthy(ctx)
	}
	if err == nil {
		return nil
	}
	// Restore a failed startup automatically, but leave a failed update visible to the scheduler.
	recoverCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stopErr := s.Stop(recoverCtx)
	restoreErr := writeAtomic(opts.Target, previous, 0755)
	var startErr error
	if restoreErr == nil {
		startErr = s.Start(recoverCtx)
		if startErr == nil {
			startErr = s.Healthy(recoverCtx)
		}
	}
	return errors.Join(fmt.Errorf("new service failed health check; attempted rollback: %w", err), stopErr, restoreErr, startErr)
}

func rollback(ctx context.Context, opts Options) (string, error) {
	unlock, err := lock.Acquire(opts.Target + ".update.lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	metadata, err := os.ReadFile(opts.Target + ".update.json")
	if err != nil {
		return "", err
	}
	var r receipt
	if err = json.Unmarshal(metadata, &r); err != nil {
		return "", err
	}
	backup, err := os.ReadFile(opts.Target + ".previous")
	if err != nil {
		return "", err
	}
	if digest(backup) != r.BackupSHA256 {
		return "", fmt.Errorf("rollback backup SHA256 mismatch")
	}
	stage, err := writeStage(opts.Target, backup)
	if err != nil {
		return "", err
	}
	defer os.Remove(stage)
	previous, err := os.ReadFile(opts.Target)
	if err != nil {
		return "", err
	}
	if err = install(ctx, opts, stage, previous, systemService{}); err != nil {
		return "", err
	}
	return "previous binary restored; configuration and state preserved", nil
}

func writeStage(target string, data []byte) (string, error) {
	pattern := ".banhack233-update-*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	f, err := os.CreateTemp(filepath.Dir(target), pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(path, 0755)
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	stage, err := writeStage(path, data)
	if err != nil {
		return err
	}
	defer os.Remove(stage)
	if err = os.Chmod(stage, mode); err != nil {
		return err
	}
	return os.Rename(stage, path)
}
