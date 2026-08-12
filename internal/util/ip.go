package util

import "strings"

// ClientIP extracts the client IP from common proxy headers.
func ClientIP(getHeader func(name string) string) string {
	if v := getHeader("x-forwarded-for"); v != "" {
		if idx := strings.Index(v, ","); idx > 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	if v := getHeader("x-real-ip"); v != "" {
		return v
	}
	if v := getHeader("cf-connecting-ip"); v != "" {
		return v
	}
	return "unknown"
}
