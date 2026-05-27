package transformer

import (
	"encoding/json"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComputeFinalStats produces the post-pipeline summary shown at the top of
// the preview panel. Detection is conservative — we only invoke a parser
// when the content shape clearly matches the corresponding format header,
// and we fall back to classical line counting on any parse failure:
//
//  1. yaml-payload: content begins with `payload:` after optional blank
//     lines. Parsed with the YAML decoder; PayloadCount records the
//     payload-array length and ByType groups by classical leading token.
//
//  2. sing-box rule-set source: content begins with `{` (after optional
//     whitespace) and contains a top-level `rules` array of objects.
//     PayloadCount records the number of rule objects; ByType groups by
//     matcher field name (domain, domain_suffix, ip_cidr, …) and
//     TotalLines is the total number of matcher values. This replaces the
//     historical classical-line counting that would otherwise treat the
//     JSON braces and field labels as nonsensical "rules".
//
//  3. classical fallback: lines minus blanks and comments, grouped by the
//     leading comma-delimited token.
//
// The function lives in the transformer package because its semantics are
// fully owned by the rule-format layer; processor.go now only calls it.
func ComputeFinalStats(content string) FinalStats {
	if content == "" {
		return FinalStats{ByType: map[string]int{}, Format: FormatClassical}
	}
	if yamlPayloadHead.MatchString(content) {
		if stats, ok := computeYAMLPayloadStats(content); ok {
			return stats
		}
	}
	if jsonObjectHead.MatchString(content) {
		if stats, ok := computeSingboxSourceStats(content); ok {
			return stats
		}
	}
	return computeClassicalStats(content)
}

// yamlPayloadHead detects "payload:" at the document top after any leading
// blank lines. Used as a cheap pre-filter before invoking the YAML decoder.
var yamlPayloadHead = regexp.MustCompile(`(?m)\A\s*payload\s*:`)

// jsonObjectHead detects a top-level JSON object after optional whitespace.
// Cheap pre-filter so we don't even touch the JSON decoder on classical /
// yaml inputs that obviously don't qualify.
var jsonObjectHead = regexp.MustCompile(`\A\s*\{`)

// payloadDoc is the minimal shape we need from mihomo rule-set yaml. The
// !!str variants (single-quoted, double-quoted, plain) all decode into a
// plain Go string, so we don't have to worry about quoting style here —
// the regex-based predecessor did, and that's exactly the source of the
// "best-effort" caveat we want to remove.
type payloadDoc struct {
	Payload []string `yaml:"payload"`
}

// computeYAMLPayloadStats returns ok=true when the content was a valid
// mihomo `payload:` YAML document. The caller falls back to classical
// counting when ok=false (e.g. payload is not a string sequence, or
// the document fails to parse).
func computeYAMLPayloadStats(content string) (FinalStats, bool) {
	var doc payloadDoc
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return FinalStats{}, false
	}
	count := len(doc.Payload)
	byType := make(map[string]int, count)
	for _, item := range doc.Payload {
		typ := extractClassicalType(item)
		if typ == "" {
			typ = "UNKNOWN"
		}
		byType[typ]++
	}
	return FinalStats{
		TotalLines:   count,
		ByType:       byType,
		PayloadCount: &count,
		Format:       FormatYAMLPayload,
	}, true
}

// singboxSourceDoc captures the shape of a sing-box rule-set source
// document — at least the parts we need for stats. We deliberately leave
// each matcher field as a raw json.RawMessage so the same struct works
// for both string-arrays (domain, ip_cidr, …) and int-arrays (port,
// source_port). Length is recovered from the unmarshalled slice; we
// never need the individual values for stats, only their count.
type singboxSourceDoc struct {
	Version *int                         `json:"version,omitempty"`
	Rules   []map[string]json.RawMessage `json:"rules"`
}

// computeSingboxSourceStats returns ok=true when content decodes as a
// sing-box rule-set source document. We require an explicit `rules`
// array so a random JSON object (e.g. a {"version":N} stub or some
// other rule format that happens to start with `{`) doesn't get
// mis-categorised — without the array there's nothing meaningful to
// count and the caller's classical fallback produces a more honest
// "UNKNOWN: 1" row.
func computeSingboxSourceStats(content string) (FinalStats, bool) {
	var doc singboxSourceDoc
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return FinalStats{}, false
	}
	// Require the `rules` key to be present. json.Unmarshal leaves Rules
	// nil when the key is absent and a non-nil empty slice when it's
	// present and []; both shapes are valid sing-box documents but only
	// the latter (or a populated array) tells us we're looking at a
	// rule-set source — a plain {"version":N} could be anything.
	if !strings.Contains(content, "\"rules\"") {
		return FinalStats{}, false
	}

	ruleObjects := len(doc.Rules)
	byType := make(map[string]int)
	totalMatchers := 0
	for _, rule := range doc.Rules {
		for key, raw := range rule {
			// `invert` is a per-rule bool, not a matcher list, so it
			// shouldn't inflate the count. Likewise `type` (logical
			// rule discriminator) is metadata. Skip any field whose
			// value isn't a JSON array so we count matchers only.
			if !isJSONArray(raw) {
				continue
			}
			n := countJSONArrayItems(raw)
			if n == 0 {
				continue
			}
			byType[key] += n
			totalMatchers += n
		}
	}
	return FinalStats{
		TotalLines:   totalMatchers,
		ByType:       byType,
		PayloadCount: &ruleObjects,
		Format:       FormatSingboxSource,
	}, true
}

// isJSONArray reports whether raw is a JSON array. Skips leading
// whitespace because json.RawMessage preserves the original byte slice
// verbatim, including any indent the encoder happened to emit.
func isJSONArray(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}

// countJSONArrayItems returns the number of top-level items in a JSON
// array. We unmarshal into []json.RawMessage rather than re-tokenising
// the raw bytes so the count survives every legal whitespace / escape
// permutation a producer might emit. On parse failure we return 0;
// computeSingboxSourceStats treats that the same as an empty matcher
// (skipped from byType).
func countJSONArrayItems(raw json.RawMessage) int {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return 0
	}
	return len(items)
}

// computeClassicalStats counts meaningful classical-list lines and groups
// them by leading token. Blank lines and `#` / `;` comments are skipped so
// the count matches what mihomo / shadowrocket loaders actually see.
func computeClassicalStats(content string) FinalStats {
	byType := make(map[string]int)
	total := 0
	for _, line := range strings.Split(normalizeLineEndings(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		typ := extractClassicalType(trimmed)
		if typ == "" {
			typ = "UNKNOWN"
		}
		byType[typ]++
		total++
	}
	return FinalStats{TotalLines: total, ByType: byType, Format: FormatClassical}
}

// extractClassicalType returns the leading comma-delimited token of a
// classical rule line.
func extractClassicalType(line string) string {
	idx := strings.IndexByte(line, ',')
	if idx < 0 {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(line[:idx])
}

// CountSignificantLines counts non-blank, non-comment lines in content.
// Used by the reported pipeline to keep merge-step input counts aligned
// with ComputeFinalStats so the UI doesn't show a spurious mismatch
// between "merge: 10 → 10" and FinalStats showing 8 (where the
// difference is comment lines that ComputeFinalStats skips).
func CountSignificantLines(content string) int {
	if content == "" {
		return 0
	}
	n := 0
	for _, line := range strings.Split(normalizeLineEndings(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		n++
	}
	return n
}
