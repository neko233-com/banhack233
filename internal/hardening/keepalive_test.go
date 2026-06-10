package hardening

import (
	"runtime"
	"strings"
	"testing"
)

func TestApplyKeepalivePreviewSkipsTCPByDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("keepalive is linux-only")
	}
	got, err := ApplyKeepalive(false, 24, false)
	if err != nil {
		t.Fatalf("ApplyKeepalive preview error: %v", err)
	}
	if !strings.Contains(got, "ClientAliveCountMax 1440") {
		t.Fatalf("preview missing SSH keepalive:\n%s", got)
	}
	if strings.Contains(got, "tcp_keepalive_time") || strings.Contains(got, "/etc/sysctl.d/99-banhack233-keepalive.conf") {
		t.Fatalf("preview should not include TCP sysctl by default:\n%s", got)
	}
}

func TestApplyKeepalivePreviewIncludesTCPWhenOptedIn(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("keepalive is linux-only")
	}
	got, err := ApplyKeepalive(false, 24, true)
	if err != nil {
		t.Fatalf("ApplyKeepalive preview error: %v", err)
	}
	if !strings.Contains(got, "net.ipv4.tcp_keepalive_time = 60") {
		t.Fatalf("preview missing opt-in TCP sysctl:\n%s", got)
	}
}
