package hardening

import (
	"strings"
	"testing"

	"github.com/neko233-com/banhack233/internal/config"
)

func TestRenderSSHDManagedBlock(t *testing.T) {
	block := renderSSHDManagedBlock(config.SSHHardening{
		PasswordAuthentication: true,
		PermitRootLogin:        "no",
		AllowedUsers:           []string{"deploy"},
		MaxAuthTries:           3,
		LoginGraceTime:         "20s",
		DisableEmptyPasswords:  true,
	})
	for _, want := range []string{"PasswordAuthentication yes", "PermitRootLogin no", "AllowUsers deploy", "MaxAuthTries 3", "PermitEmptyPasswords no"} {
		if !strings.Contains(block, want) {
			t.Fatalf("block missing %q:\n%s", want, block)
		}
	}
}

func TestReplaceManagedBlock(t *testing.T) {
	got := replaceManagedBlock("A\n# BEGIN banhack233 managed\nold\n# END banhack233 managed\nB\n", "new\n")
	if got != "A\nnew\nB\n" {
		t.Fatalf("got %q", got)
	}
}
