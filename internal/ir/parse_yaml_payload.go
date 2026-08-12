package ir

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlPayloadParser handles YAML payload documents (format ID "mihomo-yaml"):
// a `payload:` or `rules:` list as used by mihomo rule providers.
// It only unwraps the YAML envelope; each item is classified by the shared
// entry grammar in entry_syntax.go.
type yamlPayloadParser struct{}

func (yamlPayloadParser) ID() string { return FormatMihomoYAML }

type yamlPayloadItem struct {
	Value string
	Line  int
}

// Parse extracts the payload list and classifies each item: rule lines, bare
// CIDRs, or domain patterns.
func (yamlPayloadParser) Parse(content string) RuleSet {
	items, ok := extractYAMLItems(content)
	if !ok {
		return RuleSet{Diagnostics: []Diagnostic{{Text: "(document)", Reason: "no payload/rules list found in YAML"}}}
	}
	var rs RuleSet
	for i, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" || isCommentLine(value) {
			continue
		}
		entry, err := parseYAMLItem(value)
		if err != nil {
			line := item.Line
			if line == 0 {
				line = i + 1
			}
			rs.Diagnostics = append(rs.Diagnostics, Diagnostic{Line: line, Text: value, Reason: err.Error()})
			continue
		}
		rs.Entries = append(rs.Entries, entry)
	}
	return rs
}

// extractYAMLItems tries a strict YAML decode first, then falls back to the
// lenient line-scan mihomo itself uses (which tolerates broken YAML around
// the payload block).
func extractYAMLItems(content string) ([]yamlPayloadItem, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err == nil {
		root := &doc
		if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
			root = doc.Content[0]
		}
		if root.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(root.Content); i += 2 {
				key, value := root.Content[i], root.Content[i+1]
				if (key.Value != "payload" && key.Value != "rules") || value.Kind != yaml.SequenceNode {
					continue
				}
				items := make([]yamlPayloadItem, 0, len(value.Content))
				for _, node := range value.Content {
					var item string
					if err := node.Decode(&item); err == nil {
						items = append(items, yamlPayloadItem{Value: item, Line: node.Line})
					}
				}
				if len(items) > 0 {
					return items, true
				}
			}
		}
	}
	return scanYAMLItems(content)
}

// scanYAMLItems mirrors mihomo's line-oriented reader: find the payload/rules
// header, then collect `- item` lines.
func scanYAMLItems(content string) ([]yamlPayloadItem, bool) {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "payload:" || t == "rules:" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	var items []yamlPayloadItem
	for i, line := range lines[start:] {
		t := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.HasPrefix(t, "-") {
			break // end of the list block
		}
		item := strings.TrimSpace(strings.TrimPrefix(t, "-"))
		item = stripInlineYAMLComment(item)
		item = strings.Trim(item, `'"`)
		if item != "" {
			items = append(items, yamlPayloadItem{Value: item, Line: start + i + 1})
		}
	}
	return items, len(items) > 0
}

// stripInlineYAMLComment removes ` # ...` trailing comments outside quotes.
func stripInlineYAMLComment(s string) string {
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return strings.TrimSpace(s)
}

// parseYAMLItem classifies one payload item: a rule line if it starts with a
// known type name followed by a comma, otherwise a bare domain/IP pattern.
func parseYAMLItem(item string) (Entry, error) {
	if comma := strings.Index(item, ","); comma > 0 {
		typeName := strings.ToUpper(strings.TrimSpace(item[:comma]))
		if _, known := ruleTypeSpellings[typeName]; known {
			return parseRuleLine(item)
		}
		if _, nonEntry := nonEntryTypes[typeName]; nonEntry {
			return parseRuleLine(item) // yields the proper diagnostic reason
		}
		switch typeName {
		case "AND", "OR", "NOT":
			return parseRuleLine(item)
		}
	}
	return parsePlainItem(item)
}
