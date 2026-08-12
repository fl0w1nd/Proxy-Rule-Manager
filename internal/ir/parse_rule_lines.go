package ir

import "strings"

// ruleLineListParser handles line-based rule lists (format ID "classical"):
// the shared list syntax of mihomo classical providers, Surge rule sets,
// Shadowrocket, Quantumult X lists, and bare domain/IP plain lists. All
// entry-level grammar lives in entry_syntax.go; this file only iterates the
// lines. Lines without a TYPE,VALUE comma fall back to bare domain/IP
// classification (parsePlainItem), subsuming the former plain-list dialect.
type ruleLineListParser struct{}

func (ruleLineListParser) ID() string { return FormatClassical }

func (ruleLineListParser) Parse(content string) RuleSet {
	var rs RuleSet
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || isCommentLine(line) {
			continue
		}
		entry, err := parseRuleLine(line)
		if err == nil {
			rs.Entries = append(rs.Entries, entry)
			continue
		}
		// Lines without a comma fall back to bare domain/IP classification
		// (the classical format subsumes the plain-list dialect).
		if !strings.Contains(line, ",") {
			if pentry, perr := parsePlainItem(line); perr == nil {
				rs.Entries = append(rs.Entries, pentry)
				continue
			} else {
				err = perr // use the more specific plain-item error
			}
		}
		rs.Diagnostics = append(rs.Diagnostics, Diagnostic{Line: i + 1, Text: line, Reason: err.Error()})
	}
	return rs
}
