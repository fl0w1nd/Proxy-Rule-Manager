package version

import "testing"

func TestCurrentUsesSourceAndInjectedVersions(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = ""
	if got := Current(); got != "0.0.1" {
		t.Fatalf("source version = %q, want 0.0.1", got)
	}

	Version = "1.2.3"
	if got := Current(); got != "1.2.3" {
		t.Fatalf("injected version = %q, want 1.2.3", got)
	}
}
