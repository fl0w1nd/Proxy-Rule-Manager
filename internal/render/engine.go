package render

import (
	"fmt"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

// Render dispatches to the appropriate codec based on the template's codec field.
func Render(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	// Only the singbox codec can represent logical (AND/OR/NOT) entries.
	if tmpl.Codec != "singbox" {
		for _, entry := range entries {
			if entry.Kind.IsLogical() {
				return nil, fmt.Errorf("template %q does not support logical rule entries", tmpl.ID)
			}
		}
	}
	switch tmpl.Codec {
	case "linelist":
		return renderLineList(tmpl, entries)
	case "yaml_payload":
		return renderYAMLPayload(tmpl, entries)
	case "singbox":
		return renderSingbox(tmpl, entries)
	default:
		return nil, fmt.Errorf("unknown codec %q", tmpl.Codec)
	}
}

// resolveTypeName returns the output type name for an entry given the template.
func resolveTypeName(tmpl *Template, entry ir.Entry) (string, bool) {
	kind := string(entry.Kind)
	mapping, ok := tmpl.KindMap[kind]
	if !ok {
		return "", false
	}
	// Check for IPv6-specific type name
	if hint, ok := tmpl.Hints[kind]; ok && hint.IPv6TypeName != "" {
		if isIPv6Value(entry.Value) {
			return hint.IPv6TypeName, true
		}
	}
	name := mapping.TypeName
	if name == "" {
		name = mapping.FieldName
	}
	return name, name != ""
}

// resolveFieldName returns the output field name for singbox codec.
func resolveFieldName(tmpl *Template, entry ir.Entry) (string, bool) {
	kind := string(entry.Kind)
	mapping, ok := tmpl.KindMap[kind]
	if !ok {
		return "", false
	}
	name := mapping.FieldName
	if name == "" {
		name = mapping.TypeName
	}
	return name, name != ""
}

// isIPv6Value returns true if the value looks like an IPv6 CIDR or address.
func isIPv6Value(v string) bool {
	for _, c := range v {
		if c == ':' {
			return true
		}
		if c == '.' {
			return false
		}
	}
	return false
}

// shouldNoResolve returns true when the source entry carries no-resolve and
// the target template supports it for that kind.
func shouldNoResolve(tmpl *Template, entry ir.Entry) bool {
	if !entry.HasFlag(ir.FlagNoResolve) {
		return false
	}
	kind := string(entry.Kind)
	for _, k := range tmpl.FlagKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// applyTransform applies any value transform specified in hints.
func applyTransform(tmpl *Template, entry ir.Entry) string {
	kind := string(entry.Kind)
	hint, ok := tmpl.Hints[kind]
	if !ok || hint.Transform == "" {
		return entry.Value
	}
	return ApplyTransform(hint.Transform, entry.Value)
}

// formatEntryCore produces the (typeName, value, noResolve) triple shared by
// the linelist and yaml_payload codecs. Returns ok=false for logical entries
// or kinds absent from the template.
func formatEntryCore(tmpl *Template, entry ir.Entry) (typeName, value string, noResolve, ok bool) {
	if entry.Kind == ir.KindAnd || entry.Kind == ir.KindOr || entry.Kind == ir.KindNot {
		return "", "", false, false
	}
	typeName, ok = resolveTypeName(tmpl, entry)
	if !ok {
		return "", "", false, false
	}
	value = applyTransform(tmpl, entry)
	hint := tmpl.Hints[string(entry.Kind)]
	if hint.LeadingDot {
		value = ensureLeadingDot(value)
	}
	noResolve = shouldNoResolve(tmpl, entry)
	return typeName, value, noResolve, true
}
