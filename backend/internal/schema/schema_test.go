package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuleConfig_JSONRoundTrip(t *testing.T) {
	payload := `{
		"name": "test",
		"displayName": "Test",
		"sources": [
			{"type":"geosite","provider":"v2fly","list":"google","attrs":["ads"]},
			{"type":"url","url":"https://example.com/list"},
			{"type":"local","content":"DOMAIN,a.com"}
		],
		"merge": {"strategy":"union","dedupe":true},
		"transforms": [
			{"type":"replace","target":"all","pattern":"old","replacement":"new"}
		],
		"output": {"clients":["clash_meta"]}
	}`
	var rule RuleConfig
	if err := json.Unmarshal([]byte(payload), &rule); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rule.Name != "test" || rule.DisplayName != "Test" {
		t.Fatalf("decoded fields: %+v", rule)
	}
	if len(rule.Sources) != 3 || rule.Sources[2].Content == nil || *rule.Sources[2].Content != "DOMAIN,a.com" {
		t.Fatalf("sources: %+v", rule.Sources)
	}
	if !IsGeositeRule(&rule) {
		t.Fatalf("expected IsGeositeRule to return true")
	}
	if src := PrimaryGeositeSource(&rule); src == nil || src.Provider != "v2fly" || src.List != "google" {
		t.Fatalf("primary geosite source: %+v", src)
	}
	if attrs := NormalizeGeositeAttrs([]string{" ADS ", "ads", "@cn"}); len(attrs) != 2 || attrs[0] != "@cn" || attrs[1] != "ads" {
		t.Fatalf("NormalizeGeositeAttrs: %v", attrs)
	}
	internal := GeositeInternalRuleName("v2fly", "google", []string{"ads"})
	if !strings.Contains(internal, "geosite") || !strings.Contains(internal, "google") {
		t.Fatalf("internal name: %s", internal)
	}
	if name := GeositeOutputName(PrimaryGeositeSource(&rule)); name == "" {
		t.Fatalf("output name empty")
	}

	out, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(out), `"clients":["clash_meta"]`) {
		t.Fatalf("re-encoded missing clients: %s", out)
	}
}

func TestMergeConfig_Defaults(t *testing.T) {
	var nilMerge *MergeConfig
	if got := nilMerge.EffectiveStrategy(); got != "concat" {
		t.Fatalf("nil merge strategy: %s", got)
	}
	mc := &MergeConfig{Strategy: "union"}
	if got := mc.EffectiveStrategy(); got != "union" {
		t.Fatalf("explicit union: %s", got)
	}
}

func TestDefaultConfigHasDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnsureDefaults()
	if cfg.Version == 0 {
		t.Errorf("expected non-zero version")
	}
	if cfg.Transformers == nil {
		t.Errorf("transformers map should be initialised")
	}
}

// TestRuleConfig_EnsureDefaultsTags verifies that an empty RuleConfig after
// EnsureDefaults serialises with "tags":[] and not a missing or null tags key.
func TestRuleConfig_EnsureDefaultsTags(t *testing.T) {
	var r RuleConfig
	r.Name = "test"
	r.Output = OutputConfig{Clients: []string{"clash_meta"}}
	r.EnsureDefaults()

	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"tags":[]`) {
		t.Errorf("expected \"tags\":[] in output, got: %s", s)
	}
}

// TestMergeConfig_EnsureDefaultsStrategy verifies that an empty MergeConfig after
// EnsureDefaults serialises with "strategy":"concat" and "dedupe":false.
func TestMergeConfig_EnsureDefaultsStrategy(t *testing.T) {
	var m MergeConfig
	m.EnsureDefaults()

	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"strategy":"concat"`) {
		t.Errorf("expected \"strategy\":\"concat\" in output, got: %s", s)
	}
	if !strings.Contains(s, `"dedupe":false`) {
		t.Errorf("expected \"dedupe\":false in output, got: %s", s)
	}
}

// TestClientOutputOverride_EnsureDefaults verifies that a zero-value
// ClientOutputOverride after EnsureDefaults serialises with enabled=true and
// useGlobalTransforms=true.
func TestClientOutputOverride_EnsureDefaults(t *testing.T) {
	var c ClientOutputOverride
	c.EnsureDefaults()

	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"enabled":true`) {
		t.Errorf("expected \"enabled\":true in output, got: %s", s)
	}
	if !strings.Contains(s, `"useGlobalTransforms":true`) {
		t.Errorf("expected \"useGlobalTransforms\":true in output, got: %s", s)
	}
}

// TestClientOutputOverride_UnmarshalJSON_Defaults verifies that decoding {}
// produces enabled=true and useGlobalTransforms=true (TS schema defaults).
func TestClientOutputOverride_UnmarshalJSON_Defaults(t *testing.T) {
	var c ClientOutputOverride
	if err := json.Unmarshal([]byte(`{}`), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !c.Enabled {
		t.Errorf("expected Enabled=true when key absent, got false")
	}
	if !c.UseGlobalTransforms {
		t.Errorf("expected UseGlobalTransforms=true when key absent, got false")
	}
}

// TestClientOutputOverride_UnmarshalJSON_ExplicitFalse verifies that
// explicit false values are honoured and not overridden by the default.
func TestClientOutputOverride_UnmarshalJSON_ExplicitFalse(t *testing.T) {
	var c ClientOutputOverride
	if err := json.Unmarshal([]byte(`{"enabled":false,"useGlobalTransforms":false}`), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Enabled {
		t.Errorf("expected Enabled=false when explicitly set, got true")
	}
	if c.UseGlobalTransforms {
		t.Errorf("expected UseGlobalTransforms=false when explicitly set, got true")
	}
}

// TestValidateCacheMode checks the CdnSettings.cacheMode allow-list.
func TestValidateCacheMode(t *testing.T) {
	if err := ValidateCacheMode("no-store"); err != nil {
		t.Errorf("expected no-store to be valid, got: %v", err)
	}
	if err := ValidateCacheMode("no-cache"); err != nil {
		t.Errorf("expected no-cache to be valid, got: %v", err)
	}
	if err := ValidateCacheMode("custom"); err != nil {
		t.Errorf("expected custom to be valid, got: %v", err)
	}
	if err := ValidateCacheMode("transparent"); err == nil {
		t.Errorf("expected transparent to be invalid, but got nil error")
	}
	if err := ValidateCacheMode(""); err == nil {
		t.Errorf("expected empty string to be invalid, but got nil error")
	}
}

// TestValidateConfigID checks the configId regex constraint.
func TestValidateConfigID(t *testing.T) {
	valid := []string{"abc", "abc-123", "my_config", "A1-B2_C3"}
	for _, s := range valid {
		if err := ValidateConfigID(s); err != nil {
			t.Errorf("expected %q to be valid, got: %v", s, err)
		}
	}
	invalid := []string{"", "has space", "has.dot", "has/slash", "has@at"}
	for _, s := range invalid {
		if err := ValidateConfigID(s); err == nil {
			t.Errorf("expected %q to be invalid, but got nil error", s)
		}
	}
}

// TestSourceConfig_ContentNullSerialisation verifies that a nil Content field
// serialises as JSON null (not omitted), allowing routes to signal deletion.
func TestSourceConfig_ContentNullSerialisation(t *testing.T) {
	src := SourceConfig{Type: "local", Content: nil}
	out, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"content":null`) {
		t.Errorf("expected \"content\":null in output, got: %s", s)
	}
}
