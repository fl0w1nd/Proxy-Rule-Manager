package syncengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestRuleArtifactPath_UsesProvidedExtAndFallsBackToDefault locks in the
// "client.outputExt drives the on-disk filename" contract and verifies that
// an empty ext defaults to `.list` so the historical zero-config behaviour
// keeps working unchanged.
func TestRuleArtifactPath_UsesProvidedExtAndFallsBackToDefault(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name       string
		ext        string
		wantSuffix string
	}{
		{"empty ext falls back to default", "", ".list"},
		{"yaml ext is honored", "yaml", ".yaml"},
		{"leading dot stripped", ".json", ".json"},
		{"mixed case lowered", "SRS", ".srs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := RuleArtifactPath(root, "MyRule", "clash_meta", tt.ext)
			if err != nil {
				t.Fatalf("RuleArtifactPath: %v", err)
			}
			wantFile := filepath.Join(root, "clash_meta", "MyRule"+tt.wantSuffix)
			if res.FilePath != wantFile {
				t.Fatalf("FilePath = %q, want %q", res.FilePath, wantFile)
			}
			if got, want := res.URL, "/Rules/clash_meta/MyRule"+tt.wantSuffix; got != want {
				t.Fatalf("URL = %q, want %q", got, want)
			}
		})
	}
}

// TestUploadAndReadRuleContent_RoundTripCustomExt verifies the full
// upload->read cycle for a non-default extension so we know the on-disk
// layout produced by UploadRuleContent is what ReadRuleContent will look
// for. Without this, a future drift between the two helpers (e.g. one
// hard-codes `.list`, the other reads `.yaml`) would silently leak.
func TestUploadAndReadRuleContent_RoundTripCustomExt(t *testing.T) {
	root := t.TempDir()
	payload := "DOMAIN-SUFFIX,example.com\n"

	if _, err := UploadRuleContent(root, "rule_a", "client_yaml", "yaml", payload); err != nil {
		t.Fatalf("UploadRuleContent: %v", err)
	}
	// File is on disk with the yaml extension.
	if _, err := os.Stat(filepath.Join(root, "client_yaml", "rule_a.yaml")); err != nil {
		t.Fatalf("expected rule_a.yaml on disk: %v", err)
	}
	got, err := ReadRuleContent(root, "rule_a", "client_yaml", "yaml")
	if err != nil {
		t.Fatalf("ReadRuleContent: %v", err)
	}
	if got != payload {
		t.Fatalf("ReadRuleContent = %q, want %q", got, payload)
	}
	// A read with the WRONG ext returns empty (file-not-found path).
	stale, err := ReadRuleContent(root, "rule_a", "client_yaml", "list")
	if err != nil {
		t.Fatalf("ReadRuleContent stale ext: %v", err)
	}
	if stale != "" {
		t.Fatalf("ReadRuleContent with mismatching ext should return empty, got %q", stale)
	}
}

// TestRemoveArtifactFile_HonorsExt makes sure deletion finds the right
// file. Previously RemoveArtifactFile always tried to delete `.list`, so
// renaming a client to yaml would silently leave orphan files when a rule
// was deleted.
func TestRemoveArtifactFile_HonorsExt(t *testing.T) {
	root := t.TempDir()
	rule := &schema.RuleConfig{Name: "rule_b"}
	if _, err := UploadForRule(root, rule, "shadowrocket", "json", "{}"); err != nil {
		t.Fatalf("UploadForRule: %v", err)
	}
	path := filepath.Join(root, "shadowrocket", "rule_b.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := RemoveArtifactFile(root, rule, "shadowrocket", "json"); err != nil {
		t.Fatalf("RemoveArtifactFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, stat err=%v", err)
	}
}
