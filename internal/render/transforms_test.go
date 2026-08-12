package render

import "testing"

func TestWildcardToRegex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"*.example.com", `^.*\.example\.com$`},
		{"test?", `^test.$`},
		{"plain", `^plain$`},
	}
	for _, tt := range tests {
		got := wildcardToRegex(tt.input)
		if got != tt.want {
			t.Errorf("wildcardToRegex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestApplyTransform(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"uppercase", "hello", "HELLO"},
		{"lowercase", "HELLO", "hello"},
		{"strip_leading_dot", ".example.com", "example.com"},
		{"add_leading_dot", "example.com", ".example.com"},
		{"add_leading_dot", ".example.com", ".example.com"},
		{"unknown", "hello", "hello"},
	}
	for _, tt := range tests {
		got := ApplyTransform(tt.name, tt.value)
		if got != tt.want {
			t.Errorf("ApplyTransform(%q, %q) = %q, want %q", tt.name, tt.value, got, tt.want)
		}
	}
}
