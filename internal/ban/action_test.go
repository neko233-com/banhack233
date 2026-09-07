package ban

import "testing"

func TestNotifyDoesNotRunFirewall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, dryRun := range []bool{false, true} {
		got, err := Apply("192.0.2.3", "notify", dryRun)
		if err != nil || got != "notify" {
			t.Fatalf("action=%s err=%v", got, err)
		}
	}
}

func TestInvalidIPRejected(t *testing.T) {
	if _, err := Apply("--help", "auto", false); err == nil {
		t.Fatal("invalid IP accepted")
	}
	if err := Unban("--help"); err == nil {
		t.Fatal("invalid IP accepted")
	}
}
