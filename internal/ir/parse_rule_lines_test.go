package ir

import (
	"strings"
	"testing"
)

func parseOne(t *testing.T, line string) Entry {
	t.Helper()
	e, err := parseRuleLine(line)
	if err != nil {
		t.Fatalf("parseRuleLine(%q) error: %v", line, err)
	}
	return e
}

func TestClassicalSpellingUnion(t *testing.T) {
	cases := []struct {
		line  string
		kind  Kind
		value string
	}{
		{"DOMAIN,Example.COM", KindDomain, "example.com"},
		{"HOST,example.com", KindDomain, "example.com"},            // QX
		{"HOST-SUFFIX,Google.com", KindDomainSuffix, "google.com"}, // QX
		{"DOMAIN-SUFFIX,google.com", KindDomainSuffix, "google.com"},
		{"DOMAIN-KEYWORD,google", KindDomainKeyword, "google"},
		{"DOMAIN-WILDCARD,ad*.example.com", KindDomainWildcard, "ad*.example.com"},
		{"DOMAIN-REGEX,^abc.*com", KindDomainRegex, "^abc.*com"},
		{"GEOSITE,YouTube", KindGeosite, "youtube"},
		{"IP-CIDR,127.0.0.0/8", KindIPCIDR, "127.0.0.0/8"},
		{"IP-CIDR6,2620:0:2d0:200::7/32", KindIPCIDR, "2620::/32"}, // masked canonical
		{"IP6-CIDR,2001:db8::/32", KindIPCIDR, "2001:db8::/32"},    // QX
		{"IP-SUFFIX,8.8.8.8/24", KindIPSuffix, "8.8.8.8/24"},
		{"IP-ASN,AS13335", KindIPASN, "13335"},
		{"IP-ASN,13335", KindIPASN, "13335"},
		{"GEOIP,cn", KindGeoIP, "CN"},
		{"SRC-GEOIP,cn", KindSrcGeoIP, "CN"},
		{"SRC-IP-CIDR,192.168.1.201/32", KindSrcIPCIDR, "192.168.1.201/32"},
		{"SRC-IP,192.168.20.0/24", KindSrcIPCIDR, "192.168.20.0/24"}, // Surge
		{"DST-PORT,80", KindDstPort, "80"},
		{"DEST-PORT,10000-20000", KindDstPort, "10000-20000"}, // Surge spelling
		{"SRC-PORT,>=50000", KindSrcPort, "50000-65535"},      // Surge operator
		{"DST-PORT,114-514/810-1919", KindDstPort, "114-514/810-1919"},
		{"IN-PORT,7890", KindInPort, "7890"},
		{"IN-TYPE,SOCKS/HTTP", KindInType, "SOCKS/HTTP"},
		{"IN-USER,mihomo", KindInUser, "mihomo"},
		{"IN-NAME,ss", KindInName, "ss"},
		{"PROCESS-NAME,curl", KindProcessName, "curl"},
		{"PROCESS-NAME-WILDCARD,*telegram*", KindProcessNameWildcard, "*telegram*"},
		{"PROCESS-PATH,/usr/bin/wget", KindProcessPath, "/usr/bin/wget"},
		{"PROCESS-PATH-WILDCARD,/usr/*/wget", KindProcessPathWildcard, "/usr/*/wget"},
		{"UID,1001", KindUID, "1001"},
		{"NETWORK,UDP", KindNetwork, "udp"},
		{"DSCP,4", KindDSCP, "4"},
		{"USER-AGENT,Instagram*", KindUserAgent, "Instagram*"},
		{"SUBNET,SSID:MyHome", KindSubnet, "SSID:MyHome"},
		{"PROTOCOL,udp", KindProtocol, "UDP"},
		{"HOSTNAME-TYPE,IPv6", KindHostnameType, "IPv6"},
	}
	for _, c := range cases {
		e := parseOne(t, c.line)
		if e.Kind != c.kind || e.Value != c.value {
			t.Errorf("%q => (%s, %q); want (%s, %q)", c.line, e.Kind, e.Value, c.kind, c.value)
		}
	}
}

func TestClassicalPolicyAndFlags(t *testing.T) {
	// Full-config line: policy stripped.
	e := parseOne(t, "DOMAIN,example.com,DIRECT")
	if e.Kind != KindDomain || e.Value != "example.com" || len(e.Flags) != 0 {
		t.Fatalf("policy not stripped: %+v", e)
	}
	// Provider line with flag.
	e = parseOne(t, "IP-CIDR,10.0.0.0/8,no-resolve")
	if !e.HasFlag(FlagNoResolve) {
		t.Fatalf("no-resolve lost: %+v", e)
	}
	// Full-config line with policy AND flag.
	e = parseOne(t, "IP-CIDR,10.0.0.0/8,PROXY,no-resolve")
	if !e.HasFlag(FlagNoResolve) {
		t.Fatalf("no-resolve after policy lost: %+v", e)
	}
	// src flag converts kind and drops redundant no-resolve.
	e = parseOne(t, "GEOIP,CN,DIRECT,src")
	if e.Kind != KindSrcGeoIP || len(e.Flags) != 0 {
		t.Fatalf("src flag conversion failed: %+v", e)
	}
	// Surge-only params are recognised but not preserved.
	e = parseOne(t, "DOMAIN-SUFFIX,apple.com,DIRECT,extended-matching")
	if e.Kind != KindDomainSuffix || len(e.Flags) != 0 {
		t.Fatalf("extended-matching mishandled: %+v", e)
	}
}

func TestClassicalCommaPayloads(t *testing.T) {
	// Regex payload with commas must stay intact.
	e := parseOne(t, `DOMAIN-REGEX,^a{1,3}\.example\.com$`)
	if e.Value != `^a{1,3}\.example\.com$` {
		t.Fatalf("comma payload mangled: %q", e.Value)
	}
}

func TestClassicalLogical(t *testing.T) {
	e := parseOne(t, "AND,((DOMAIN,baidu.com),(NETWORK,UDP)),DIRECT")
	if e.Kind != KindAnd || len(e.Sub) != 2 {
		t.Fatalf("AND parse: %+v", e)
	}
	if e.Sub[0].Kind != KindDomain || e.Sub[1].Kind != KindNetwork || e.Sub[1].Value != "udp" {
		t.Fatalf("AND subs: %+v", e.Sub)
	}

	// Surge-style spaces around groups.
	e = parseOne(t, "OR,((SRC-IP,192.168.1.110), (SRC-IP,192.168.1.111)),DIRECT")
	if e.Kind != KindOr || len(e.Sub) != 2 || e.Sub[0].Kind != KindSrcIPCIDR {
		t.Fatalf("OR parse: %+v", e)
	}

	// Nested logic.
	e = parseOne(t, "AND,((OR,((DOMAIN-SUFFIX,a.com),(DOMAIN-SUFFIX,b.com))),(NOT,((NETWORK,udp)))),PROXY")
	if e.Kind != KindAnd || len(e.Sub) != 2 || e.Sub[0].Kind != KindOr || e.Sub[1].Kind != KindNot {
		t.Fatalf("nested logical parse: %+v", e)
	}

	// NOT requires exactly one sub-rule.
	if _, err := parseRuleLine("NOT,((DOMAIN,a.com),(DOMAIN,b.com)),X"); err == nil {
		t.Fatal("NOT with two subs should fail")
	}
}

func TestClassicalDiagnostics(t *testing.T) {
	content := strings.Join([]string{
		"# comment",
		"; also comment",
		"// premium comment",
		"",
		"DOMAIN-SUFFIX,ok.com",
		"MATCH,DIRECT",
		"RULE-SET,foo,DIRECT",
		"TOTALLY-BOGUS,xyz",
		"IP-CIDR,not-an-ip/8",
		"FINAL,PROXY",
	}, "\n")
	rs := ruleLineListParser{}.Parse(content)
	if len(rs.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(rs.Entries), rs.Entries)
	}
	if len(rs.Diagnostics) != 5 {
		t.Fatalf("want 5 diagnostics, got %d: %+v", len(rs.Diagnostics), rs.Diagnostics)
	}
	for _, d := range rs.Diagnostics {
		if d.Line == 0 || d.Reason == "" {
			t.Errorf("diagnostic missing line/reason: %+v", d)
		}
	}
}

func TestClassicalBareIPCIDRGainsPrefix(t *testing.T) {
	e := parseOne(t, "IP-CIDR,8.8.8.8")
	if e.Value != "8.8.8.8/32" {
		t.Fatalf("bare IPv4: %q", e.Value)
	}
	e = parseOne(t, "IP-CIDR6,2001:db8::1")
	if e.Value != "2001:db8::1/128" {
		t.Fatalf("bare IPv6: %q", e.Value)
	}
}
