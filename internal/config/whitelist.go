package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

// IsIgnoredIP accepts individual IPv4/IPv6 addresses and CIDR networks.
func IsIgnoredIP(ignore []string, ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, item := range ignore {
		item = strings.TrimSpace(item)
		if prefix, err := netip.ParsePrefix(item); err == nil {
			if prefix.Contains(addr) {
				return true
			}
		} else if candidate, err := netip.ParseAddr(item); err == nil && candidate.Unmap() == addr {
			return true
		}
	}
	return false
}

func validateIgnoreIP(item string) error {
	if _, err := netip.ParseAddr(item); err == nil {
		return nil
	}
	if _, err := netip.ParsePrefix(item); err == nil {
		return nil
	}
	return fmt.Errorf("invalid ignore_ips entry %q: expected an IP address or CIDR", item)
}

// AddIgnoreIPs updates only ignore_ips, preserving other fields, including
// unknown fields and notification credentials. It never edits daemon state.
func AddIgnoreIPs(path string, items []string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(b, &document); err != nil {
		return nil, err
	}
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if err := validateIgnoreIP(item); err != nil {
			return nil, err
		}
		found := false
		for _, existing := range cfg.IgnoreIPs {
			found = found || existing == item
		}
		if !found {
			cfg.IgnoreIPs = append(cfg.IgnoreIPs, item)
		}
	}
	if document == nil {
		return nil, fmt.Errorf("config must be a JSON object")
	}
	document["ignore_ips"], err = json.Marshal(cfg.IgnoreIPs)
	if err != nil {
		return nil, err
	}
	b, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".whitelist-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return nil, err
	}
	return cfg.IgnoreIPs, nil
}
