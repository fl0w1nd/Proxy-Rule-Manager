package ir

import (
	"strings"
	"testing"
)

func TestClassicalParsesPlainItems(t *testing.T) {
	content := strings.Join([]string{
		"# comment",
		"apple.com",
		".icloud.com",       // Surge domain-set leading dot
		"+.example.org",     // Clash suffix incl. self
		"*.*.microsoft.com", // Clash wildcard
		"1.2.3.4",
		"192.168.0.0/16",
		"2001:db8::/32",
		"*",                   // bare star -> diagnostic
		"http://not a domain", // diagnostic
	}, "\n")
	rs := ruleLineListParser{}.Parse(content)
	want := []struct {
		kind  Kind
		value string
	}{
		{KindDomain, "apple.com"},
		{KindDomainSuffix, "icloud.com"},
		{KindDomainSuffix, "example.org"},
		{KindDomainWildcard, "*.*.microsoft.com"},
		{KindIPCIDR, "1.2.3.4/32"},
		{KindIPCIDR, "192.168.0.0/16"},
		{KindIPCIDR, "2001:db8::/32"},
	}
	if len(rs.Entries) != len(want) {
		t.Fatalf("want %d entries, got %d: %+v", len(want), len(rs.Entries), rs.Entries)
	}
	for i, w := range want {
		if rs.Entries[i].Kind != w.kind || rs.Entries[i].Value != w.value {
			t.Errorf("entry %d: got (%s,%q) want (%s,%q)", i, rs.Entries[i].Kind, rs.Entries[i].Value, w.kind, w.value)
		}
	}
	if len(rs.Diagnostics) != 2 {
		t.Fatalf("want 2 diagnostics, got %+v", rs.Diagnostics)
	}
}

func TestMihomoYAMLParse(t *testing.T) {
	content := `# provider file
payload:
  - '.blogger.com'
  - '+.example.org'
  - 'books.itunes.apple.com'
  - DOMAIN-SUFFIX,google.com
  - IP-CIDR,127.0.0.0/8,no-resolve
  - GEOIP,CN # inline comment
  - '192.168.1.0/24'
`
	rs := yamlPayloadParser{}.Parse(content)
	if len(rs.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", rs.Diagnostics)
	}
	kinds := []Kind{KindDomainSuffix, KindDomainSuffix, KindDomain, KindDomainSuffix, KindIPCIDR, KindGeoIP, KindIPCIDR}
	if len(rs.Entries) != len(kinds) {
		t.Fatalf("want %d entries, got %d: %+v", len(kinds), len(rs.Entries), rs.Entries)
	}
	for i, k := range kinds {
		if rs.Entries[i].Kind != k {
			t.Errorf("entry %d kind %s want %s", i, rs.Entries[i].Kind, k)
		}
	}
	if !rs.Entries[4].HasFlag(FlagNoResolve) {
		t.Error("no-resolve flag lost in YAML item")
	}
	if rs.Entries[5].Value != "CN" {
		t.Errorf("inline comment not stripped: %q", rs.Entries[5].Value)
	}
}

func TestMihomoYAMLRulesKey(t *testing.T) {
	rs := yamlPayloadParser{}.Parse("rules:\n  - DOMAIN,x.com\n")
	if len(rs.Entries) != 1 || rs.Entries[0].Kind != KindDomain {
		t.Fatalf("rules: key not accepted: %+v", rs)
	}
}

func TestMihomoYAMLDiagnosticUsesSourceLine(t *testing.T) {
	content := `# provider metadata
name: example
payload:
  - DOMAIN,valid.example
  - IP-CIDR,999.1.1.1/33
`
	rs := yamlPayloadParser{}.Parse(content)
	if len(rs.Diagnostics) != 1 || rs.Diagnostics[0].Line != 5 {
		t.Fatalf("diagnostics: %+v", rs.Diagnostics)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name    string
		content string
		format  string
	}{
		{"classical", "DOMAIN-SUFFIX,google.com\nIP-CIDR,10.0.0.0/8,no-resolve\nGEOIP,CN\n", FormatClassical},
		{"plain domains", "apple.com\n.icloud.com\n+.example.org\n", FormatClassical},
		{"bare cidrs", "10.0.0.0/8\n192.168.0.0/16\n", FormatClassical},
		{"mihomo yaml", "payload:\n  - '+.google.com'\n  - DOMAIN,x.com\n", FormatMihomoYAML},
	}
	for _, c := range cases {
		det := Detect(c.content)
		if det.Format != c.format {
			t.Errorf("%s: detected %s (%.2f), want %s", c.name, det.Format, det.Confidence, c.format)
		}
		if det.Confidence < 0.9 {
			t.Errorf("%s: low confidence %.2f", c.name, det.Confidence)
		}
	}
}

func TestParseAutoAndExplicit(t *testing.T) {
	rs, det, err := Parse("DOMAIN,x.com\n", FormatAuto)
	if err != nil || det.Format != FormatClassical || len(rs.Entries) != 1 {
		t.Fatalf("auto parse: %v %+v %+v", err, det, rs)
	}
	// Explicit format wins over sniffing.
	rs, det, err = Parse("apple.com\n", FormatClassical)
	if err != nil || det.Format != FormatClassical || det.Confidence != 1 {
		t.Fatalf("explicit parse: %v %+v", err, det)
	}
	if _, _, err := Parse("x", "bogus-format"); err == nil {
		t.Fatal("bogus format should error")
	}
	_ = rs
}
