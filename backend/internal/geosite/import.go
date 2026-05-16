package geosite

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// ImportSelection matches GeositeImportSelection in TS.
type ImportSelection struct {
	List  string   `json:"list"`
	Attrs []string `json:"attrs,omitempty"`
}

// ImportResult mirrors ImportAllGeositeResult.
type ImportResult struct {
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Skipped   int      `json:"skipped"`
	Total     int      `json:"total"`
	RuleNames []string `json:"ruleNames"`
}

// PreviewResult is returned by the geosite preview endpoint.
type PreviewResult struct {
	Content      string `json:"content"`
	TotalEntries int    `json:"totalEntries"`
	TotalLines   int    `json:"totalLines"`
	Truncated    bool   `json:"truncated"`
}

func normalizeAttrSelection(attrs []string) []string {
	return NormalizeAttrs(attrs)
}

func normalizeSelections(items []ImportSelection) []ImportSelection {
	dedup := map[string]ImportSelection{}
	for _, sel := range items {
		list := normalizeName(sel.List)
		if list == "" {
			continue
		}
		attrs := normalizeAttrSelection(sel.Attrs)
		key := list + "::" + strings.Join(attrs, "+")
		dedup[key] = ImportSelection{List: list, Attrs: attrs}
	}
	out := make([]ImportSelection, 0, len(dedup))
	for _, v := range dedup {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i].List + "::" + strings.Join(out[i].Attrs, "+")
		b := out[j].List + "::" + strings.Join(out[j].Attrs, "+")
		return a < b
	})
	return out
}

func importedRuleKey(provider, list string, attrs []string) string {
	return provider + "::" + normalizeName(list) + "::" + strings.Join(normalizeAttrSelection(attrs), "+")
}

func buildImportedIndex(cfg *schema.RulesConfig, provider string) map[string]*schema.RuleConfig {
	index := map[string]*schema.RuleConfig{}
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		src := schema.PrimaryGeositeSource(rule)
		if src == nil || src.Provider != provider || src.List == "" {
			continue
		}
		index[importedRuleKey(provider, src.List, src.Attrs)] = rule
	}
	return index
}

func legacyGeositeInternalName(provider, list string, attrs []string) string {
	a := normalizeAttrSelection(attrs)
	if len(a) == 0 {
		return "geosite_" + provider + "_" + list
	}
	return "geosite_" + provider + "_" + list + "__" + strings.Join(a, "_")
}

// UpsertImportedGeositeRules mutates cfg in-place.
func UpsertImportedGeositeRules(cfg *schema.RulesConfig, provider, clientID string, items []ImportSelection) ImportResult {
	selections := normalizeSelections(items)
	index := buildImportedIndex(cfg, provider)
	var result ImportResult
	for _, sel := range selections {
		name := schema.GeositeInternalRuleName(provider, sel.List, sel.Attrs)
		key := importedRuleKey(provider, sel.List, sel.Attrs)
		result.RuleNames = append(result.RuleNames, name)
		existing, ok := index[key]
		if !ok {
			cfg.Rules = append(cfg.Rules, buildDefaultGeositeRule(provider, sel.List, clientID, sel.Attrs))
			index[key] = &cfg.Rules[len(cfg.Rules)-1]
			result.Created++
			continue
		}
		if !schema.IsGeositeRule(existing) {
			result.Skipped++
			continue
		}
		src := schema.PrimaryGeositeSource(existing)
		if src == nil || importedRuleKey(src.Provider, src.List, src.Attrs) != key {
			result.Skipped++
			continue
		}
		syncManagedPresentation(existing, provider, sel.List, sel.Attrs)
		legacyName := legacyGeositeInternalName(provider, sel.List, sel.Attrs)
		nextName := schema.GeositeInternalRuleName(provider, sel.List, sel.Attrs)
		if existing.Name == legacyName || existing.Name == nextName {
			existing.Name = nextName
		}
		alreadyHas := false
		for _, c := range existing.Output.Clients {
			if c == clientID {
				alreadyHas = true
				break
			}
		}
		if !alreadyHas {
			existing.Output.Clients = append(existing.Output.Clients, clientID)
			result.Updated++
		} else {
			result.Skipped++
		}
	}
	result.Total = len(selections)
	return result
}

func buildDefaultGeositeRule(provider, list, clientID string, attrs []string) schema.RuleConfig {
	a := normalizeAttrSelection(attrs)
	name := schema.GeositeInternalRuleName(provider, list, a)
	displayName := list
	if len(a) > 0 {
		displayName = list + "@" + strings.Join(a, "+")
	}
	description := fmt.Sprintf("Geosite %s from %s", displayName, provider)
	return schema.RuleConfig{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Tags:        []string{"geosite", provider},
		Sources: []schema.SourceConfig{{
			Type:          "geosite",
			Provider:      provider,
			List:          list,
			Attrs:         a,
			RenderProfile: "mihomo-classical",
		}},
		Output:     schema.OutputConfig{Clients: []string{clientID}},
		Transforms: []schema.Transform{},
	}
}

func syncManagedPresentation(rule *schema.RuleConfig, provider, list string, attrs []string) {
	a := normalizeAttrSelection(attrs)
	if len(a) == 0 {
		rule.DisplayName = list
		rule.Description = fmt.Sprintf("Geosite %s from %s", list, provider)
		return
	}
	rule.DisplayName = list + "@" + strings.Join(a, "+")
	rule.Description = fmt.Sprintf("Geosite %s@%s from %s", list, strings.Join(a, "+"), provider)
}

// Preview renders a single (provider, list, attrs) tuple, applying client-global
// transforms if any. limitLines<=0 disables truncation; positive values cap the
// content at the first N lines and append a sentinel comment.
func Preview(ctx context.Context, mgr *Manager, provider, list, clientID string, attrs []string, renderProfile string,
	clients []schema.ClientConfig, transformersConfig map[string]schema.ScriptTransformer,
	applyTransforms func(contents []string, transforms []schema.Transform, t map[string]schema.ScriptTransformer) ([]string, error),
	limitLines int,
) (PreviewResult, error) {
	cache, err := mgr.Ensure(ctx, provider)
	if err != nil {
		return PreviewResult{}, err
	}
	entries, err := ResolveEntries(cache, list, attrs)
	if err != nil {
		return PreviewResult{}, err
	}
	content, err := RenderEntries(entries, renderProfile)
	if err != nil {
		return PreviewResult{}, err
	}
	for _, c := range clients {
		if c.ID == clientID && len(c.Transforms) > 0 && applyTransforms != nil {
			out, err := applyTransforms([]string{content}, c.Transforms, transformersConfig)
			if err != nil {
				return PreviewResult{}, err
			}
			if len(out) > 0 {
				content = out[0]
			}
			break
		}
	}
	totalLines := countLines(content)
	truncated := false
	if limitLines > 0 && totalLines > limitLines {
		content = truncateToLines(content, limitLines) + "\n# ... (truncated)"
		truncated = true
	}
	return PreviewResult{
		Content:      content,
		TotalEntries: len(entries),
		TotalLines:   totalLines,
		Truncated:    truncated,
	}, nil
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	if strings.HasSuffix(s, "\n") {
		n--
	}
	return n
}

func truncateToLines(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
			if count == limit {
				return s[:i]
			}
		}
	}
	return s
}
