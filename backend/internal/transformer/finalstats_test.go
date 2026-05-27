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
