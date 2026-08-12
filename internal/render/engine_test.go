package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"gopkg.in/yaml.v3"
)

func mihomoClassicalTemplate() *Template {
	return &Template{
		ID:        "mihomo-classical",
		Codec:     "linelist",
		Extension: ".list",
		KindMap: map[string]KindMapping{
			"domain":         {TypeName: "DOMAIN"},
			"domain_suffix":  {TypeName: "DOMAIN-SUFFIX"},
			"domain_keyword": {TypeName: "DOMAIN-KEYWORD"},
			"ip_cidr":        {TypeName: "IP-CIDR"},
		},
		FlagKinds: []string{"ip_cidr"},
		Hints: map[string]KindHint{
			"ip_cidr": {IPv6TypeName: "IP-CIDR6"},
		},
	}
}

func singboxTemplate() *Template {
	return &Template{
		ID:        "singbox",
		Codec:     "singbox",
		Extension: ".json",
		KindMap: map[string]KindMapping{
			"domain":         {FieldName: "domain"},
			"domain_suffix":  {FieldName: "domain_suffix"},
			"domain_keyword": {FieldName: "domain_keyword"},
			"ip_cidr":        {FieldName: "ip_cidr"},
			"dst_port":       {FieldName: "port"},
			"dst_port_range": {FieldName: "port_range"},
		},
		FieldGroups: []FieldGroup{
			{Name: "domain", Fields: []string{"domain", "domain_suffix", "domain_keyword", "ip_cidr"}},
		},
	}
}

func TestRenderLineList(t *testing.T) {
	tmpl := mihomoClassicalTemplate()
	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "google.com"},
		{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
		{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8", Flags: []string{ir.FlagNoResolve}},
		{Kind: ir.KindIPCIDR, Value: "192.0.2.0/24"},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "DOMAIN,google.com") {
		t.Errorf("missing DOMAIN line: %s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,youtube.com") {
		t.Errorf("missing DOMAIN-SUFFIX line: %s", s)
	}
	if !strings.Contains(s, "IP-CIDR,10.0.0.0/8,no-resolve") {
		t.Errorf("missing IP-CIDR line with no-resolve: %s", s)
	}
	if !strings.Contains(s, "IP-CIDR,192.0.2.0/24\n") {
		t.Errorf("unflagged IP-CIDR gained no-resolve: %s", s)
	}
}

func TestRenderIPv6TypeName(t *testing.T) {
	tmpl := mihomoClassicalTemplate()
	entries := []ir.Entry{
		{Kind: ir.KindIPCIDR, Value: "2001:db8::/32", Flags: []string{ir.FlagNoResolve}},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "IP-CIDR6,2001:db8::/32,no-resolve") {
		t.Errorf("expected IP-CIDR6 for IPv6: %s", s)
	}
}

func TestRenderYAMLPayload(t *testing.T) {
	tmpl := &Template{
		ID:        "mihomo-yaml",
		Codec:     "yaml_payload",
		Extension: ".yaml",
		KindMap: map[string]KindMapping{
			"domain":        {TypeName: "DOMAIN"},
			"domain_suffix": {TypeName: "DOMAIN-SUFFIX"},
		},
	}
	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "example.com"},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "payload:\n") {
		t.Errorf("missing payload header: %s", s)
	}
	if !strings.Contains(s, "  - 'DOMAIN,example.com'") {
		t.Errorf("missing payload entry: %s", s)
	}
}

func TestRenderYAMLPayloadEscapesSingleQuotes(t *testing.T) {
	tmpl := &Template{
		ID:        "yaml-quotes",
		Codec:     "yaml_payload",
		Extension: ".yaml",
		KindMap: map[string]KindMapping{
			"domain_regex": {TypeName: "DOMAIN-REGEX"},
		},
	}
	out, err := Render(tmpl, []ir.Entry{{Kind: ir.KindDomainRegex, Value: "^foo'bar$"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Payload []string `yaml:"payload"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	if len(doc.Payload) != 1 || doc.Payload[0] != "DOMAIN-REGEX,^foo'bar$" {
		t.Fatalf("payload: %#v", doc.Payload)
	}
}

func TestRenderReturnsEmptyWhenTemplateSupportsNoEntries(t *testing.T) {
	for _, codec := range []string{"linelist", "yaml_payload", "singbox"} {
		t.Run(codec, func(t *testing.T) {
			tmpl := &Template{
				ID:        codec,
				Codec:     codec,
				Extension: ".out",
				KindMap: map[string]KindMapping{
					"domain": {TypeName: "DOMAIN", FieldName: "domain"},
				},
			}
			out, err := Render(tmpl, []ir.Entry{{Kind: ir.KindIPCIDR, Value: "192.0.2.0/24"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 0 {
				t.Fatalf("empty render produced %q", out)
			}
		})
	}
}

func TestRenderSingbox(t *testing.T) {
	tmpl := singboxTemplate()
	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "google.com"},
		{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
		{Kind: ir.KindDstPort, Value: "443/1000-2000"},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc["version"] != float64(3) {
		t.Errorf("version = %v, want 3", doc["version"])
	}
	rules, ok := doc["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatal("expected at least one rule")
	}
	var portRule map[string]any
	for _, value := range rules {
		rule := value.(map[string]any)
		if rule["port"] != nil {
			portRule = rule
		}
	}
	if portRule == nil || portRule["port"] != float64(443) || portRule["port_range"] != "1000:2000" {
		t.Fatalf("port rule: %#v", portRule)
	}
}

func TestRenderRejectsLogicalEntries(t *testing.T) {
	tmpl := mihomoClassicalTemplate()
	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "example.com"},
		{Kind: ir.KindAnd, Sub: []ir.Entry{
			{Kind: ir.KindDomain, Value: "and-child.com"},
		}},
	}
	if _, err := Render(tmpl, entries); err == nil {
		t.Fatal("logical entry was silently discarded")
	}
}

func TestRenderSingboxLogical(t *testing.T) {
	tmpl := singboxTemplate()
	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "direct.example.com"},
		{Kind: ir.KindAnd, Sub: []ir.Entry{
			{Kind: ir.KindDomainSuffix, Value: "google.com"},
			{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8"},
		}},
		{Kind: ir.KindNot, Sub: []ir.Entry{
			{Kind: ir.KindDomain, Value: "ads.example.com"},
		}},
		{Kind: ir.KindOr, Sub: []ir.Entry{
			{Kind: ir.KindDomainSuffix, Value: "google.com"},
			{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
		}},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc["version"] != float64(3) {
		t.Errorf("version = %v, want 3", doc["version"])
	}
	rules, ok := doc["rules"].([]any)
	if !ok {
		t.Fatalf("rules is not a slice: %T", doc["rules"])
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules (1 flat + 3 logical), got %d", len(rules))
	}

	// Rule 0: flat domain entry (grouped)
	flatRule := rules[0].(map[string]any)
	if flatRule["domain"] != "direct.example.com" {
		t.Errorf("flat rule: %#v", flatRule)
	}

	// Rule 1: AND logical
	andRule := rules[1].(map[string]any)
	if andRule["type"] != "logical" || andRule["mode"] != "and" {
		t.Errorf("AND rule: %#v", andRule)
	}
	andSubs := andRule["rules"].([]any)
	if len(andSubs) != 2 {
		t.Fatalf("AND sub-rules: %d", len(andSubs))
	}
	andSub0 := andSubs[0].(map[string]any)
	if andSub0["domain_suffix"] != "google.com" {
		t.Errorf("AND sub 0: %#v", andSub0)
	}
	andSub1 := andSubs[1].(map[string]any)
	if andSub1["ip_cidr"] != "10.0.0.0/8" {
		t.Errorf("AND sub 1: %#v", andSub1)
	}

	// Rule 2: NOT logical
	notRule := rules[2].(map[string]any)
	if notRule["type"] != "logical" || notRule["mode"] != "not" {
		t.Errorf("NOT rule: %#v", notRule)
	}
	notSubs := notRule["rules"].([]any)
	if len(notSubs) != 1 {
		t.Fatalf("NOT sub-rules: %d", len(notSubs))
	}
	notSub0 := notSubs[0].(map[string]any)
	if notSub0["domain"] != "ads.example.com" {
		t.Errorf("NOT sub 0: %#v", notSub0)
	}

	// Rule 3: OR logical
	orRule := rules[3].(map[string]any)
	if orRule["type"] != "logical" || orRule["mode"] != "or" {
		t.Errorf("OR rule: %#v", orRule)
	}
	orSubs := orRule["rules"].([]any)
	if len(orSubs) != 2 {
		t.Fatalf("OR sub-rules: %d", len(orSubs))
	}
}

func TestRenderSingboxNestedLogical(t *testing.T) {
	tmpl := singboxTemplate()
	// AND of (OR of two domain_suffix) and (ip_cidr)
	entries := []ir.Entry{
		{Kind: ir.KindAnd, Sub: []ir.Entry{
			{Kind: ir.KindOr, Sub: []ir.Entry{
				{Kind: ir.KindDomainSuffix, Value: "google.com"},
				{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
			}},
			{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8"},
		}},
	}
	out, err := Render(tmpl, entries)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	rules := doc["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	andRule := rules[0].(map[string]any)
	if andRule["type"] != "logical" || andRule["mode"] != "and" {
		t.Fatalf("AND rule: %#v", andRule)
	}
	andSubs := andRule["rules"].([]any)
	if len(andSubs) != 2 {
		t.Fatalf("AND sub-rules: %d", len(andSubs))
	}
	// First sub is nested OR
	nestedOr := andSubs[0].(map[string]any)
	if nestedOr["type"] != "logical" || nestedOr["mode"] != "or" {
		t.Errorf("nested OR: %#v", nestedOr)
	}
	orSubs := nestedOr["rules"].([]any)
	if len(orSubs) != 2 {
		t.Fatalf("nested OR sub-rules: %d", len(orSubs))
	}
	// Second sub is ip_cidr
	ipSub := andSubs[1].(map[string]any)
	if ipSub["ip_cidr"] != "10.0.0.0/8" {
		t.Errorf("ip_cidr sub: %#v", ipSub)
	}
}
