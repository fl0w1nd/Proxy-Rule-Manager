// Package transformer mirrors src/lib/transformer.ts: pipeline, merge, headers,
// and JS script execution via goja.
package transformer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// Engine encapsulates a shared JS runner used by all transforms.
type Engine struct {
	JS *ScriptRunner
}

// NewEngine creates an engine with default JS runner options.
func NewEngine() *Engine {
	return &Engine{JS: NewScriptRunner(DefaultScriptOptions())}
}

// ApplyNewTransforms runs each transform in order, returning the resulting
// contents (one entry per input source).
func (e *Engine) ApplyNewTransforms(contents []string, transforms []schema.Transform, transformers map[string]schema.ScriptTransformer) ([]string, error) {
	result := append([]string(nil), contents...)
	for _, t := range transforms {
		var err error
		result, err = e.executeNewTransform(result, t, transformers)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (e *Engine) executeNewTransform(contents []string, transform schema.Transform, transformers map[string]schema.ScriptTransformer) ([]string, error) {
	indices, all, err := transform.TargetIndices()
	if err != nil {
		return contents, nil
	}
	targets := make(map[int]struct{})
	if all {
		for i := range contents {
			targets[i] = struct{}{}
		}
	} else {
		for _, idx := range indices {
			targets[idx] = struct{}{}
		}
	}

	out := make([]string, len(contents))
	for i, content := range contents {
		if _, ok := targets[i]; !ok {
			out[i] = content
			continue
		}
		switch transform.Type {
		case "use":
			t, ok := transformers[transform.Use]
			if !ok || strings.TrimSpace(t.Script) == "" {
				out[i] = content
				continue
			}
			res, _ := e.JS.Execute(t.Script, content)
			out[i] = res
		case "replace":
			if transform.Pattern == "" {
				out[i] = content
				continue
			}
			replaced, err := jsReplace(content, transform.Pattern, transform.Replacement, transform.Flags)
			if err != nil {
				// TS silently returns original content on regex error.
				out[i] = content
				continue
			}
			out[i] = replaced
		case "remove_lines":
			if transform.Pattern == "" {
				out[i] = content
				continue
			}
			filtered, err := removeLines(content, transform.Pattern)
			if err != nil {
				// TS silently returns original content on regex error.
				out[i] = content
				continue
			}
			out[i] = filtered
		default:
			out[i] = content
		}
	}
	return out, nil
}

// MergeContents mirrors mergeContents in transformer.ts.
func MergeContents(contents []string, strategy string, dedupe bool) string {
	if len(contents) == 0 {
		return ""
	}
	switch strategy {
	case "concat":
		result := strings.Join(contents, "\n")
		if !dedupe {
			return result
		}
		lines := strings.Split(result, "\n")
		seen := make(map[string]struct{})
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				out = append(out, line)
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, line)
		}
		return strings.Join(out, "\n")
	case "union":
		seen := make(map[string]struct{})
		order := make([]string, 0)
		for _, c := range contents {
			for _, line := range strings.Split(c, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if _, ok := seen[trimmed]; ok {
					continue
				}
				seen[trimmed] = struct{}{}
				order = append(order, trimmed)
			}
		}
		return strings.Join(order, "\n")
	case "intersect":
		if len(contents) == 1 {
			return contents[0]
		}
		var firstOrder []string
		firstSet := make(map[string]struct{})
		for _, line := range strings.Split(contents[0], "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if _, ok := firstSet[trimmed]; ok {
				continue
			}
			firstSet[trimmed] = struct{}{}
			firstOrder = append(firstOrder, trimmed)
		}
		current := firstSet
		for i := 1; i < len(contents); i++ {
			next := make(map[string]struct{})
			for _, line := range strings.Split(contents[i], "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if _, ok := current[trimmed]; ok {
					next[trimmed] = struct{}{}
				}
			}
			current = next
		}
		out := make([]string, 0, len(current))
		for _, line := range firstOrder {
			if _, ok := current[line]; ok {
				out = append(out, line)
			}
		}
		return strings.Join(out, "\n")
	default:
		return strings.Join(contents, "\n")
	}
}

// AddRuleHeader prepends the managed comment header.
func AddRuleHeader(content, _ruleName, _description, updatedAt string) string {
	normalized := normalizeLineEndings(content)
	lines := effectiveRuleLines(normalized)
	counts := ruleTypeCounts(lines)
	timestamp := formatHeaderTimestamp(updatedAt)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 规则数量：%d 条\n", len(lines)))
	sb.WriteString(fmt.Sprintf("# 更新时间：%s\n", timestamp))
	sb.WriteString("# 规则类型：\n")
	for _, c := range counts {
		sb.WriteString(fmt.Sprintf("# %s: %d 条\n", c.Type, c.Count))
	}
	sb.WriteString("\n")
	sb.WriteString(normalized)
	return sb.String()
}

// StripManagedRuleHeader removes our header so that diff/comparison is fair.
func StripManagedRuleHeader(content string) string {
	if content == "" {
		return ""
	}
	normalized := normalizeLineEndings(content)
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 ||
		!strings.HasPrefix(lines[0], "# 规则数量：") ||
		!strings.HasPrefix(lines[1], "# 更新时间：") ||
		lines[2] != "# 规则类型：" {
		return normalized
	}
	index := 3
	for index < len(lines) && strings.HasPrefix(lines[index], "# ") {
		index++
	}
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	return strings.Join(lines[index:], "\n")
}

// NormalizeEffectiveRuleContent returns the effective rule lines joined by \n.
func NormalizeEffectiveRuleContent(content string) string {
	return strings.Join(effectiveRuleLines(content), "\n")
}

func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

func effectiveRuleLines(content string) []string {
	if content == "" {
		return nil
	}
	normalized := normalizeLineEndings(content)
	var out []string
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

type typeCount struct {
	Type  string
	Count int
}

func ruleTypeCounts(lines []string) []typeCount {
	counts := make(map[string]int)
	for _, line := range lines {
		// Split on first comma only.
		typ := line
		if idx := strings.Index(line, ","); idx >= 0 {
			typ = line[:idx]
		}
		typ = strings.TrimSpace(typ)
		if typ == "" {
			typ = "UNKNOWN"
		}
		counts[typ]++
	}
	out := make([]typeCount, 0, len(counts))
	for k, v := range counts {
		out = append(out, typeCount{Type: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func formatHeaderTimestamp(timestamp string) string {
	if timestamp == "" {
		timestamp = util.NowISO()
	}
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		// Try fallback formats.
		t, err = time.Parse(time.RFC3339, timestamp)
	}
	if err != nil {
		return timestamp
	}
	loc := t.Local()
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
		loc.Year(), loc.Month(), loc.Day(), loc.Hour(), loc.Minute(), loc.Second())
}

func removeLines(content, pattern string) (string, error) {
	rt := goja.New()
	if err := rt.Set("content", content); err != nil {
		return content, nil
	}
	if err := rt.Set("pattern", pattern); err != nil {
		return content, nil
	}
	v, err := rt.RunString(`(function(){var re = new RegExp(pattern); return String(content).split("\n").filter(function(line){ return !re.test(line); }).join("\n"); })()`)
	if err != nil {
		return content, nil
	}
	str, ok := v.Export().(string)
	if !ok {
		return content, nil
	}
	return str, nil
}

// jsReplace mirrors the TS transformer which defaults to "g" when no flags are
// provided: new RegExp(pattern, transform.flags || "g").
func jsReplace(content, pattern, replacement, flags string) (string, error) {
	if flags == "" {
		flags = "g"
	}
	rt := goja.New()
	if err := rt.Set("content", content); err != nil {
		return content, err
	}
	if err := rt.Set("pattern", pattern); err != nil {
		return content, err
	}
	if err := rt.Set("replacement", replacement); err != nil {
		return content, err
	}
	if err := rt.Set("flags", flags); err != nil {
		return content, err
	}
	v, err := rt.RunString(`(function(){var re = new RegExp(pattern, flags); return String(content).replace(re, replacement || ""); })()`)
	if err != nil {
		return content, nil
	}
	str, ok := v.Export().(string)
	if !ok {
		return content, nil
	}
	return str, nil
}
