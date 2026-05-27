package api

import (
	"encoding/json"
	"fmt"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
)

func validateRulesConfigPayload(cfg *schema.RulesConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if cfg.Rules == nil {
		return fmt.Errorf("rules is required")
	}
	// Reserved namespace: built-in transformers are shipped by the binary
	// and must not be shadowed by user-defined entries with the same key.
	for name := range cfg.Transformers {
		if transformer.HasBuiltinPrefix(name) {
			return fmt.Errorf("transformer name %q uses the reserved \"builtin:\" prefix", name)
		}
	}
	// Built-in transformer params live on the config so each built-in is
	// configured exactly once for the whole deployment; rule/client
	// transforms only reference the built-in by name. Validate any value
	// the user persisted, and reject keys that don't correspond to a real
	// built-in so a config-restore typo can't silently change publishing.
	for name, raw := range cfg.BuiltinParams {
		if !transformer.HasBuiltinPrefix(name) || !transformer.IsBuiltinName(name) {
			return fmt.Errorf("builtinParams[%q] is not a known built-in transformer", name)
		}
		if err := validateBuiltinParams(name, raw); err != nil {
			return fmt.Errorf("builtinParams[%q]: %w", name, err)
		}
	}
	for i := range cfg.Rules {
		if err := validateRulePayload(&cfg.Rules[i], cfg.Transformers); err != nil {
			return fmt.Errorf("rule %q: %w", cfg.Rules[i].Name, err)
		}
	}
	return nil
}

func validateRulePayload(rule *schema.RuleConfig, transformers map[string]schema.ScriptTransformer) error {
	if rule == nil {
		return fmt.Errorf("rule is required")
	}
	if rule.Output.Clients == nil {
		return fmt.Errorf("output.clients is required")
	}
	for i, t := range rule.Transforms {
		if err := validateTransform(t, transformers); err != nil {
			return fmt.Errorf("transforms[%d]: %w", i, err)
		}
	}
	for client, override := range rule.Output.ClientOverrides {
		for i, t := range override.Transforms {
			if err := validateTransform(t, transformers); err != nil {
				return fmt.Errorf("output.client_overrides[%q].transforms[%d]: %w", client, i, err)
			}
		}
	}
	return nil
}

// validateTransform applies the shared transform validation that every
// save entry point (rules config, per-rule client override, client global
// transforms) needs:
//
//   - `use` with a builtin: prefix must resolve to a registered built-in
//     so a typo doesn't silently publish unconverted content (esp. on
//     Shadowrocket targets where mihomo classical is invalid).
//   - `use` with a non-builtin name must resolve to a user-defined entry,
//     otherwise the transform is a silent no-op.
//
// Built-in transformer parameters live on RulesConfig.BuiltinParams, not on
// the transform itself, so a `use` reference is just a name lookup.
func validateTransform(t schema.Transform, transformers map[string]schema.ScriptTransformer) error {
	if t.Type != "use" {
		return nil
	}
	if t.Use == "" {
		return fmt.Errorf("transform of type \"use\" requires `use`")
	}
	if transformer.HasBuiltinPrefix(t.Use) {
		if !transformer.IsBuiltinName(t.Use) {
			return fmt.Errorf("unknown built-in transformer %q", t.Use)
		}
		return nil
	}
	if _, ok := transformers[t.Use]; !ok {
		return fmt.Errorf("transform references undefined transformer %q", t.Use)
	}
	return nil
}

// validateBuiltinParams dispatches to the per-builtin validator. Builtins
// that don't accept any params return nil regardless of the blob — the
// runner ignores unknown fields.
func validateBuiltinParams(name string, raw json.RawMessage) error {
	switch name {
	case transformer.BuiltinMihomoToShadowrocket:
		return validateShadowrocketParams(raw)
	}
	return nil
}

// shadowrocketParamsPayload mirrors the on-disk shape; we keep it local to
// the validator so we can reject unknown fields and validate enums
// without exposing internal types.
type shadowrocketParamsPayload struct {
	Rules         []shadowrocketRulePayload `json:"rules"`
	UnknownAction string                    `json:"unknownAction"`
}

type shadowrocketRulePayload struct {
	Type     string `json:"type"`
	Action   string `json:"action"`
	RenameTo string `json:"renameTo"`
	Reason   string `json:"reason"`
}

func validateShadowrocketParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		// Empty params is fine: the runner falls back to the default
		// curated mapping table.
		return nil
	}
	var p shadowrocketParamsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if p.UnknownAction != "" {
		if !isShadowrocketUnknownAction(p.UnknownAction) {
			return fmt.Errorf("unknownAction must be \"keep\" or \"drop\", got %q", p.UnknownAction)
		}
	}
	seen := make(map[string]struct{}, len(p.Rules))
	for i, r := range p.Rules {
		if r.Type == "" {
			return fmt.Errorf("rules[%d].type is required", i)
		}
		if _, dup := seen[r.Type]; dup {
			return fmt.Errorf("rules[%d].type %q appears more than once", i, r.Type)
		}
		seen[r.Type] = struct{}{}
		switch r.Action {
		case transformer.ShadowrocketActionKeep, transformer.ShadowrocketActionDrop:
			// renameTo is ignored for these actions; no further check.
		case transformer.ShadowrocketActionRename:
			if r.RenameTo == "" {
				return fmt.Errorf("rules[%d] (%q): rename action requires `renameTo`", i, r.Type)
			}
		default:
			return fmt.Errorf("rules[%d] (%q): action must be \"keep\" | \"rename\" | \"drop\", got %q", i, r.Type, r.Action)
		}
	}
	return nil
}

// isShadowrocketUnknownAction restricts the fallback to actions that make
// sense as a default; "rename" doesn't because there's no target type to
// rename to without a per-row spec.
func isShadowrocketUnknownAction(action string) bool {
	return action == transformer.ShadowrocketActionKeep || action == transformer.ShadowrocketActionDrop
}

// validateClientTransforms is the dedicated entry point for the client
// CRUD routes. It only sees a `Transforms` slice (no transformer map), so
// any `use` against a JS transformer is checked against the persisted
// config at the time of save.
func validateClientTransforms(transforms []schema.Transform, transformers map[string]schema.ScriptTransformer) error {
	for i, t := range transforms {
		if err := validateTransform(t, transformers); err != nil {
			return fmt.Errorf("transforms[%d]: %w", i, err)
		}
	}
	return nil
}

func validateClientFileExt(ext string) (string, error) {
	normalized := ext
	if len(normalized) > 0 && normalized[0] == '.' {
		normalized = normalized[1:]
	}
	if normalized == "" {
		return "", fmt.Errorf("ext is required")
	}
	if containsPathSeparators(normalized) || containsDotDot(normalized) {
		return "", fmt.Errorf("file extension contains invalid characters")
	}
	return normalized, nil
}

func containsPathSeparators(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] == '/' || value[i] == '\\' {
			return true
		}
	}
	return false
}

func containsDotDot(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] == '.' && value[i+1] == '.' {
			return true
		}
	}
	return false
}
