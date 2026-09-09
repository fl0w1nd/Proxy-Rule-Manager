package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceGroupValidation(t *testing.T) {
	for _, tc := range []struct{ name, sources, want string }{
		{"valid", "- group:\n    - content: DOMAIN,a.example\n  preprocess: \"\"\n", ""},
		{"empty", "- group: []\n", "at least one source"},
		{"nested", "- group:\n    - group:\n        - content: DOMAIN,a.example\n", "cannot be nested"},
		{"selector", "- group:\n    - content: DOMAIN,a.example\n  url: https://example.com\n", "group cannot configure"},
		{"unknown ref", "- group:\n    - ref: missing\n", "unknown rule ID"},
		{"member script", "- group:\n    - content: DOMAIN,a.example\n      preprocess: \"\"\n", "configure preprocessing and ops on the group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := "clients:\n  - id: mihomo\n    name: Mihomo\n    template: mihomo-classical\nrules:\n  - id: example\n    name: Example\n    sources:\n"
			for _, line := range strings.Split(strings.TrimSuffix(tc.sources, "\n"), "\n") {
				document += "      " + line + "\n"
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(document), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path, t.TempDir())
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("wanted %q, got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			clone := cfg.DeepCopy()
			*clone.Rules[0].Sources[0].Preprocess = "changed"
			clone.Rules[0].Sources[0].Group[0].Content = "changed"
			if *cfg.Rules[0].Sources[0].Preprocess != "" || cfg.Rules[0].Sources[0].Group[0].Content != "DOMAIN,a.example" {
				t.Fatal("source group copy aliases the original")
			}
		})
	}
}
