package syncengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

// openTestStore creates a temporary store suitable for engine tests.
func openTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	paths := store.Paths{
		DataDir:       dir,
		RulesDir:      filepath.Join(dir, "rules"),
		SourcesDir:    filepath.Join(dir, "sources"),
		GeositeDir:    filepath.Join(dir, "geosite"),
		IconSetDir:    filepath.Join(dir, "iconset"),
		ClientFileDir: filepath.Join(dir, "client"),
		WAFDir:        filepath.Join(dir, "waf"),
	}
	st, err := store.Open(filepath.Join(dir, "db.sqlite"), paths)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, dir
}

// TestExecuteFullSync_FlushArtifactFailure_MarksFailedRules verifies that when
// disk writes fail, the engine returns Success=false AND the rule appears in
// FailedRules. Before the fix, flushArtifact errors were recorded as
// failureRecords but NOT added to failedRules, so Success was incorrectly true.
func TestExecuteFullSync_FlushArtifactFailure_MarksFailedRules(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()

	// Write a regular file at the rulesDir path so that os.MkdirAll inside
	// UploadRuleContent fails — simulating a write failure without needing
	// to manipulate filesystem permissions.
	blockedRulesDir := filepath.Join(dir, "blocked-rules")
	if err := os.WriteFile(blockedRulesDir, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("setup blocked rulesDir: %v", err)
	}

	contentStr := "DOMAIN,example.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:    "test-rule",
				Sources: []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Output:  schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:    []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Engine points at a path that cannot be created (file where dir is expected).
	engine := NewEngine(st, nil, blockedRulesDir)

	result, err := engine.ExecuteFullSync(ctx)
	if err != nil {
		t.Fatalf("ExecuteFullSync returned unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false when flushArtifact fails, got true")
	}
	found := false
	for _, fr := range result.FailedRules {
		if fr.Name == "test-rule" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'test-rule' in FailedRules, got: %+v", result.FailedRules)
	}
}

// TestExecutePartialSync_FlushArtifactFailure_MarksFailedRules mirrors the
// full-sync test but exercises the partial-sync (executeSelective) code path.
func TestExecutePartialSync_FlushArtifactFailure_MarksFailedRules(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()

	blockedRulesDir := filepath.Join(dir, "blocked-rules")
	if err := os.WriteFile(blockedRulesDir, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("setup blocked rulesDir: %v", err)
	}

	contentStr2 := "DOMAIN,partial.example.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:    "partial-rule",
				Sources: []schema.SourceConfig{{Type: "local", Content: &contentStr2}},
				Output:  schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:    []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	engine := NewEngine(st, nil, blockedRulesDir)

	result, err := engine.ExecutePartialSync(ctx, "partial-rule")
	if err != nil {
		t.Fatalf("ExecutePartialSync returned unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false when flushArtifact fails in partial sync, got true")
	}
	found := false
	for _, fr := range result.FailedRules {
		if fr.Name == "partial-rule" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'partial-rule' in FailedRules, got: %+v", result.FailedRules)
	}
}

func TestExecutePartialSync_MissingSeedFails(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()

	contentStr := "DOMAIN,example.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:    "existing-rule",
				Sources: []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Output:  schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:    []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	engine := NewEngine(st, nil, filepath.Join(t.TempDir(), "rules"))
	result, err := engine.ExecutePartialSync(ctx, "missing-rule")
	if err != nil {
		t.Fatalf("ExecutePartialSync: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure for missing seed, got success")
	}
	if len(result.FailedRules) != 1 || result.FailedRules[0].Name != "missing-rule" {
		t.Fatalf("unexpected failed rules: %+v", result.FailedRules)
	}
}
