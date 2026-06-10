package notify

import "testing"

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
