package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
)

// raw is a small helper that lets test cases stay readable: write the
// payload as a literal struct and we render it to json.RawMessage so the
// validator sees the same shape an API request would.
func raw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// TestValidateTransform_BuiltinRequiresRegisteredName guards the regression
// where an unknown `builtin:` name would silently pass save and then
// no-op at publish time. That used to mean a typo like
// "builtin:mihomo-to-shadowrockt" published unconverted mihomo classical
// rules to Shadowrocket — invalid output, no observable error.
func TestValidateTransform_BuiltinRequiresRegisteredName(t *testing.T) {
	err := validateTransform(schema.Transform{Type: "use", Use: "builtin:does-not-exist"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown builtin name, got nil")
	}
	if !strings.Contains(err.Error(), "unknown built-in transformer") {
		t.Fatalf("error mismatch: %v", err)
	}
}

// TestValidateTransform_UseRequiresUserOrBuiltin asserts that a non-prefixed
// `use` value must resolve in the user-supplied transformers map.
func TestValidateTransform_UseRequiresUserOrBuiltin(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		err := validateTransform(schema.Transform{Type: "use", Use: "redundant_cleaner"}, nil)
		if err == nil {
			t.Fatal("expected error for undefined transformer")
		}
	})
	t.Run("present", func(t *testing.T) {
		transformers := map[string]schema.ScriptTransformer{
			"redundant_cleaner": {Name: "redundant_cleaner", Script: "// noop"},
		}
		if err := validateTransform(schema.Transform{Type: "use", Use: "redundant_cleaner"}, transformers); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
}

// TestValidateShadowrocketParams_RejectsBadShapes covers every wire-level
// failure the validator must catch before a config is persisted.
func TestValidateShadowrocketParams_RejectsBadShapes(t *testing.T) {
	type row struct {
		name    string
		params  any
		wantSub string // substring of expected error
	}
	cases := []row{
		{
			name:    "bad json",
			params:  json.RawMessage(`{"rules":[`),
			wantSub: "invalid JSON",
		},
		{
			name:    "unknown action enum",
			params:  map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "drpo"}}},
			wantSub: "action must be",
		},
		{
			name:    "rename missing renameTo",
			params:  map[string]any{"rules": []any{map[string]string{"type": "MATCH", "action": "rename"}}},
			wantSub: "rename action requires `renameTo`",
		},
		{
			name:    "type required",
			params:  map[string]any{"rules": []any{map[string]string{"action": "keep"}}},
			wantSub: "type is required",
		},
		{
			name:    "duplicate type",
			params:  map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "keep"}, map[string]string{"type": "DOMAIN", "action": "drop"}}},
			wantSub: "appears more than once",
		},
		{
			name:    "unknownAction rename rejected",
			params:  map[string]any{"unknownAction": "rename"},
			wantSub: "unknownAction must be",
		},
		{
			name:    "unknownAction unknown value",
			params:  map[string]any{"unknownAction": "drop_all"},
			wantSub: "unknownAction must be",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rm json.RawMessage
			if m, ok := c.params.(json.RawMessage); ok {
				rm = m
			} else {
				rm = raw(c.params)
			}
			err := validateShadowrocketParams(rm)
			if err == nil {
				t.Fatalf("expected error, got nil (params=%s)", string(rm))
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.wantSub)
			}
		})
	}
}

// TestValidateShadowrocketParams_AcceptsValidShapes ensures the validator
// doesn't over-reject the happy paths the editor produces.
func TestValidateShadowrocketParams_AcceptsValidShapes(t *testing.T) {
	cases := [][]byte{
		nil, // empty → defaults
		[]byte(`{}`),
		[]byte(`{"rules":[{"type":"DOMAIN","action":"keep"}]}`),
		[]byte(`{"rules":[{"type":"MATCH","action":"rename","renameTo":"FINAL"}]}`),
		[]byte(`{"rules":[{"type":"PROCESS-NAME","action":"drop","reason":"unsupported"}]}`),
		[]byte(`{"rules":[],"unknownAction":"drop"}`),
	}
	for _, c := range cases {
		if err := validateShadowrocketParams(c); err != nil {
			t.Errorf("validateShadowrocketParams(%q) unexpected error: %v", string(c), err)
		}
	}
}

// TestValidateTransform_UseEmptyString covers the edge case where a
// transform of type "use" has an empty Use field — this is a wire-level
// bug that should be caught at save time rather than silently no-oping
// in the pipeline.
func TestValidateTransform_UseEmptyString(t *testing.T) {
	err := validateTransform(schema.Transform{Type: "use", Use: ""}, nil)
	if err == nil {
		t.Fatal("expected error for use transform with empty `use` field")
	}
	if !strings.Contains(err.Error(), "requires `use`") {
		t.Fatalf("error mismatch: %v", err)
	}
}

// TestValidateSingboxSourceParams_RejectsBadShapes covers every wire-level
// failure the validator must catch before a config is persisted for the
// builtin:mihomo-classical-to-singbox-source transformer.
func TestValidateSingboxSourceParams_RejectsBadShapes(t *testing.T) {
	type row struct {
		name    string
		params  any
		wantSub string
	}
	cases := []row{
		{
			name:    "bad json",
			params:  json.RawMessage(`{"rules":[`),
			wantSub: "invalid JSON",
		},
		{
			name:    "type required",
			params:  map[string]any{"rules": []any{map[string]string{"action": "map", "mapTo": "domain"}}},
			wantSub: "type is required",
		},
		{
			name:    "unknown action enum",
			params:  map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "rename"}}},
			wantSub: "action must be",
		},
		{
			name:    "map missing mapTo",
			params:  map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "map"}}},
			wantSub: "map action requires `mapTo`",
		},
		{
			name:    "map mapTo unknown sing-box field",
			params:  map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "map", "mapTo": "fake_field"}}},
			wantSub: "not a known sing-box headless rule field",
		},
		{
			name: "duplicate type",
			params: map[string]any{
				"rules": []any{
					map[string]string{"type": "DOMAIN", "action": "map", "mapTo": "domain"},
					map[string]string{"type": "DOMAIN", "action": "drop"},
				},
			},
			wantSub: "appears more than once",
		},
		{
			name:    "version below floor",
			params:  map[string]any{"version": 0, "rules": []any{}}, // 0 is fine (default sentinel), so we use -1
			wantSub: "",                                             // sentinel: we replace below
		},
		{
			name:    "version above ceiling",
			params:  map[string]any{"version": 99, "rules": []any{}},
			wantSub: "version must be between",
		},
	}
	for _, c := range cases {
		if c.name == "version below floor" {
			// 0 is a valid sentinel ("use default"). The real "below
			// floor" test uses a negative value, which is rejected.
			c.params = map[string]any{"version": -1, "rules": []any{}}
			c.wantSub = "version must be between"
		}
		t.Run(c.name, func(t *testing.T) {
			var rm json.RawMessage
			if m, ok := c.params.(json.RawMessage); ok {
				rm = m
			} else {
				rm = raw(c.params)
			}
			err := validateSingboxSourceParams(rm)
			if err == nil {
				t.Fatalf("expected error, got nil (params=%s)", string(rm))
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.wantSub)
			}
		})
	}
}

// TestValidateSingboxSourceParams_AcceptsValidShapes ensures the
// validator doesn't over-reject the happy paths the editor produces.
func TestValidateSingboxSourceParams_AcceptsValidShapes(t *testing.T) {
	cases := [][]byte{
		nil, // empty → defaults
		[]byte(`{}`),
		[]byte(`{"version": 0}`),              // sentinel: runner picks default
		[]byte(`{"version": 3}`),              // explicit current default
		[]byte(`{"version": 1, "rules": []}`), // floor
		[]byte(`{"version": 5, "rules": []}`), // ceiling
		[]byte(`{"rules":[{"type":"DOMAIN","action":"map","mapTo":"domain"}]}`),         // simple map
		[]byte(`{"rules":[{"type":"GEOSITE","action":"drop","reason":"unsupported"}]}`), // drop with reason
		// version ceilings permit their own ceiling-only fields:
		[]byte(`{"version": 5, "rules":[{"type":"P","action":"map","mapTo":"package_name_regex"}]}`),
		[]byte(`{"version": 3, "rules":[{"type":"W","action":"map","mapTo":"wifi_ssid"}]}`),
		// regression for Bug 1: an explicit empty rules array is a
		// valid "drop everything" intent, not a config error.
		[]byte(`{"rules": []}`),
	}
	for _, c := range cases {
		if err := validateSingboxSourceParams(c); err != nil {
			t.Errorf("validateSingboxSourceParams(%q) unexpected error: %v", string(c), err)
		}
	}
}

// TestValidateSingboxSourceParams_RejectsCrossVersionField guards the
// save-time check for the second reviewed bug: a row that targets a
// sing-box field newer than the declared rule-set version produces
// JSON the targeted sing-box release will reject. The validator must
// catch the mismatch before the config is persisted, and the error
// message must point the operator at the fix.
func TestValidateSingboxSourceParams_RejectsCrossVersionField(t *testing.T) {
	type row struct {
		name      string
		params    string
		wantField string
	}
	cases := []row{
		{
			name:      "v1 rejects process_path_regex (v2-only)",
			params:    `{"version": 1, "rules": [{"type": "PROCESS-PATH-REGEX", "action": "map", "mapTo": "process_path_regex"}]}`,
			wantField: "process_path_regex",
		},
		{
			name:      "v2 rejects network_type (v3-only)",
			params:    `{"version": 2, "rules": [{"type": "NETWORK-TYPE", "action": "map", "mapTo": "network_type"}]}`,
			wantField: "network_type",
		},
		{
			name:      "v4 rejects package_name_regex (v5-only)",
			params:    `{"version": 4, "rules": [{"type": "PKG", "action": "map", "mapTo": "package_name_regex"}]}`,
			wantField: "package_name_regex",
		},
		{
			// Default version is DefaultSingboxSourceVersion (3), so
			// package_name_regex (requires v5) is rejected even when
			// the user didn't pick a version explicitly.
			name:      "default version (3) rejects package_name_regex (v5-only)",
			params:    `{"rules": [{"type": "PKG", "action": "map", "mapTo": "package_name_regex"}]}`,
			wantField: "package_name_regex",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSingboxSourceParams(json.RawMessage(c.params))
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantField) {
				t.Fatalf("error %q does not mention field %q", err.Error(), c.wantField)
			}
			if !strings.Contains(err.Error(), "version") {
				t.Fatalf("error %q does not mention version", err.Error())
			}
		})
	}
}

// TestValidateRulesConfigPayload_BuiltinPrefixCollision ensures that
// user-defined transformers whose key starts with the reserved "builtin:"
// prefix are rejected by the config-level validator so the dispatcher can
// never accidentally resolve a user script instead of a native runner.
func TestValidateRulesConfigPayload_BuiltinPrefixCollision(t *testing.T) {
	err := validateRulesConfigPayload(&schema.RulesConfig{
		Version: 1,
		Transformers: map[string]schema.ScriptTransformer{
			"builtin:mihomo-classical-to-yaml": {Name: "builtin:mihomo-classical-to-yaml", Script: "// malicious override"},
		},
		Rules: []schema.RuleConfig{},
	})
	if err == nil {
		t.Fatal("expected error for user-defined transformer with builtin: prefix")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("error mismatch: %v", err)
	}
}

// TestValidateRulesConfigPayload_BuiltinParams covers the new path where
// built-in transformer params live on the config (one configuration per
// built-in, shared by every transform that references it):
//
//   - keys must be real registered built-in names (no typos, no
//     user-defined names).
//   - the per-builtin validator still runs against the value.
//   - well-formed entries pass.
func TestValidateRulesConfigPayload_BuiltinParams(t *testing.T) {
	base := func(builtin map[string]json.RawMessage) *schema.RulesConfig {
		return &schema.RulesConfig{
			Version:       1,
			Transformers:  map[string]schema.ScriptTransformer{},
			BuiltinParams: builtin,
			Rules:         []schema.RuleConfig{},
		}
	}
	t.Run("unknown key rejected", func(t *testing.T) {
		err := validateRulesConfigPayload(base(map[string]json.RawMessage{
			"builtin:does-not-exist": json.RawMessage(`{}`),
		}))
		if err == nil || !strings.Contains(err.Error(), "not a known built-in") {
			t.Fatalf("expected unknown built-in error, got %v", err)
		}
	})
	t.Run("user namespace rejected", func(t *testing.T) {
		err := validateRulesConfigPayload(base(map[string]json.RawMessage{
			"my-custom-transformer": json.RawMessage(`{}`),
		}))
		if err == nil || !strings.Contains(err.Error(), "not a known built-in") {
			t.Fatalf("expected non-builtin-prefix rejection, got %v", err)
		}
	})
	t.Run("invalid params propagate", func(t *testing.T) {
		err := validateRulesConfigPayload(base(map[string]json.RawMessage{
			transformer.BuiltinMihomoToShadowrocket: raw(map[string]any{
				"rules": []any{map[string]string{"type": "MATCH", "action": "rename"}},
			}),
		}))
		if err == nil || !strings.Contains(err.Error(), "rename action requires") {
			t.Fatalf("expected nested params validation error, got %v", err)
		}
	})
	t.Run("happy path", func(t *testing.T) {
		err := validateRulesConfigPayload(base(map[string]json.RawMessage{
			transformer.BuiltinMihomoToShadowrocket: raw(map[string]any{
				"rules": []any{
					map[string]string{"type": "DOMAIN", "action": "keep"},
					map[string]string{"type": "MATCH", "action": "rename", "renameTo": "FINAL"},
				},
				"unknownAction": "drop",
			}),
		}))
		if err != nil {
			t.Fatalf("expected nil for valid params, got %v", err)
		}
	})
}
