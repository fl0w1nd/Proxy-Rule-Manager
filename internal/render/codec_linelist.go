package render

import (
	"fmt"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

// renderLineList renders entries as TYPE,VALUE lines (mihomo-classical, surge,
// shadowrocket formats). Each flat entry becomes one line.
func renderLineList(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	var b strings.Builder
	for _, entry := range entries {
		line, ok := renderLineListEntry(tmpl, entry)
		if !ok {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func renderLineListEntry(tmpl *Template, entry ir.Entry) (string, bool) {
	typeName, value, noResolve, ok := formatEntryCore(tmpl, entry)
	if !ok {
		return "", false
	}

	hint := tmpl.Hints[string(entry.Kind)]
	if hint.SplitPortRange {
		return renderSplitPorts(typeName, value, tmpl, entry), true
	}

	if noResolve {
		return fmt.Sprintf("%s,%s,no-resolve", typeName, value), true
	}
	return fmt.Sprintf("%s,%s", typeName, value), true
}

func ensureLeadingDot(v string) string {
	if !strings.HasPrefix(v, ".") {
		return "." + v
	}
	return v
}

// renderSplitPorts handles port entries that need to be split into individual
// lines (e.g. Surge wants one DST-PORT per port/range).
func renderSplitPorts(typeName, value string, tmpl *Template, entry ir.Entry) string {
	sep := "/"
	if hint, ok := tmpl.Hints[string(entry.Kind)]; ok && hint.PortRangeSep != "" {
		sep = hint.PortRangeSep
	}

	parts := strings.Split(value, "/")
	var lines []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if sep != "/" {
			p = strings.ReplaceAll(p, "-", sep)
		}
		lines = append(lines, fmt.Sprintf("%s,%s", typeName, p))
	}
	return strings.Join(lines, "\n")
}
