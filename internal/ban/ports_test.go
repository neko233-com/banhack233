package ban

import "testing"

func TestPortPlansDoNotConfuse22With2222(t *testing.T) {
	p := planChainRules("ip saddr @blocked tcp dport 2222 drop # handle 7")
	if !p.NeedV4 || len(p.DeleteHandles) != 1 {
		t.Fatal("wrong port treated as correct", p)
	}
	p = planChainRulesPorts("ip saddr @blocked tcp dport { 22, 2222 } drop # handle 7", []int{2222, 22, 22})
	if p.NeedV4 || len(p.DeleteHandles) != 0 {
		t.Fatal("configured ports not recognized", p)
	}
}
