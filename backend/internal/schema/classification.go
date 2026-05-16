package schema

import (
	"sort"
	"strings"
)

// NormalizeGeositeAttrs mirrors normalizeGeositeAttrs in rule-classification.ts.
func NormalizeGeositeAttrs(attrs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(attrs))
	for _, a := range attrs {
		v := strings.TrimSpace(strings.ToLower(a))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// PrimaryGeositeSource returns the first source if it is a geosite source.
func PrimaryGeositeSource(rule *RuleConfig) *SourceConfig {
	if rule == nil || len(rule.Sources) == 0 {
		return nil
	}
	src := rule.Sources[0]
	if src.SourceType() == "geosite" {
		return &src
	}
	return nil
}

// IsGeositeRule reports whether the rule's primary source is geosite.
func IsGeositeRule(rule *RuleConfig) bool {
	return PrimaryGeositeSource(rule) != nil
}

// GeositeInternalRuleName mirrors getGeositeInternalRuleName.
func GeositeInternalRuleName(provider, list string, attrs []string) string {
	a := NormalizeGeositeAttrs(attrs)
	if len(a) == 0 {
		return "geosite_" + provider + "_" + list
	}
	return "geosite_" + provider + "_" + list + "@" + strings.Join(a, "+")
}

// GeositeOutputName mirrors getGeositeOutputName.
func GeositeOutputName(source *SourceConfig) string {
	if source == nil {
		return ""
	}
	list := strings.TrimSpace(source.List)
	attrs := NormalizeGeositeAttrs(source.Attrs)
	if len(attrs) == 0 {
		return list
	}
	return list + "@" + strings.Join(attrs, "+")
}
