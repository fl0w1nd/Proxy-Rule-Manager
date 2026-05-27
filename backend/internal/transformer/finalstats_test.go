package transformer

import (
	"testing"
)

func TestComputeFinalStats_ClassicalList(t *testing.T) {
	content := "DOMAIN,example.com\nDOMAIN-SUFFIX,foo.com\n# comment\n\nIP-CIDR,10.0.0.0/8\n"
	stats := ComputeFinalStats(content)
	if stats.TotalLines != 3 {
		t.Fatalf("expected 3 significant lines, got %d", stats.TotalLines)
	}
	if stats.Format != FormatClassical {
		t.Errorf("Format: got %q, want %q", stats.Format, FormatClassical)
	}
	if stats.ByType["DOMAIN"] != 1 {
		t.Errorf("DOMAIN count: got %d, want 1", stats.ByType["DOMAIN"])
	}
	if stats.ByType["DOMAIN-SUFFIX"] != 1 {
		t.Errorf("DOMAIN-SUFFIX count: got %d, want 1", stats.ByType["DOMAIN-SUFFIX"])
	}
	if stats.ByType["IP-CIDR"] != 1 {
		t.Errorf("IP-CIDR count: got %d, want 1", stats.ByType["IP-CIDR"])
	}
	if stats.PayloadCount != nil {
		t.Errorf("PayloadCount should be nil for classical, got %d", *stats.PayloadCount)
	}
}

func TestComputeFinalStats_SingboxSource(t *testing.T) {
	content := `{
  "version": 3,
  "rules": [
    { "domain": ["a.com", "b.com"] },
    { "domain_suffix": [".cn"] },
    { "ip_cidr": ["1.1.1.1/32", "2.2.2.2/32", "3.3.3.3/32"] },
    { "port": [80, 443] },
    { "domain_keyword": ["ads"], "invert": true }
  ]
}
`
	stats := ComputeFinalStats(content)
	if stats.Format != FormatSingboxSource {
		t.Fatalf("Format: got %q, want %q", stats.Format, FormatSingboxSource)
	}
	if stats.PayloadCount == nil || *stats.PayloadCount != 5 {
		t.Fatalf("PayloadCount: got %v, want 5", stats.PayloadCount)
	}
	if stats.TotalLines != 9 {
		// 2 + 1 + 3 + 2 + 1 = 9 matcher values total
		t.Fatalf("TotalLines: got %d, want 9", stats.TotalLines)
	}
	wantByType := map[string]int{
		"domain":         2,
		"domain_suffix":  1,
		"ip_cidr":        3,
		"port":           2,
		"domain_keyword": 1,
	}
	for key, want := range wantByType {
		if stats.ByType[key] != want {
			t.Errorf("ByType[%q]: got %d, want %d", key, stats.ByType[key], want)
		}
	}
	if _, leaked := stats.ByType["invert"]; leaked {
		t.Errorf("invert boolean should not be counted as a matcher type")
	}
}

func TestComputeFinalStats_SingboxEmptyRules(t *testing.T) {
	stats := ComputeFinalStats(`{"version": 3, "rules": []}`)
	if stats.Format != FormatSingboxSource {
		t.Fatalf("Format: got %q, want %q", stats.Format, FormatSingboxSource)
	}
	if stats.TotalLines != 0 {
		t.Errorf("TotalLines: got %d, want 0", stats.TotalLines)
	}
	if stats.PayloadCount == nil || *stats.PayloadCount != 0 {
		t.Errorf("PayloadCount: got %v, want 0", stats.PayloadCount)
	}
	if len(stats.ByType) != 0 {
		t.Errorf("ByType should be empty, got %v", stats.ByType)
	}
}

func TestComputeFinalStats_PlainJSONFallsBackToClassical(t *testing.T) {
	// A JSON object without a "rules" array shouldn't be treated as
	// sing-box source; we'd rather show a slightly wrong classical
	// breakdown than invent a meaningless empty sing-box table.
	stats := ComputeFinalStats(`{"version": 3, "metadata": {"name": "x"}}`)
	if stats.Format != FormatClassical {
		t.Fatalf("Format: got %q, want %q", stats.Format, FormatClassical)
	}
}

func TestComputeFinalStats_YAMLPayload(t *testing.T) {
	content := "payload:\n  - 'DOMAIN,example.com'\n  - 'DOMAIN-SUFFIX,foo.com'\n  - 'IP-CIDR,10.0.0.0/8'\n"
	stats := ComputeFinalStats(content)
	if stats.TotalLines != 3 {
		t.Fatalf("expected 3 payload entries, got %d", stats.TotalLines)
	}
	if stats.PayloadCount == nil || *stats.PayloadCount != 3 {
		t.Errorf("PayloadCount: got %v, want 3", stats.PayloadCount)
	}
	if stats.ByType["DOMAIN"] != 1 || stats.ByType["DOMAIN-SUFFIX"] != 1 || stats.ByType["IP-CIDR"] != 1 {
		t.Errorf("ByType mismatch: %v", stats.ByType)
	}
}

func TestComputeFinalStats_YAMLPayloadEmpty(t *testing.T) {
	stats := ComputeFinalStats("payload: []\n")
	if stats.TotalLines != 0 {
		t.Fatalf("expected 0, got %d", stats.TotalLines)
	}
	if stats.PayloadCount == nil || *stats.PayloadCount != 0 {
		t.Errorf("PayloadCount: got %v, want 0", stats.PayloadCount)
	}
}

func TestComputeFinalStats_Empty(t *testing.T) {
	stats := ComputeFinalStats("")
	if stats.TotalLines != 0 {
		t.Fatalf("expected 0, got %d", stats.TotalLines)
	}
	if len(stats.ByType) != 0 {
		t.Errorf("expected empty ByType, got %v", stats.ByType)
	}
}

func TestComputeFinalStats_CRLFIdempotent(t *testing.T) {
	lfContent := "DOMAIN,example.com\nDOMAIN-SUFFIX,foo.com\n"
	crlfContent := "DOMAIN,example.com\r\nDOMAIN-SUFFIX,foo.com\r\n"
	lfStats := ComputeFinalStats(lfContent)
	crlfStats := ComputeFinalStats(crlfContent)
	if lfStats.TotalLines != crlfStats.TotalLines {
		t.Fatalf("CRLF mismatch: LF=%d CRLF=%d", lfStats.TotalLines, crlfStats.TotalLines)
	}
}

func TestCountSignificantLines(t *testing.T) {
	content := "DOMAIN,a.com\n# comment\n\nDOMAIN-SUFFIX,b.com\n; semicolon comment\nIP-CIDR,1.2.3.4/32\n"
	n := CountSignificantLines(content)
	if n != 3 {
		t.Fatalf("expected 3 significant lines, got %d", n)
	}
}

func TestCountSignificantLines_Empty(t *testing.T) {
	if n := CountSignificantLines(""); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}
