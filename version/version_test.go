package version

import (
	"strings"
	"testing"
)

func TestCurrentUsesSourceAndInjectedVersions(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = ""
	source := strings.TrimSpace(sourceVersion)
	if got := Current(); got != source {
		t.Fatalf("source version = %q, want %q", got, source)
	}

	Version = "1.2.3"
	if got := Current(); got != "1.2.3" {
		t.Fatalf("injected version = %q, want 1.2.3", got)
	}
}
