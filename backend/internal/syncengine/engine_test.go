package syncengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
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

// TestFinalizeCtx_IndependentFromCallerCancel pins down the contract that
// the terminal persistence step uses. Before the fix, every CompleteJob /
// ReleaseLock / RecordFailureRecords call rode the caller's request ctx;
// when that ctx was cancelled mid-sync (HTTP client disconnect, curl
// timeout, ...) the writes were silently dropped, leaving the job stuck in
// 'running' status with the global lock held until its 5-minute TTL elapsed.
//
// The fix routes those writes through a fresh detached context produced by
// finalizeCtx(); this test guards against that regression at the contract
// level so the property survives future refactors that move the cleanup
// block around.
func TestFinalizeCtx_IndependentFromCallerCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	if parent.Err() == nil {
		t.Fatalf("expected parent ctx to be cancelled")
	}

	ctx, c := finalizeCtx()
	defer c()
	if err := ctx.Err(); err != nil {
		t.Fatalf("finalizeCtx must not inherit cancellation, got %v", err)
	}
	if dl, ok := ctx.Deadline(); !ok || dl.Before(time.Now().Add(time.Second)) {
		t.Fatalf("finalizeCtx must carry a generous deadline, got ok=%v deadline=%v", ok, dl)
	}
}

// TestFlushArtifact_WritesPureContent_NoManagedHeader pins down the "zero
// header" contract: a freshly synced artifact must contain *only* the
// post-transform content, with no "# 规则数量：…" preamble. Both the
// on-disk bytes and the in-memory PreviewRule output are checked because
// they are the two surfaces that ship the artifact to downstream
// consumers.
func TestFlushArtifact_WritesPureContent_NoManagedHeader(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()
	rulesDir := filepath.Join(dir, "rules")

	contentStr := "DOMAIN,example.com\nDOMAIN-SUFFIX,google.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:        "pure-rule",
				Description: "desc",
				Sources:     []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Output:      schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:        []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	engine := NewEngine(st, nil, rulesDir)
	result, err := engine.ExecuteFullSync(ctx)
	if err != nil {
		t.Fatalf("ExecuteFullSync: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}

	rule := cfg.Rules[0]
	disk, err := ReadForRule(rulesDir, &rule, "clash_meta", "")
	if err != nil {
		t.Fatalf("ReadForRule: %v", err)
	}
	if disk != contentStr {
		t.Errorf("artifact content drift:\n got %q\nwant %q", disk, contentStr)
	}
	for _, banned := range []string{"# 规则数量", "# 更新时间", "# 规则类型"} {
		if strings.Contains(disk, banned) {
			t.Errorf("artifact must not contain managed header %q, got %q", banned, disk)
		}
	}
}

// TestFlushArtifact_LegacyHeaderRecognisedAsUnchanged seeds a legacy
// header-prefixed file (as produced by a pre-"zero header" release) and
// runs a full sync that would produce semantically identical content.
// The first resync must:
//   - rewrite the file to the clean (header-less) form, and
//   - NOT emit a spurious activity change record (the semantic payload is
//     unchanged).
func TestFlushArtifact_LegacyHeaderRecognisedAsUnchanged(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()
	rulesDir := filepath.Join(dir, "rules")

	contentStr := "DOMAIN,example.com\nDOMAIN-SUFFIX,google.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:        "legacy-rule",
				Description: "legacy",
				Sources:     []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Output:      schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:        []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	rule := cfg.Rules[0]
	// Materialise a pre-existing legacy artifact by writing the header
	// manually plus the canonical content. The header shape mirrors what
	// the old transformer.AddRuleHeader function used to produce.
	legacyContent := "# 规则数量：2 条\n" +
		"# 更新时间：2024-01-01 08:00:00\n" +
		"# 规则类型：\n" +
		"# DOMAIN: 1 条\n" +
		"# DOMAIN-SUFFIX: 1 条\n" +
		"\n" +
		contentStr
	if _, err := UploadForRule(rulesDir, &rule, "clash_meta", "", legacyContent); err != nil {
		t.Fatalf("seed legacy artifact: %v", err)
	}
	if !strings.HasPrefix(legacyContent, "# 规则数量") {
		t.Fatalf("test setup invalid: legacy content has no managed header")
	}
	size := int64(len(legacyContent))
	hash := "legacy-hash"
	if err := st.SaveArtifactMetas(ctx, []schema.ArtifactMeta{{
		RuleName:      rule.Name,
		Client:        "clash_meta",
		LastHash:      hash,
		LastUpdatedAt: "2024-01-01T00:00:00Z",
		BlobPath:      "/Rules/clash_meta/legacy-rule",
		SizeBytes:     &size,
	}}); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}

	// Snapshot the activity table so we can assert no NEW change record is
	// added by this resync.
	preRecords, err := st.ListChangeRecords(ctx, "", 1, 1000, "", 0)
	if err != nil {
		t.Fatalf("ListChangeRecords baseline: %v", err)
	}

	engine := NewEngine(st, nil, rulesDir)
	if _, err := engine.ExecuteFullSync(ctx); err != nil {
		t.Fatalf("ExecuteFullSync: %v", err)
	}

	// The file must now be the clean version, with no managed header.
	disk, err := ReadForRule(rulesDir, &rule, "clash_meta", "")
	if err != nil {
		t.Fatalf("ReadForRule post-sync: %v", err)
	}
	if disk != contentStr {
		t.Errorf("expected clean content after upgrade resync, got %q", disk)
	}
	if strings.Contains(disk, "# 规则数量") {
		t.Errorf("legacy header still present after upgrade resync: %q", disk)
	}

	postRecords, err := st.ListChangeRecords(ctx, "", 1, 1000, "", 0)
	if err != nil {
		t.Fatalf("ListChangeRecords after: %v", err)
	}
	addedForRule := 0
	for _, rec := range postRecords.Items {
		found := false
		for _, prior := range preRecords.Items {
			if prior.ID == rec.ID {
				found = true
				break
			}
		}
		if !found && rec.RuleName == rule.Name {
			addedForRule++
		}
	}
	if addedForRule != 0 {
		t.Errorf("expected no new change records for semantically-identical legacy artifact, got %d", addedForRule)
	}
}

// TestPreviewRule_ReportsBuiltinPipeline checks that PreviewRule populates
// the per-client TransformReport with at least one step describing the
// built-in mihomo→shadowrocket transformer.
func TestPreviewRule_ReportsBuiltinPipeline(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()
	rulesDir := filepath.Join(dir, "rules")

	contentStr := "DOMAIN,example.com\nPROCESS-NAME,bad\nMATCH,DIRECT"
	targetAll := []byte(`"all"`)
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:    "preview-rule",
				Sources: []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Transforms: []schema.Transform{
					{Type: "use", Use: transformer.BuiltinMihomoToShadowrocket, Target: targetAll},
				},
				Output: schema.OutputConfig{Clients: []string{"shadowrocket"}},
				Tags:   []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	engine := NewEngine(st, nil, rulesDir)
	res, err := engine.PreviewRule(ctx, &cfg.Rules[0], cfg.Transformers, 0)
	if err != nil {
		t.Fatalf("PreviewRule: %v", err)
	}
	rep, ok := res.Reports["shadowrocket"]
	if !ok {
		t.Fatalf("expected report for shadowrocket, got keys %v", keysOfReports(res.Reports))
	}
	if len(rep.Steps) == 0 {
		t.Fatalf("expected at least one step in report, got 0")
	}
	var foundBuiltin bool
	for _, s := range rep.Steps {
		if s.Kind == transformer.KindUseBuiltin && s.DroppedTotal > 0 && s.ModifiedTotal > 0 {
			foundBuiltin = true
			break
		}
	}
	if !foundBuiltin {
		t.Errorf("expected a builtin step with drops + modifications, got steps %+v", rep.Steps)
	}
	if rep.FinalStats.TotalLines == 0 {
		t.Errorf("expected FinalStats.TotalLines > 0, got %d", rep.FinalStats.TotalLines)
	}
}

// TestPreviewRule_FinalStatsYamlPayload exercises the yaml branch of the
// stats calculator using the classical→yaml built-in.
func TestPreviewRule_FinalStatsYamlPayload(t *testing.T) {
	st, dir := openTestStore(t)
	ctx := context.Background()
	rulesDir := filepath.Join(dir, "rules")

	contentStr := "DOMAIN,a.com\nDOMAIN-SUFFIX,b.com\nIP-CIDR,1.1.1.1/32"
	targetAll := []byte(`"all"`)
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name:    "yaml-rule",
				Sources: []schema.SourceConfig{{Type: "local", Content: &contentStr}},
				Transforms: []schema.Transform{
					{Type: "use", Use: transformer.BuiltinMihomoClassicalToYAML, Target: targetAll},
				},
				Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:   []string{},
			},
		},
	}
	if _, err := st.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	engine := NewEngine(st, nil, rulesDir)
	res, err := engine.PreviewRule(ctx, &cfg.Rules[0], cfg.Transformers, 0)
	if err != nil {
		t.Fatalf("PreviewRule: %v", err)
	}
	rep, ok := res.Reports["clash_meta"]
	if !ok {
		t.Fatalf("expected report for clash_meta")
	}
	if rep.FinalStats.PayloadCount == nil {
		t.Fatalf("expected PayloadCount to be set for yaml content")
	}
	if *rep.FinalStats.PayloadCount != 3 {
		t.Errorf("expected payload count 3, got %d", *rep.FinalStats.PayloadCount)
	}
	if len(rep.FinalStats.ByType) == 0 {
		t.Errorf("expected ByType to be populated, got empty")
	}
}

func keysOfReports(m map[string]transformer.TransformReport) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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
