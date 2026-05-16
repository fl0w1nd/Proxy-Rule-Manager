package syncengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

func newReconcileEnv(t *testing.T) (*store.Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	paths := store.Paths{
		DataDir:       dir,
		RulesDir:      filepath.Join(dir, "Rules"),
		SourcesDir:    filepath.Join(dir, "sources"),
		GeositeDir:    filepath.Join(dir, "geosite"),
		IconSetDir:    filepath.Join(dir, "iconset"),
		ClientFileDir: filepath.Join(dir, "client"),
		WAFDir:        filepath.Join(dir, "waf"),
	}
	st, err := store.Open(filepath.Join(dir, "db.sqlite"), paths)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, paths.RulesDir, paths.ClientFileDir
}

func TestCheckConsistencyDetectsMissingArtifact(t *testing.T) {
	st, rulesDir, cfDir := newReconcileEnv(t)
	ctx := context.Background()
	if err := st.AddClient(ctx, schema.ClientConfig{ID: "c1", DisplayName: "c1"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{{
		Name:    "R1",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtrR("payload")}},
		Output:  schema.OutputConfig{Clients: []string{"c1"}},
	}}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	// Insert an artifact row pointing at a file we never write.
	size := int64(7)
	meta := schema.ArtifactMeta{
		RuleName: "R1", Client: "c1",
		LastHash: "x", LastUpdatedAt: "now",
		BlobPath: "/Rules/c1/R1.list", SizeBytes: &size,
	}
	if err := st.SaveArtifactMetas(ctx, []schema.ArtifactMeta{meta}); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}

	report, err := CheckConsistency(ctx, st, rulesDir, cfDir)
	if err != nil {
		t.Fatalf("CheckConsistency: %v", err)
	}
	if !hasIssueOfType(report.Issues, "artifact_file_missing") {
		t.Fatalf("expected artifact_file_missing issue, got %+v", report.Issues)
	}
}

func TestCheckConsistencyDetectsOrphanArtifact(t *testing.T) {
	st, rulesDir, cfDir := newReconcileEnv(t)
	ctx := context.Background()
	if err := st.AddClient(ctx, schema.ClientConfig{ID: "c1", DisplayName: "c1"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rulesDir, "c1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "c1", "Ghost.list"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	report, err := CheckConsistency(ctx, st, rulesDir, cfDir)
	if err != nil {
		t.Fatalf("CheckConsistency: %v", err)
	}
	if !hasIssueOfType(report.Issues, "artifact_orphan") {
		t.Fatalf("expected artifact_orphan issue, got %+v", report.Issues)
	}
}

func TestCheckConsistencyDetectsTempFile(t *testing.T) {
	st, rulesDir, cfDir := newReconcileEnv(t)
	ctx := context.Background()
	if err := st.AddClient(ctx, schema.ClientConfig{ID: "c1", DisplayName: "c1"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rulesDir, "c1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Leftover atomic-write temp file format: .<name>.<rand>.tmp
	if err := os.WriteFile(filepath.Join(rulesDir, "c1", ".Foo.list.abc.tmp"), []byte("partial"), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	report, err := CheckConsistency(ctx, st, rulesDir, cfDir)
	if err != nil {
		t.Fatalf("CheckConsistency: %v", err)
	}
	if !hasIssueOfType(report.Issues, "temp_file") {
		t.Fatalf("expected temp_file issue, got %+v", report.Issues)
	}
}

func hasIssueOfType(issues []ConsistencyIssue, typ string) bool {
	for _, iss := range issues {
		if iss.Type == typ {
			return true
		}
	}
	return false
}

func strPtrR(s string) *string { return &s }
