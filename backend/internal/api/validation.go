package api

import (
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
	for i := range cfg.Rules {
		if err := validateRulePayload(&cfg.Rules[i]); err != nil {
			return fmt.Errorf("rule %q: %w", cfg.Rules[i].Name, err)
		}
	}
	return nil
}

func validateRulePayload(rule *schema.RuleConfig) error {
	if rule == nil {
		return fmt.Errorf("rule is required")
	}
	if rule.Output.Clients == nil {
		return fmt.Errorf("output.clients is required")
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
