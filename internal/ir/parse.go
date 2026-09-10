package ir

import (
	"fmt"
	"strings"
)

// Format identifiers reported by automatic detection.
const (
	FormatClassical  = "classical"
	FormatMihomoYAML = "mihomo-yaml"
)

// detectOrder fixes the sniffing order so equal scores resolve deterministically.
var detectOrder = []string{FormatClassical, FormatMihomoYAML}

type parser interface {
	ID() string
	Parse(content string) RuleSet
}

var parsers = map[string]parser{
	FormatClassical:  ruleLineListParser{},
	FormatMihomoYAML: yamlPayloadParser{},
}

// Detection is the result of format sniffing.
type Detection struct {
	Format     string  `json:"format"`
	Confidence float64 `json:"confidence"` // 0..1
}

// Detect scores the content against every parser and returns the best match.
// A zero-confidence result means nothing matched at all.
func Detect(content string) Detection {
	best := Detection{Format: FormatClassical, Confidence: 0}
	for _, f := range detectOrder {
		score := sniff(f, content)
		if score > best.Confidence {
			best = Detection{Format: f, Confidence: score}
		}
	}
	return best
}

// Parse sniffs content and returns the rule set plus the detection used.
// Undetectable content is an error; sources do not declare their format.
func Parse(content string) (RuleSet, Detection, error) {
	det := Detect(content)
	if det.Confidence <= 0 {
		return RuleSet{}, det, fmt.Errorf("could not detect rule format")
	}
	return parsers[det.Format].Parse(content), det, nil
}

// sniff returns a 0..1 confidence score for parsing content as format.
func sniff(format, content string) float64 {
	switch format {
	case FormatMihomoYAML:
		return sniffYAMLPayload(content)
	case FormatClassical:
		return sniffLines(content, func(line string) bool {
			comma := strings.Index(line, ",")
			if comma > 0 {
				t := strings.ToUpper(strings.TrimSpace(line[:comma]))
				if _, ok := ruleTypeSpellings[t]; ok {
					return true
				}
				if _, ok := nonEntryTypes[t]; ok {
					return true
				}
				return t == "AND" || t == "OR" || t == "NOT"
			}
			// Bare domain/IP items are also accepted (plain-list dialect
			// subsumed by classical).
			_, err := parsePlainItem(line)
			return err == nil
		})
	}
	return 0
}

func sniffYAMLPayload(content string) float64 {
	// Cheap header check: if the document has a payload: or rules: key,
	// it is almost certainly a mihomo YAML provider. The full parse is
	// left to the actual parser, avoiding a double YAML decode.
	for _, raw := range strings.Split(content, "\n") {
		t := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if t == "payload:" || t == "rules:" {
			return 0.95
		}
	}
	return 0
}

// sniffLines computes the fraction of non-comment lines accepted by match,
// sampling at most 400 lines for large documents.
func sniffLines(content string, match func(string) bool) float64 {
	total, good := 0, 0
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || isCommentLine(line) {
			continue
		}
		total++
		if match(line) {
			good++
		}
		if total >= 400 {
			break
		}
	}
	if total == 0 {
		return 0
	}
	return float64(good) / float64(total)
}
