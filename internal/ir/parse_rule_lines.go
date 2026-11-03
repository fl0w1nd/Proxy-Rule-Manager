package ir

import (
	"errors"
	"strings"
)

// ruleLineListParser handles line-based rule lists (format ID "classical"):
// the shared list syntax of mihomo classical providers, Surge rule sets,
// Shadowrocket, Quantumult X lists, and bare domain/IP plain lists. All
// entry-level grammar lives in entry_syntax.go; this file only iterates the
// lines. Lines without a TYPE,VALUE comma fall back to bare domain/IP
// classification (parsePlainItem), subsuming the former plain-list dialect.
type ruleLineListParser struct{}

func (ruleLineListParser) ID() string { return FormatClassical }

func (p ruleLineListParser) Parse(content string) RuleSet { return collect(p.walk, content) }

func (ruleLineListParser) walk(content string, visit visitFunc) {
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || isCommentLine(line) {
			continue
		}
		entry, err := parseRuleLine(line)
		if err == nil {
			visit(i+1, line, entry, nil)
			continue
		}
		// Lines without a comma fall back to bare domain/IP classification
		// (the classical format subsumes the plain-list dialect).
		if !strings.Contains(line, ",") {
			if pentry, perr := parsePlainItem(line); perr == nil {
				visit(i+1, line, pentry, nil)
				continue
			} else {
				err = perr // use the more specific plain-item error
			}
		}
		visit(i+1, line, Entry{}, err)
	}
}

// visitFunc receives every parsed item in document order: either an entry
// or the error that prevented one. line is 1-based, 0 for document-level
// problems.
type visitFunc func(line int, text string, entry Entry, err error)

// collect turns a parser walk into a RuleSet.
func collect(walk func(string, visitFunc), content string) RuleSet {
	var rs RuleSet
	walk(content, func(line int, text string, entry Entry, err error) {
		if err != nil {
			rs.Diagnostics = append(rs.Diagnostics, Diagnostic{Line: line, Text: text, Reason: err.Error()})
			return
		}
		rs.Entries = append(rs.Entries, entry)
	})
	return rs
}

var errNoYAMLPayload = errors.New("no payload/rules list found in YAML")
