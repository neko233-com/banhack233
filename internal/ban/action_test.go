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

func TestPlanChainRules(t *testing.T) {
	legacy := `chain input {
		ip saddr @blocked drop # handle 3
		ip saddr @blocked drop # handle 4
	}`
	plan := planChainRules(legacy)
	if !plan.NeedAdd || len(plan.DeleteHandles) != 2 || plan.DeleteHandles[0] != "3" || plan.DeleteHandles[1] != "4" || plan.MissingHandle {
		t.Fatalf("legacy plan = %+v", plan)
	}

	scoped := "ip saddr @blocked tcp dport 22 drop # handle 5\n"
	plan = planChainRules(scoped)
	if plan.NeedAdd || len(plan.DeleteHandles) != 0 || plan.MissingHandle {
		t.Fatalf("scoped plan = %+v", plan)
	}

	dup := "ip saddr @blocked tcp dport 22 drop # handle 5\n" +
		"ip saddr @blocked tcp dport 22 drop # handle 6\n" +
		"ip saddr @blocked drop # handle 7\n"
	plan = planChainRules(dup)
	if plan.NeedAdd || len(plan.DeleteHandles) != 2 || plan.DeleteHandles[0] != "6" || plan.DeleteHandles[1] != "7" {
		t.Fatalf("dup plan = %+v", plan)
	}

	plan = planChainRules("chain input {\n}")
	if !plan.NeedAdd || len(plan.DeleteHandles) != 0 {
		t.Fatalf("empty plan = %+v", plan)
	}

	plan = planChainRules("ip saddr @blocked drop\n")
	if !plan.MissingHandle || !plan.NeedAdd {
		t.Fatalf("missing-handle plan = %+v", plan)
	}
}
