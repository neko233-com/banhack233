package geoip

import "testing"

func TestParseRegion(t *testing.T) {
	loc := ParseRegion("巴西|圣保罗|圣保罗|电信|BR")
	if loc.Country != "巴西" || loc.City != "圣保罗" {
		t.Fatalf("loc=%+v", loc)
	}
	if got := loc.String(); got != "巴西, 圣保罗, 圣保罗" {
		t.Fatalf("string=%q", got)
	}
}

func TestLocationStringSkipsEmpty(t *testing.T) {
	loc := ParseRegion("United States|California|Los Angeles|0|US")
	if got := loc.String(); got != "United States, California, Los Angeles" {
		t.Fatalf("string=%q", got)
	}
}

func TestLookupInvalidIP(t *testing.T) {
	l := New("")
	if _, err := l.Lookup("not-an-ip"); err == nil {
		t.Fatal("expected error")
	}
}
