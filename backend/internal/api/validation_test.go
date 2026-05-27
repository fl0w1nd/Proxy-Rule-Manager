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

// TestValidateTransform_BuiltinParamsRoundtrip exercises the full
// validateTransform path with shadowrocket params, including the error
// path where unknownAction is invalid.
func TestValidateTransform_BuiltinParamsRoundtrip(t *testing.T) {
	good := schema.Transform{
		Type:   "use",
		Use:    transformer.BuiltinMihomoToShadowrocket,
		Params: raw(map[string]any{"rules": []any{map[string]string{"type": "DOMAIN", "action": "keep"}}}),
	}
	if err := validateTransform(good, nil); err != nil {
		t.Fatalf("good transform should pass: %v", err)
	}
	bad := schema.Transform{
		Type:   "use",
		Use:    transformer.BuiltinMihomoToShadowrocket,
		Params: raw(map[string]any{"rules": []any{map[string]string{"type": "MATCH", "action": "rename"}}}),
	}
	if err := validateTransform(bad, nil); err == nil {
		t.Fatal("bad transform (rename missing renameTo) should fail")
	}
}
