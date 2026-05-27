package transformer

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComputeFinalStats produces the post-pipeline summary shown at the top of
// the preview panel. Detection is conservative:
//
//   - Content that starts (after optional blank lines) with `payload:` is
//     attempted as a mihomo payload YAML document. Success populates
//     PayloadCount and groups entries by their leading classical token.
//   - Anything else (including YAML that fails to decode) is treated as a
//     classical list: lines are counted minus blanks and comments, and
//     grouped by leading token.
//
// The function lives in the transformer package because its semantics are
// fully owned by the rule-format layer; processor.go now only calls it.
func ComputeFinalStats(content string) FinalStats {
	if content == "" {
		return FinalStats{ByType: map[string]int{}}
	}
	if yamlPayloadHead.MatchString(content) {
		if stats, ok := computeYAMLPayloadStats(content); ok {
			return stats
		}
	}
	return computeClassicalStats(content)
}

// yamlPayloadHead detects "payload:" at the document top after any leading
// blank lines. Used as a cheap pre-filter before invoking the YAML decoder.
var yamlPayloadHead = regexp.MustCompile(`(?m)\A\s*payload\s*:`)

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
	}, true
}

// computeClassicalStats counts meaningful classical-list lines and groups
// them by leading token. Blank lines and `#` / `;` comments are skipped so
// the count matches what mihomo / shadowrocket loaders actually see.
func computeClassicalStats(content string) FinalStats {
	byType := make(map[string]int)
	total := 0
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
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
	return FinalStats{TotalLines: total, ByType: byType}
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
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
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
