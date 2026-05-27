package transformer

import (
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func TestBuiltinTransformers_RegistryHasBoth(t *testing.T) {
	reg := BuiltinTransformers()
	for _, name := range []string{BuiltinMihomoClassicalToYAML, BuiltinMihomoToShadowrocket} {
		if _, ok := reg[name]; !ok {
			t.Fatalf("expected %q in registry, got keys %v", name, keysOf(reg))
		}
		if !IsBuiltinName(name) {
			t.Fatalf("IsBuiltinName(%q) returned false", name)
		}
	}
}

func TestHasBuiltinPrefix(t *testing.T) {
	cases := map[string]bool{
		"builtin:foo":                   true,
		"builtin:":                      true,
		"builtin":                       false,
		"":                              false,
		BuiltinMihomoClassicalToYAML:    true,
		"user:builtin:something":        false,
		"BuiltIn:Mihomo-Classical-Yaml": false, // case sensitive
	}
	for in, want := range cases {
		if got := HasBuiltinPrefix(in); got != want {
			t.Errorf("HasBuiltinPrefix(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRunMihomoClassicalToYAML(t *testing.T) {
	input := strings.Join([]string{
		"# header comment",
		"",
		"DOMAIN,example.com",
		"DOMAIN-SUFFIX,google.com,no-resolve",
		"IP-CIDR,1.1.1.1/32,DIRECT",
		"DOMAIN-KEYWORD,it's-quoted", // single quote inside value
	}, "\n")

	res, ok := RunBuiltin(BuiltinMihomoClassicalToYAML, nil, input)
	if !ok {
		t.Fatal("expected builtin to be dispatched")
	}
	out := res.Output

	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
	// Output should start with `payload:` and contain each rule line as a
	// single-quoted YAML scalar (with `'` escaped to `''`).
	wantLines := []string{
		"payload:",
		"  - 'DOMAIN,example.com'",
		"  - 'DOMAIN-SUFFIX,google.com,no-resolve'",
		"  - 'IP-CIDR,1.1.1.1/32,DIRECT'",
		"  - 'DOMAIN-KEYWORD,it''s-quoted'",
		"", // trailing newline produces a final empty element
	}
	if got := strings.Split(out, "\n"); !equalStringSlice(got, wantLines) {
		t.Fatalf("unexpected yaml output:\n%s\n\nwant:\n%s", out, strings.Join(wantLines, "\n"))
	}

	if res.DroppedTotal == 0 {
		t.Fatalf("expected at least one dropped comment line, got 0")
	}
	if len(res.Modified) != 0 {
		t.Fatalf("expected no modified samples for classical→yaml, got %d", len(res.Modified))
	}
	// First dropped sample should describe the comment.
	if len(res.Dropped) == 0 || res.Dropped[0].LineNo != 1 {
		t.Fatalf("expected first dropped line at LineNo=1, got %+v", res.Dropped)
	}
}

func TestRunMihomoClassicalToYAML_EmptyInput(t *testing.T) {
	for _, input := range []string{"", "   \n  \n"} {
		res, ok := RunBuiltin(BuiltinMihomoClassicalToYAML, nil, input)
		if !ok {
			t.Fatal("expected dispatch")
		}
		if res.Output != "payload: []\n" {
			t.Errorf("input %q → unexpected output %q", input, res.Output)
		}
	}
}

func TestRunMihomoClassicalToYAML_OnlyComments(t *testing.T) {
	input := "# only comments\n# all the way down\n"
	res, ok := RunBuiltin(BuiltinMihomoClassicalToYAML, nil, input)
	if !ok {
		t.Fatal("expected dispatch")
	}
	if res.Output != "payload: []\n" {
		t.Errorf("expected empty payload, got %q", res.Output)
	}
	if res.DroppedTotal != 2 {
		t.Errorf("expected 2 dropped, got %d", res.DroppedTotal)
	}
}

func TestRunMihomoToShadowrocket_TableDriven(t *testing.T) {
	type row struct {
		name       string
		in         string
		wantKeep   bool   // line should appear (possibly rewritten) in output
		wantRename string // non-empty → expect modified-line capture with this `to`
		wantReason string // non-empty → expect dropped-line capture matching this reason prefix
	}
	cases := []row{
		{name: "DOMAIN passthrough", in: "DOMAIN,example.com", wantKeep: true},
		{name: "DOMAIN-SUFFIX passthrough", in: "DOMAIN-SUFFIX,google.com", wantKeep: true},
		{name: "DOMAIN-KEYWORD passthrough", in: "DOMAIN-KEYWORD,ads", wantKeep: true},
		{name: "IP-CIDR passthrough", in: "IP-CIDR,1.1.1.1/32,no-resolve", wantKeep: true},
		{name: "IP-CIDR6 passthrough", in: "IP-CIDR6,2001:db8::/32", wantKeep: true},
		{name: "GEOIP passthrough", in: "GEOIP,CN", wantKeep: true},
		{name: "SRC-IP-CIDR passthrough", in: "SRC-IP-CIDR,10.0.0.0/8", wantKeep: true},
		{name: "DST-PORT passthrough", in: "DST-PORT,8080", wantKeep: true},
		{name: "SRC-PORT passthrough", in: "SRC-PORT,12345", wantKeep: true},
		{name: "IN-PORT passthrough", in: "IN-PORT,7890", wantKeep: true},
		{name: "PROTOCOL passthrough", in: "PROTOCOL,tcp", wantKeep: true},
		{name: "NETWORK passthrough", in: "NETWORK,udp", wantKeep: true},
		{name: "USER-AGENT passthrough", in: "USER-AGENT,Mozilla/5.0", wantKeep: true},
		{name: "URL-REGEX passthrough", in: "URL-REGEX,^https?://", wantKeep: true},
		{name: "MATCH renames to FINAL", in: "MATCH,DIRECT", wantKeep: true, wantRename: "FINAL,DIRECT"},
		{name: "FINAL passthrough", in: "FINAL,PROXY", wantKeep: true},
		{name: "PROCESS-NAME dropped", in: "PROCESS-NAME,Telegram", wantReason: "PROCESS-NAME"},
		{name: "PROCESS-PATH dropped", in: "PROCESS-PATH,/usr/bin/curl", wantReason: "PROCESS-PATH"},
		{name: "IP-ASN dropped", in: "IP-ASN,13335", wantReason: "IP-ASN"},
		{name: "DOMAIN-REGEX dropped", in: "DOMAIN-REGEX,^ads", wantReason: "DOMAIN-REGEX"},
		{name: "RULE-SET dropped", in: "RULE-SET,my-set,DIRECT", wantReason: "RULE-SET"},
		{name: "SUB-RULE dropped", in: "SUB-RULE,...", wantReason: "SUB-RULE"},
		{name: "AND dropped", in: "AND,((DOMAIN,a.com),(DOMAIN,b.com)),DIRECT", wantReason: "AND"},
		{name: "OR dropped", in: "OR,((DOMAIN,a.com),(DOMAIN,b.com)),DIRECT", wantReason: "OR"},
		{name: "NOT dropped", in: "NOT,((DOMAIN,a.com)),DIRECT", wantReason: "NOT"},
		{name: "comment dropped", in: "# leading comment", wantReason: "comment removed"},
		// Unknown rule types now pass through by default (UnknownAction=keep)
		// so that brand-new mihomo tokens like DOMAIN-NEW don't silently
		// vanish from a Shadowrocket output. Operators who want strict
		// behaviour can flip `unknownAction` to "drop" in params.
		{name: "unknown type passthrough", in: "DOMAIN-NEW,future.example", wantKeep: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, ok := RunBuiltin(BuiltinMihomoToShadowrocket, nil, c.in)
			if !ok {
				t.Fatal("expected dispatch")
			}
			out := strings.TrimSpace(res.Output)
			if c.wantKeep {
				if out == "" {
					t.Fatalf("expected output to contain a line, got empty (in=%q)", c.in)
				}
				if c.wantRename != "" {
					if out != c.wantRename {
						t.Fatalf("expected rename to %q, got %q", c.wantRename, out)
					}
					if len(res.Modified) != 1 {
						t.Fatalf("expected 1 modified entry, got %d", len(res.Modified))
					}
					if res.Modified[0].From != c.in || res.Modified[0].To != c.wantRename {
						t.Fatalf("modified mismatch: %+v", res.Modified[0])
					}
				}
			}
			if c.wantReason != "" {
				if len(res.Dropped) != 1 {
					t.Fatalf("expected 1 dropped entry, got %d (out=%q)", len(res.Dropped), out)
				}
				if !strings.Contains(res.Dropped[0].Reason, c.wantReason) {
					t.Fatalf("reason %q does not contain %q", res.Dropped[0].Reason, c.wantReason)
				}
				if out != "" {
					t.Fatalf("expected empty output for dropped rule, got %q", out)
				}
			}
		})
	}
}

func TestRunMihomoToShadowrocket_PreservesMultilineOrder(t *testing.T) {
	input := strings.Join([]string{
		"DOMAIN,a.com",
		"PROCESS-NAME,bad",
		"MATCH,DIRECT",
		"IP-CIDR,1.1.1.1/32",
	}, "\n")
	res, ok := RunBuiltin(BuiltinMihomoToShadowrocket, nil, input)
	if !ok {
		t.Fatal("expected dispatch")
	}
	wantLines := []string{
		"DOMAIN,a.com",
		"FINAL,DIRECT",
		"IP-CIDR,1.1.1.1/32",
		"", // trailing newline
	}
	gotLines := strings.Split(res.Output, "\n")
	if !equalStringSlice(gotLines, wantLines) {
		t.Fatalf("order mismatch: got %v want %v", gotLines, wantLines)
	}
	if res.DroppedTotal != 1 || res.ModifiedTotal != 1 {
		t.Fatalf("expected 1 drop + 1 modify, got %d / %d", res.DroppedTotal, res.ModifiedTotal)
	}
}

func TestMergeBuiltinTransformers_UserCannotOverrideReservedPrefix(t *testing.T) {
	user := map[string]schema.ScriptTransformer{
		"hello":                      {Name: "hello", Script: "// ok"},
		BuiltinMihomoClassicalToYAML: {Name: BuiltinMihomoClassicalToYAML, Script: "// user override attempt"},
		"builtin:doesnotexist":       {Name: "builtin:doesnotexist", Script: "// also reserved"},
	}
	merged := MergeBuiltinTransformers(user)
	if got := merged["hello"].Script; got != "// ok" {
		t.Errorf("user transformer dropped: %q", got)
	}
	if got := merged[BuiltinMihomoClassicalToYAML].Script; got != "" {
		t.Errorf("expected empty (built-in placeholder), got %q", got)
	}
	if _, ok := merged["builtin:doesnotexist"]; ok {
		t.Errorf("reserved-prefix user transformer should have been dropped")
	}
}

func TestRunBuiltin_UnknownName(t *testing.T) {
	res, ok := RunBuiltin("builtin:not-registered", nil, "DOMAIN,a.com")
	if ok {
		t.Fatal("expected ok=false for unknown builtin name")
	}
	if res.Output != "DOMAIN,a.com" {
		t.Fatalf("expected content passthrough, got %q", res.Output)
	}
}

// TestRunMihomoToShadowrocket_CustomParams verifies that user-supplied
// params override the default mapping: here we tell the transformer to
// rename `DOMAIN` to `HOST` and drop `IP-CIDR` even though the defaults
// keep both untouched.
func TestRunMihomoToShadowrocket_CustomParams(t *testing.T) {
	params := []byte(`{
	  "unknownAction": "drop",
	  "rules": [
	    {"type": "DOMAIN", "action": "rename", "renameTo": "HOST", "reason": "test rename"},
	    {"type": "IP-CIDR", "action": "drop", "reason": "test drop"}
	  ]
	}`)
	input := strings.Join([]string{
		"DOMAIN,a.com",
		"IP-CIDR,1.1.1.1/32",
		"WAT,unknown", // unknownAction=drop → dropped
	}, "\n")
	res, ok := RunBuiltin(BuiltinMihomoToShadowrocket, params, input)
	if !ok {
		t.Fatal("expected dispatch")
	}
	if got, want := strings.TrimSpace(res.Output), "HOST,a.com"; got != want {
		t.Fatalf("output mismatch: got %q want %q", got, want)
	}
	if res.ModifiedTotal != 1 {
		t.Fatalf("expected 1 rename, got %d", res.ModifiedTotal)
	}
	if res.DroppedTotal != 2 {
		t.Fatalf("expected 2 drops (IP-CIDR + WAT), got %d", res.DroppedTotal)
	}
}

// TestRunMihomoToShadowrocket_UnknownActionKeep verifies the default
// behaviour: rules whose type isn't in the mapping table pass through
// unchanged when params (or UnknownAction) is empty.
func TestRunMihomoToShadowrocket_UnknownActionKeep(t *testing.T) {
	input := "DOMAIN-NEW,future.example"
	res, ok := RunBuiltin(BuiltinMihomoToShadowrocket, nil, input)
	if !ok {
		t.Fatal("expected dispatch")
	}
	if got := strings.TrimSpace(res.Output); got != input {
		t.Fatalf("expected passthrough %q, got %q", input, got)
	}
	if res.DroppedTotal != 0 {
		t.Fatalf("expected 0 drops, got %d", res.DroppedTotal)
	}
}

func keysOf(m map[string]schema.ScriptTransformer) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
