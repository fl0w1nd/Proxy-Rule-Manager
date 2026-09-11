package render

import "testing"

func TestApplyTransform(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"uppercase", "hello", "HELLO"},
		{"lowercase", "HELLO", "hello"},
		{"strip_leading_dot", ".example.com", "example.com"},
		{"add_leading_dot", "example.com", ".example.com"},
		{"add_leading_dot", ".example.com", ".example.com"},
		{"wildcard_to_regex", "*.example.com", `^.*\.example\.com$`},
		{"wildcard_to_regex", "test?", `^test.$`},
		{"wildcard_to_regex", "plain", `^plain$`},
		{"unknown", "hello", "hello"},
	}
	for _, tt := range tests {
		got := ApplyTransform(tt.name, tt.value)
		if got != tt.want {
			t.Errorf("ApplyTransform(%q, %q) = %q, want %q", tt.name, tt.value, got, tt.want)
		}
	}
}
