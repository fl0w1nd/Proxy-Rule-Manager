package transformer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

var targetAll = json.RawMessage(`"all"`)

func targetIndices(idx ...int) json.RawMessage {
	raw, err := json.Marshal(idx)
	if err != nil {
		panic(err)
	}
	return raw
}

// testCtx wraps the built-in registry in a PipelineCtx so the existing
// tests don't have to repeat the builder at every call site.
func testCtx() PipelineCtx {
	return PipelineCtx{Transformers: BuiltinTransformers()}
}

// TestApplyNewTransformsReported_BuiltinStepHasDropAndModifyTotals verifies
// that the reported pipeline propagates the dropped/modified buckets from
// the built-in transformer all the way into StepReport, and stamps stage +
// kind correctly.
func TestApplyNewTransformsReported_BuiltinStepHasDropAndModifyTotals(t *testing.T) {
	eng := NewEngine()
	in := []string{strings.Join([]string{
		"DOMAIN,example.com",
		"PROCESS-NAME,bad",
		"MATCH,DIRECT",
	}, "\n")}
	transforms := []schema.Transform{
		{Type: "use", Use: BuiltinMihomoToShadowrocket, Target: targetAll},
	}
	out, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageRule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 output stream, got %d", len(out))
	}
	wantOut := "DOMAIN,example.com\nFINAL,DIRECT"
	if out[0] != wantOut {
		t.Fatalf("unexpected output: %q", out[0])
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step report, got %d", len(steps))
	}
	step := steps[0]
	if step.Stage != StageRule {
		t.Errorf("stage = %q, want %q", step.Stage, StageRule)
	}
	if step.Kind != KindUseBuiltin {
		t.Errorf("kind = %q, want %q", step.Kind, KindUseBuiltin)
	}
	if step.InputLines != 3 || step.OutputLines != 2 {
		t.Errorf("input/output line counts wrong: %d → %d", step.InputLines, step.OutputLines)
	}
	if step.DroppedTotal != 1 {
		t.Errorf("DroppedTotal = %d, want 1", step.DroppedTotal)
	}
	if step.ModifiedTotal != 1 {
		t.Errorf("ModifiedTotal = %d, want 1", step.ModifiedTotal)
	}
	if !strings.Contains(step.Label, BuiltinMihomoToShadowrocket) {
		t.Errorf("label should reference builtin name: %q", step.Label)
	}
}

func TestApplyNewTransformsReported_UnknownBuiltinFallsThroughAsNoop(t *testing.T) {
	eng := NewEngine()
	in := []string{"DOMAIN,a.com\nDOMAIN,b.com"}
	transforms := []schema.Transform{
		{Type: "use", Use: "builtin:nonexistent-transformer", Target: targetAll},
	}
	out, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageRule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0] != in[0] {
		t.Fatalf("expected content passthrough for unknown builtin, got %q", out[0])
	}
	if len(steps) != 1 || steps[0].InputLines != steps[0].OutputLines {
		t.Fatalf("unexpected steps %+v", steps)
	}
}

// TestApplyNewTransforms_NonReported_NoOverhead asserts that the legacy
// API still returns identical content (no report leakage) and matches the
// reported variant byte-for-byte.
func TestApplyNewTransforms_NonReported_MatchesReported(t *testing.T) {
	eng := NewEngine()
	in := []string{"DOMAIN,example.com\nPROCESS-NAME,bad\nMATCH,DIRECT"}
	transforms := []schema.Transform{
		{Type: "use", Use: BuiltinMihomoToShadowrocket, Target: targetAll},
	}
	gotPlain, err := eng.ApplyNewTransforms(in, transforms, testCtx())
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	gotReported, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageRule)
	if err != nil {
		t.Fatalf("reported: %v", err)
	}
	if !equalStringSlice(gotPlain, gotReported) {
		t.Fatalf("plain vs reported content mismatch:\nplain   = %v\nreported= %v", gotPlain, gotReported)
	}
	if len(steps) == 0 {
		t.Fatal("reported variant returned zero steps")
	}
}

func TestApplyNewTransformsReported_PerSourceTracking(t *testing.T) {
	eng := NewEngine()
	in := []string{"DOMAIN,a.com", "DOMAIN,b.com\nPROCESS-NAME,bad"}
	transforms := []schema.Transform{
		{Type: "use", Use: BuiltinMihomoToShadowrocket, Target: targetAll},
	}
	_, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageRule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected one step per source (2), got %d", len(steps))
	}
	for i, s := range steps {
		if s.SourceIndex != i {
			t.Errorf("step %d SourceIndex = %d", i, s.SourceIndex)
		}
	}
}

func TestApplyNewTransformsReported_TargetSubsetSkipsUntargetedSources(t *testing.T) {
	eng := NewEngine()
	in := []string{"DOMAIN,a.com", "DOMAIN,b.com\nPROCESS-NAME,bad"}
	transforms := []schema.Transform{
		{Type: "use", Use: BuiltinMihomoToShadowrocket, Target: targetIndices(1)},
	}
	out, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageRule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0] != in[0] {
		t.Errorf("source 0 should be untouched, got %q", out[0])
	}
	if out[1] == in[1] {
		t.Errorf("source 1 should have been rewritten")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step (only targeted source), got %d", len(steps))
	}
	if steps[0].SourceIndex != 1 {
		t.Errorf("step SourceIndex = %d, want 1", steps[0].SourceIndex)
	}
}

// TestApplyNewTransformsReported_ReplaceAndRemoveLinesReported makes sure the
// non-builtin branches also emit a step report with sane labels.
func TestApplyNewTransformsReported_ReplaceAndRemoveLinesReported(t *testing.T) {
	eng := NewEngine()
	in := []string{"DOMAIN,a.com\nDOMAIN,b.com"}
	transforms := []schema.Transform{
		{Type: "replace", Pattern: "DOMAIN", Replacement: "DOMAIN-SUFFIX", Flags: "g", Target: targetAll},
		{Type: "remove_lines", Pattern: "b\\.com", Target: targetAll},
	}
	out, steps, err := eng.ApplyNewTransformsReported(in, transforms, testCtx(), StageClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out[0], "DOMAIN-SUFFIX,a.com") {
		t.Errorf("expected replace to take effect, got %q", out[0])
	}
	if strings.Contains(out[0], "b.com") {
		t.Errorf("expected b.com to be removed, got %q", out[0])
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Kind != KindReplace || steps[1].Kind != KindRemoveLines {
		t.Errorf("unexpected kinds: %q / %q", steps[0].Kind, steps[1].Kind)
	}
	for _, s := range steps {
		if s.Stage != StageClient {
			t.Errorf("expected stage=client, got %q", s.Stage)
		}
	}
}
