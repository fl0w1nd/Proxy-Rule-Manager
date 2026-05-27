package transformer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func newTransform(t *testing.T, payload string) schema.Transform {
	t.Helper()
	var out schema.Transform
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestMergeContents_Concat(t *testing.T) {
	got := MergeContents([]string{"line1\nline2", "line3\nline4"}, "concat", false)
	if got != "line1\nline2\nline3\nline4" {
		t.Fatalf("unexpected %q", got)
	}
	if got := MergeContents([]string{"line1\nline2", "line2\nline3"}, "concat", true); got != "line1\nline2\nline3" {
		t.Fatalf("dedupe failed: %q", got)
	}
	if got := MergeContents([]string{}, "concat", false); got != "" {
		t.Fatalf("expected empty got %q", got)
	}
}

func TestMergeContents_Union(t *testing.T) {
	got := MergeContents([]string{"a\nb\nc", "b\nc\nd"}, "union", false)
	parts := strings.Split(got, "\n")
	if len(parts) != 4 {
		t.Fatalf("expected 4 lines, got %v", parts)
	}
	for _, want := range []string{"a", "b", "c", "d"} {
		found := false
		for _, p := range parts {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %q in %v", want, parts)
		}
	}
	got = MergeContents([]string{"a\n\nb", "b\n\nc"}, "union", false)
	for _, p := range strings.Split(got, "\n") {
		if p == "" {
			t.Fatalf("unexpected empty line in %q", got)
		}
	}
}

func TestMergeContents_Intersect(t *testing.T) {
	got := MergeContents([]string{"a\nb\nc", "b\nc\nd"}, "intersect", false)
	parts := strings.Split(got, "\n")
	hasA := false
	hasB := false
	hasD := false
	for _, p := range parts {
		if p == "a" {
			hasA = true
		}
		if p == "b" {
			hasB = true
		}
		if p == "d" {
			hasD = true
		}
	}
	if hasA || hasD || !hasB {
		t.Fatalf("intersect off: %v", parts)
	}
	if got := MergeContents([]string{"a\nb\nc"}, "intersect", false); got != "a\nb\nc" {
		t.Fatalf("single intersect: %q", got)
	}
	if got := MergeContents([]string{"a\nb", "c\nd"}, "intersect", false); got != "" {
		t.Fatalf("expected empty intersect, got %q", got)
	}
}

func TestApplyNewTransforms_Replace(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"replace","target":"all","pattern":"old\\.com","replacement":"new.com"}`),
	}
	got, err := engine.ApplyNewTransforms([]string{"DOMAIN,old.com", "DOMAIN,other.com"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got[0] != "DOMAIN,new.com" || got[1] != "DOMAIN,other.com" {
		t.Fatalf("replace result: %v", got)
	}
}

func TestApplyNewTransforms_ReplaceTargets(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"replace","target":[0,2],"pattern":"line","replacement":"row"}`),
	}
	got, err := engine.ApplyNewTransforms([]string{"line1", "line2", "line3"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("replace targets: %v", err)
	}
	if got[0] != "row1" || got[1] != "line2" || got[2] != "row3" {
		t.Fatalf("replace targets: %v", got)
	}
}

func TestApplyNewTransforms_RemoveLines(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"remove_lines","target":"all","pattern":"^#"}`),
	}
	got, err := engine.ApplyNewTransforms([]string{"# comment\nDOMAIN,test.com\n# another"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("remove_lines: %v", err)
	}
	if got[0] != "DOMAIN,test.com" {
		t.Fatalf("remove_lines result: %q", got[0])
	}
}

func TestApplyNewTransforms_UseScript(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"use","target":"all","use":"uppercase"}`),
	}
	transformers := map[string]schema.ScriptTransformer{
		"uppercase": {Name: "uppercase", Script: "function transform(content) { return content.toUpperCase(); }"},
	}
	got, err := engine.ApplyNewTransforms([]string{"test content"}, transforms, PipelineCtx{Transformers: transformers})
	if err != nil {
		t.Fatalf("use script: %v", err)
	}
	if got[0] != "TEST CONTENT" {
		t.Fatalf("uppercase: %q", got[0])
	}
	// Missing transformer keeps input intact.
	transforms = []schema.Transform{newTransform(t, `{"type":"use","target":"all","use":"missing"}`)}
	got, err = engine.ApplyNewTransforms([]string{"test"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("missing transformer: %v", err)
	}
	if got[0] != "test" {
		t.Fatalf("missing transformer should noop: %q", got[0])
	}
}

func TestApplyNewTransforms_Chain(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"remove_lines","target":"all","pattern":"^#"}`),
		newTransform(t, `{"type":"replace","target":"all","pattern":"old","replacement":"new"}`),
	}
	got, err := engine.ApplyNewTransforms([]string{"# comment\nDOMAIN,old.com"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if got[0] != "DOMAIN,new.com" {
		t.Fatalf("chain result: %q", got[0])
	}
}

func TestApplyNewTransforms_InvalidRegex(t *testing.T) {
	engine := NewEngine()
	transforms := []schema.Transform{
		newTransform(t, `{"type":"replace","target":"all","pattern":"(","replacement":"x"}`),
	}
	// TS silently returns original content on invalid regex; Go mirrors this.
	got, err := engine.ApplyNewTransforms([]string{"test"}, transforms, PipelineCtx{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] != "test" {
		t.Fatalf("expected original content, got %q", got[0])
	}
}

func TestStripManagedRuleHeader(t *testing.T) {
	// Simulate a header produced by an older release. Production no longer
	// emits the managed header; this test only verifies that resyncs after
	// upgrade still strip it before content comparison.
	legacy := "# 规则数量：1 条\n# 更新时间：2026-04-15 10:30:45\n# 规则类型：\n# DOMAIN: 1 条\n\n# upstream\nDOMAIN,test.com\n"
	want := "# upstream\nDOMAIN,test.com\n"
	if got := StripManagedRuleHeader(legacy); got != want {
		t.Fatalf("strip lost upstream: %q vs %q", got, want)
	}
}

// TestStripManagedRuleHeader_PreservesCRLFBody guards the byte-precise
// strip behaviour: when the legacy header is found, only the header is
// dropped and the body's CRLF line endings are returned verbatim. This
// matters for the legacy-migration sync — the first resync rewrites the
// file with whatever line endings the freshly produced content has, and
// from that point on byte-equal comparisons must hold.
func TestStripManagedRuleHeader_PreservesCRLFBody(t *testing.T) {
	legacy := "# 规则数量：1 条\n# 更新时间：2026-04-15 10:30:45\n# 规则类型：\n# DOMAIN: 1 条\n\nDOMAIN,a.com\r\nDOMAIN,b.com\r\n"
	want := "DOMAIN,a.com\r\nDOMAIN,b.com\r\n"
	if got := StripManagedRuleHeader(legacy); got != want {
		t.Fatalf("expected CRLF body to round-trip verbatim, got %q want %q", got, want)
	}
}

// TestStripManagedRuleHeader_NoHeaderReturnsVerbatim is the regression
// guard for the CRLF idempotency bug: when no legacy header is present,
// the function must return content unchanged (no LF normalisation) so
// the byte-equality comparison in flushArtifact stays honest for CRLF
// artifacts.
func TestStripManagedRuleHeader_NoHeaderReturnsVerbatim(t *testing.T) {
	cases := []string{
		"DOMAIN,a.com\r\nDOMAIN,b.com\r\n",       // pure CRLF body, no header
		"# upstream comment\r\nDOMAIN,a.com\r\n", // body that happens to start with `#`
		"DOMAIN,a.com\nDOMAIN,b.com\n",           // pure LF, idempotent
		"# 规则数量：not a header\nDOMAIN,a.com\n",    // shape mismatch (line 2 prefix wrong)
	}
	for _, in := range cases {
		if got := StripManagedRuleHeader(in); got != in {
			t.Errorf("StripManagedRuleHeader(%q) should be verbatim, got %q", in, got)
		}
	}
}

func TestNormalizeEffectiveRuleContent(t *testing.T) {
	content := "# header\n\nDOMAIN,test.com\n   \n# footer\nIP-CIDR,1.1.1.1/32\n"
	if got := NormalizeEffectiveRuleContent(content); got != "DOMAIN,test.com\nIP-CIDR,1.1.1.1/32" {
		t.Fatalf("normalize: %q", got)
	}
	if got := NormalizeEffectiveRuleContent("# header\n\n   \n# footer"); got != "" {
		t.Fatalf("expected empty: %q", got)
	}
}
