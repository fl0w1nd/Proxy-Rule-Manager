package transformer

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestRunMihomoClassicalToSingboxSource_Defaults walks the curated
// mapping table and asserts that one rule per supported sing-box field
// lands in the output. The expected JSON is asserted *structurally*
// (parse + compare) rather than as a byte string so the test stays
// resilient against incidental whitespace changes in the writer.
func TestRunMihomoClassicalToSingboxSource_Defaults(t *testing.T) {
	input := strings.Join([]string{
		"# leading comment, should be dropped",
		"",
		"DOMAIN,a.com",
		"DOMAIN-SUFFIX,google.com",
		"DOMAIN-KEYWORD,ads",
		"DOMAIN-REGEX,^foo.*",
		"IP-CIDR,1.1.1.1/32",
		"IP-CIDR6,2001:db8::/32",
		"IP-SUFFIX,8.8.8.8/24",
		"SRC-IP-CIDR,10.0.0.0/8",
		"DST-PORT,80",
		"DST-PORT,443",
		"SRC-PORT,12345",
		"PROCESS-NAME,Telegram",
		"PROCESS-PATH,/usr/bin/curl",
		"PROCESS-PATH-REGEX,^/usr/bin/.+",
		"NETWORK,udp",
		"GEOSITE,cn,PROXY", // dropped by default mapping
	}, "\n")

	res, ok := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, input)
	if !ok {
		t.Fatal("expected dispatch")
	}

	var got struct {
		Version int                      `json:"version"`
		Rules   []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(res.Output), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, res.Output)
	}
	if got.Version != DefaultSingboxSourceVersion {
		t.Fatalf("expected default version %d, got %d", DefaultSingboxSourceVersion, got.Version)
	}

	// Walk the produced rules and re-index by field name so the test
	// asserts on contents rather than positional order.
	byField := make(map[string][]interface{})
	for _, r := range got.Rules {
		for k, v := range r {
			byField[k] = v.([]interface{})
		}
	}

	want := map[string][]interface{}{
		"domain":             {"a.com"},
		"domain_suffix":      {"google.com"},
		"domain_keyword":     {"ads"},
		"domain_regex":       {"^foo.*"},
		"ip_cidr":            {"1.1.1.1/32", "2001:db8::/32", "8.8.8.8/24"},
		"source_ip_cidr":     {"10.0.0.0/8"},
		"port":               {float64(80), float64(443)},
		"source_port":        {float64(12345)},
		"process_name":       {"Telegram"},
		"process_path":       {"/usr/bin/curl"},
		"process_path_regex": {"^/usr/bin/.+"},
		"network":            {"udp"},
	}
	for field, expected := range want {
		actual, ok := byField[field]
		if !ok {
			t.Errorf("missing field %q in output rules", field)
			continue
		}
		if len(actual) != len(expected) {
			t.Errorf("field %q: expected %d values, got %d (%v)", field, len(expected), len(actual), actual)
			continue
		}
		for i := range expected {
			if actual[i] != expected[i] {
				t.Errorf("field %q[%d]: expected %v, got %v", field, i, expected[i], actual[i])
			}
		}
	}

	// GEOSITE drop and the comment drop should have surfaced in the
	// dropped track.
	if res.DroppedTotal < 2 {
		t.Errorf("expected at least 2 dropped (comment + GEOSITE), got %d", res.DroppedTotal)
	}
	if res.ModifiedTotal == 0 {
		t.Errorf("expected ModifiedTotal > 0 to record per-line rewrites")
	}
}

// TestRunMihomoClassicalToSingboxSource_PolicyAndModifierStripped guards
// the contract that trailing policy/modifier columns are discarded.
// sing-box rule-sets have no policy slot — the matching `route.rules[]`
// entry decides the outbound — so leaving them in would corrupt the
// value.
func TestRunMihomoClassicalToSingboxSource_PolicyAndModifierStripped(t *testing.T) {
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, strings.Join([]string{
		"DOMAIN-SUFFIX,google.com,PROXY",
		"IP-CIDR,1.1.1.1/32,DIRECT,no-resolve",
	}, "\n"))
	if !strings.Contains(res.Output, `"google.com"`) {
		t.Fatalf("expected unsuffixed domain value, got:\n%s", res.Output)
	}
	if strings.Contains(res.Output, "PROXY") || strings.Contains(res.Output, "no-resolve") {
		t.Fatalf("policy/modifier leaked into output:\n%s", res.Output)
	}
}

// TestRunMihomoClassicalToSingboxSource_Empty asserts the writer emits a
// schema-valid skeleton even when the input is empty / comments-only.
// Returning an invalid empty string would break `sing-box rule-set
// compile` downstream.
func TestRunMihomoClassicalToSingboxSource_Empty(t *testing.T) {
	for _, in := range []string{"", "# only comments\n# nothing else\n"} {
		res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, in)
		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(res.Output), &doc); err != nil {
			t.Fatalf("input %q → invalid JSON: %v\nOutput: %s", in, err, res.Output)
		}
		if doc["version"].(float64) != float64(DefaultSingboxSourceVersion) {
			t.Errorf("input %q: wrong version", in)
		}
		rules, ok := doc["rules"].([]interface{})
		if !ok {
			t.Fatalf("input %q: rules is not an array: %T", in, doc["rules"])
		}
		if len(rules) != 0 {
			t.Errorf("input %q: expected empty rules, got %d entries", in, len(rules))
		}
	}
}

// TestRunMihomoClassicalToSingboxSource_PortValidation rejects values
// that cannot be coerced to a port-shaped int. We also check the
// boundary cases (0 and 65535 are accepted, 65536 is not).
func TestRunMihomoClassicalToSingboxSource_PortValidation(t *testing.T) {
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, strings.Join([]string{
		"DST-PORT,0",
		"DST-PORT,65535",
		"DST-PORT,65536",
		"DST-PORT,nope",
		"DST-PORT,-1",
	}, "\n"))
	if res.DroppedTotal != 3 {
		t.Fatalf("expected 3 drops (65536, nope, -1), got %d (samples=%+v)", res.DroppedTotal, res.Dropped)
	}
	if !strings.Contains(res.Output, `"port": [0, 65535]`) {
		t.Fatalf("expected port=[0,65535], got:\n%s", res.Output)
	}
}

// TestRunMihomoClassicalToSingboxSource_DedupesValues guards the writer
// against emitting the same value twice when the input repeats it.
func TestRunMihomoClassicalToSingboxSource_DedupesValues(t *testing.T) {
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, strings.Join([]string{
		"DOMAIN,a.com",
		"DOMAIN,a.com",
		"DST-PORT,80",
		"DST-PORT,80",
	}, "\n"))
	occ := strings.Count(res.Output, `"a.com"`)
	if occ != 1 {
		t.Fatalf("expected a.com to appear once, got %d times:\n%s", occ, res.Output)
	}
	if strings.Count(res.Output, "80") != 1 {
		t.Fatalf("expected port 80 to appear once, got:\n%s", res.Output)
	}
}

// TestRunMihomoClassicalToSingboxSource_CustomParams ensures the user
// can swap the default table out — here we route DOMAIN values into
// domain_keyword and drop all IP-CIDR rules — and pick a non-default
// version.
func TestRunMihomoClassicalToSingboxSource_CustomParams(t *testing.T) {
	params := []byte(`{
	  "version": 1,
	  "rules": [
	    {"type": "DOMAIN", "action": "map", "mapTo": "domain_keyword"},
	    {"type": "IP-CIDR", "action": "drop", "reason": "test drop"}
	  ]
	}`)
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, params, strings.Join([]string{
		"DOMAIN,a.com",
		"IP-CIDR,1.1.1.1/32",
	}, "\n"))
	var doc struct {
		Version int                      `json:"version"`
		Rules   []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(res.Output), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc.Version != 1 {
		t.Fatalf("expected version=1, got %d", doc.Version)
	}
	found := false
	for _, r := range doc.Rules {
		if v, ok := r["domain_keyword"]; ok {
			arr := v.([]interface{})
			if len(arr) == 1 && arr[0] == "a.com" {
				found = true
			}
		}
		if _, ok := r["ip_cidr"]; ok {
			t.Fatalf("ip_cidr should have been dropped: %v", r)
		}
	}
	if !found {
		t.Fatalf("expected domain_keyword=[a.com] in output: %s", res.Output)
	}
	if res.DroppedTotal != 1 {
		t.Fatalf("expected 1 drop (IP-CIDR), got %d", res.DroppedTotal)
	}
}

// TestRunMihomoClassicalToSingboxSource_UnknownTypesDropped guards the
// "unknown action is drop" contract. sing-box rule-set readers reject
// unknown keys outright, so silently keeping a mihomo-only token would
// be worse than producing a smaller (but valid) document.
func TestRunMihomoClassicalToSingboxSource_UnknownTypesDropped(t *testing.T) {
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, "DOMAIN-NEW,future.example")
	if !strings.Contains(res.Output, `"rules": []`) {
		t.Fatalf("expected empty rules, got: %s", res.Output)
	}
	if res.DroppedTotal != 1 {
		t.Fatalf("expected 1 drop, got %d", res.DroppedTotal)
	}
	if !strings.Contains(res.Dropped[0].Reason, "未在映射表") {
		t.Fatalf("expected default-drop reason, got %q", res.Dropped[0].Reason)
	}
}

// TestRunMihomoClassicalToSingboxSource_InvalidMapToDropped covers the
// scenario where the persisted mapping table got out of sync with the
// runner (e.g. a sing-box field was removed in a future release). The
// row's action stays "map" but the target field is no longer valid;
// the runner must drop the line rather than emit an unknown JSON key.
func TestRunMihomoClassicalToSingboxSource_InvalidMapToDropped(t *testing.T) {
	params := []byte(`{
	  "rules": [{"type": "DOMAIN", "action": "map", "mapTo": "not_a_real_field"}]
	}`)
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, params, "DOMAIN,a.com")
	if !strings.Contains(res.Output, `"rules": []`) {
		t.Fatalf("expected empty rules, got: %s", res.Output)
	}
	if res.DroppedTotal != 1 || !strings.Contains(res.Dropped[0].Reason, "not_a_real_field") {
		t.Fatalf("expected drop with invalid mapTo reason, got %+v", res.Dropped)
	}
}

// TestSingboxSourceFields_Coverage asserts the field allow-list and the
// canonical emit order stay in lockstep. A new sing-box field added to
// the kinds map without a matching slot in singboxFieldOrder would be
// silently un-emitable, which is exactly the bug this guards against.
func TestSingboxSourceFields_Coverage(t *testing.T) {
	if len(SingboxSourceFields()) != len(singboxFieldKinds) {
		t.Fatalf("field-order list and kind map are out of sync: order=%d kinds=%d",
			len(SingboxSourceFields()), len(singboxFieldKinds))
	}
	for _, f := range SingboxSourceFields() {
		if !IsSingboxSourceField(f) {
			t.Errorf("IsSingboxSourceField(%q) returned false", f)
		}
	}
}

// TestRunMihomoClassicalToSingboxSource_ExplicitEmptyRulesHonoured is a
// regression test for the bug where saving `{"rules": []}` silently
// fell back to the curated default mapping. The user intent — "drop
// every classical rule type, no exceptions" — must reach the runner.
// json.Unmarshal of `{"rules": []}` leaves the field as a non-nil
// empty slice, which the decoder now treats verbatim.
func TestRunMihomoClassicalToSingboxSource_ExplicitEmptyRulesHonoured(t *testing.T) {
	params := json.RawMessage(`{"rules": []}`)
	res, ok := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, params, strings.Join([]string{
		"DOMAIN,a.com",
		"DOMAIN-SUFFIX,google.com",
		"IP-CIDR,1.1.1.1/32",
	}, "\n"))
	if !ok {
		t.Fatal("expected dispatch")
	}
	// All three input rules should have been dropped because the
	// (explicit) empty mapping table treats every type as unknown.
	if res.DroppedTotal != 3 {
		t.Fatalf("expected 3 drops (every input row), got %d (samples=%+v)", res.DroppedTotal, res.Dropped)
	}
	if !strings.Contains(res.Output, `"rules": []`) {
		t.Fatalf("expected empty rules in output, got:\n%s", res.Output)
	}
	// Sanity check the contrapositive: an *implicit* empty (no rules
	// key at all) should still fall back to the defaults so a fresh
	// install / first preview render does something useful.
	res2, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, json.RawMessage(`{}`), "DOMAIN,a.com")
	if !strings.Contains(res2.Output, `"a.com"`) {
		t.Fatalf("implicit empty rules should fall back to defaults, got:\n%s", res2.Output)
	}
}

// TestRunMihomoClassicalToSingboxSource_VersionGatesField is a
// regression test for the bug where the runner emitted higher-version
// fields (e.g. process_path_regex requires rule-set v2) under a lower
// declared version. The output would then fail to compile on the
// targeted sing-box release. The runner is the second line of defence
// here; the validator (TestValidateSingboxSourceParams_RejectsBadShapes)
// covers the save-time rejection.
func TestRunMihomoClassicalToSingboxSource_VersionGatesField(t *testing.T) {
	type row struct {
		name  string
		field string
		ver   int // sing-box rule-set version to declare
	}
	cases := []row{
		{"process_path_regex requires v2", "process_path_regex", 1},
		{"network_type requires v3", "network_type", 2},
		{"wifi_ssid requires v3", "wifi_ssid", 2},
		{"package_name_regex requires v5", "package_name_regex", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := []byte(`{
				"version": ` + strconv.Itoa(c.ver) + `,
				"rules": [{"type": "X", "action": "map", "mapTo": "` + c.field + `"}]
			}`)
			res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, params, "X,value")
			if !strings.Contains(res.Output, `"rules": []`) {
				t.Fatalf("expected empty rules (field gated out), got: %s", res.Output)
			}
			if res.DroppedTotal != 1 {
				t.Fatalf("expected 1 drop, got %d", res.DroppedTotal)
			}
			if !strings.Contains(res.Dropped[0].Reason, "version") {
				t.Fatalf("expected version-gate reason, got: %s", res.Dropped[0].Reason)
			}
		})
	}
}

// TestExtractClassicalValue_RegexCommaPreserved is the regression test
// for the reviewer's "naive first-comma truncation corrupts regex
// values" finding. The previous implementation split at the first
// comma unconditionally, which turned `^a{1,3}\.com$` into the
// nonsense `^a{1`. The fixed extractor branches on rule type: regex
// types pop trailing modifiers and at most one policy from the
// right, leaving commas inside the regex intact.
func TestExtractClassicalValue_RegexCommaPreserved(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		rest string
		want string
	}{
		// --- regex types: commas inside the value must survive ---
		{
			name: "DOMAIN-REGEX with quantifier and no policy",
			typ:  "DOMAIN-REGEX",
			rest: `^a{1,3}\.com$`,
			want: `^a{1,3}\.com$`,
		},
		{
			name: "DOMAIN-REGEX with quantifier and trailing policy",
			typ:  "DOMAIN-REGEX",
			rest: `^a{1,3}\.com$,PROXY`,
			want: `^a{1,3}\.com$`,
		},
		{
			name: "DOMAIN-REGEX with quantifier, policy, and no-resolve",
			typ:  "DOMAIN-REGEX",
			rest: `^a{1,3}\.com$,PROXY,no-resolve`,
			want: `^a{1,3}\.com$`,
		},
		{
			name: "DOMAIN-REGEX with multi-segment quantifier",
			typ:  "DOMAIN-REGEX",
			rest: `^(foo|bar){2,5}\.example\.com$`,
			want: `^(foo|bar){2,5}\.example\.com$`,
		},
		{
			name: "DOMAIN-REGEX with multi-segment quantifier and policy",
			typ:  "DOMAIN-REGEX",
			rest: `^(foo|bar){2,5}\.example\.com$,DIRECT`,
			want: `^(foo|bar){2,5}\.example\.com$`,
		},
		{
			name: "PROCESS-PATH-REGEX with quantifier and no policy",
			typ:  "PROCESS-PATH-REGEX",
			rest: `^/usr/bin/.{1,8}/curl$`,
			want: `^/usr/bin/.{1,8}/curl$`,
		},
		{
			name: "PROCESS-PATH-REGEX with quantifier and policy",
			typ:  "PROCESS-PATH-REGEX",
			rest: `^/usr/bin/.{1,8}/curl$,DIRECT`,
			want: `^/usr/bin/.{1,8}/curl$`,
		},
		{
			name: "PROCESS-NAME-REGEX with quantifier",
			typ:  "PROCESS-NAME-REGEX",
			rest: `^chrome.{1,4}$`,
			want: `^chrome.{1,4}$`,
		},
		{
			name: "PROCESS-NAME-REGEX with quantifier and policy",
			typ:  "PROCESS-NAME-REGEX",
			rest: `^chrome.{1,4}$,PROXY`,
			want: `^chrome.{1,4}$`,
		},
		// --- regex types: simple values without commas still work ---
		{
			name: "DOMAIN-REGEX simple value no policy",
			typ:  "DOMAIN-REGEX",
			rest: `^example\.com$`,
			want: `^example\.com$`,
		},
		{
			name: "DOMAIN-REGEX simple value with policy",
			typ:  "DOMAIN-REGEX",
			rest: `^example\.com$,PROXY`,
			want: `^example\.com$`,
		},
		// --- non-regex types still use first-comma semantics ---
		{
			name: "IP-CIDR with policy and modifier",
			typ:  "IP-CIDR",
			rest: `1.1.1.1/32,DIRECT,no-resolve`,
			want: `1.1.1.1/32`,
		},
		{
			name: "DOMAIN-SUFFIX with policy",
			typ:  "DOMAIN-SUFFIX",
			rest: `google.com,PROXY`,
			want: `google.com`,
		},
		{
			name: "DOMAIN no policy",
			typ:  "DOMAIN",
			rest: `example.com`,
			want: `example.com`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractClassicalValue(c.typ, c.rest)
			if got != c.want {
				t.Errorf("extractClassicalValue(%q, %q) = %q, want %q", c.typ, c.rest, got, c.want)
			}
		})
	}
}

// TestRunMihomoClassicalToSingboxSource_RegexValuePreservesCommas is
// the end-to-end counterpart to TestExtractClassicalValue_RegexComma:
// it feeds regex rules with embedded commas through the full
// transformer and asserts the resulting JSON carries the original
// regex verbatim (modulo the JSON-level `\` → `\\` escape). Both
// with-policy and without-policy variants are covered so a future
// refactor can't quietly regress only one side.
func TestRunMihomoClassicalToSingboxSource_RegexValuePreservesCommas(t *testing.T) {
	input := strings.Join([]string{
		`DOMAIN-REGEX,^a{1,3}\.com$`,                // no policy
		`DOMAIN-REGEX,^b{2,5}\.com$,PROXY`,          // policy
		`DOMAIN-REGEX,^c\.com$,PROXY,no-resolve`,    // policy + modifier
		`DOMAIN-REGEX,^(foo|bar){2,5}\.com$,DIRECT`, // alternation + quantifier
	}, "\n")
	res, _ := RunBuiltin(BuiltinMihomoClassicalToSingboxSource, nil, input)

	// Parse the output so we don't have to fight JSON-escape rules in
	// the assertion strings.
	var doc struct {
		Rules []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(res.Output), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, res.Output)
	}
	var got []string
	for _, r := range doc.Rules {
		if v, ok := r["domain_regex"]; ok {
			for _, item := range v.([]interface{}) {
				got = append(got, item.(string))
			}
		}
	}
	want := []string{
		`^a{1,3}\.com$`,
		`^b{2,5}\.com$`,
		`^c\.com$`,
		`^(foo|bar){2,5}\.com$`,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d domain_regex values, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("domain_regex[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSingboxFieldMinVersion_KnownEntries documents the version floors
// the runner enforces. If a future sing-box release backports a field
// to an earlier rule-set version, this test fails loudly so we
// remember to update the table (and the matching frontend map).
func TestSingboxFieldMinVersion_KnownEntries(t *testing.T) {
	cases := map[string]int{
		"domain":             1,
		"domain_suffix":      1,
		"ip_cidr":            1,
		"port":               1,
		"network":            1,
		"process_path_regex": 2,
		"network_type":       3,
		"wifi_ssid":          3,
		"wifi_bssid":         3,
		"package_name_regex": 5,
	}
	for field, want := range cases {
		if got := SingboxFieldMinVersion(field); got != want {
			t.Errorf("SingboxFieldMinVersion(%q) = %d, want %d", field, got, want)
		}
	}
}
