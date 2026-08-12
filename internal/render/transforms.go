package render

import (
	"strings"
)

// ApplyTransform applies a named value transformation.
func ApplyTransform(name, value string) string {
	switch name {
	case "wildcard_to_regex":
		return wildcardToRegex(value)
	case "uppercase":
		return strings.ToUpper(value)
	case "lowercase":
		return strings.ToLower(value)
	case "strip_leading_dot":
		return strings.TrimPrefix(value, ".")
	case "add_leading_dot":
		if !strings.HasPrefix(value, ".") {
			return "." + value
		}
		return value
	default:
		return value
	}
}

// wildcardToRegex converts a simple wildcard pattern (* and ?) to a regex.
func wildcardToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for _, c := range pattern {
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '^', '$', '|', '(', ')', '[', ']', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteRune(c)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteString("$")
	return b.String()
}
