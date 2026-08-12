package render

import (
	"fmt"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

// renderYAMLPayload renders entries as a mihomo YAML payload document:
//
//	payload:
//	  - 'TYPE,value'
func renderYAMLPayload(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	var b strings.Builder
	b.WriteString("payload:\n")
	written := 0
	for _, entry := range entries {
		line, ok := renderYAMLPayloadEntry(tmpl, entry)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "  - '%s'\n", line)
		written++
	}
	if written == 0 {
		return nil, nil
	}
	return []byte(b.String()), nil
}

func renderYAMLPayloadEntry(tmpl *Template, entry ir.Entry) (string, bool) {
	typeName, value, noResolve, ok := formatEntryCore(tmpl, entry)
	if !ok {
		return "", false
	}

	if noResolve {
		return escapeYAMLSingleQuoted(fmt.Sprintf("%s,%s,no-resolve", typeName, value)), true
	}
	return escapeYAMLSingleQuoted(fmt.Sprintf("%s,%s", typeName, value)), true
}

func escapeYAMLSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
