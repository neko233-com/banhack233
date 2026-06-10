package notify

import (
	"strings"
	"testing"
)

func TestInferSMTP(t *testing.T) {
	tests := map[string]SMTPPreset{
		"bot@qq.com":      {Host: "smtp.qq.com", Port: 465},
		"bot@163.com":     {Host: "smtp.163.com", Port: 465},
		"bot@gmail.com":   {Host: "smtp.gmail.com", Port: 587},
		"bot@outlook.com": {Host: "smtp-mail.outlook.com", Port: 587},
	}
	for addr, want := range tests {
		got, ok := InferSMTP(addr)
		if !ok {
			t.Fatalf("InferSMTP(%q) not found", addr)
		}
		if got != want {
			t.Fatalf("InferSMTP(%q)=%+v want %+v", addr, got, want)
		}
	}
}

func TestBuildEmailMessageMultipart(t *testing.T) {
	msg := string(buildEmailMessage("a@qq.com", "b@example.com", "主题", "plain", "<html>card</html>"))
	if !strings.Contains(msg, "multipart/alternative") {
		t.Fatalf("missing multipart: %q", msg)
	}
	if !strings.Contains(msg, "text/plain") || !strings.Contains(msg, "text/html") {
		t.Fatal("missing parts")
	}
}

func TestBuildEmailMessagePlainOnly(t *testing.T) {
	msg := string(buildEmailMessage("a@qq.com", "b@example.com", "subject", "plain only", ""))
	if strings.Contains(msg, "multipart") {
		t.Fatalf("unexpected multipart: %q", msg)
	}
	if !strings.Contains(msg, "plain only") {
		t.Fatal("missing body")
	}
}
