package syncengine

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
)

func newTempProcessor(t *testing.T) (*Processor, *store.Store) {
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
	mgr := geosite.NewManager(paths.GeositeDir)
	p := &Processor{
		Store:       st,
		Fetcher:     NewFetcher(),
		Transformer: transformer.NewEngine(),
		Geosite:     mgr,
	}
	return p, st
}

func clashOutput() schema.OutputConfig {
	return schema.OutputConfig{Clients: []string{"clash_meta"}}
}

func strPtr(s string) *string { return &s }

func mustTarget(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	return json.RawMessage(raw)
}

// TestProcessor_LocalMissingRefSurfacesError reproduces the D-3 fix: a
// local source with a contentRef that doesn't resolve must yield a
// rule-level error mentioning the source so the operator can act on it,
// instead of the generic "no sources fetched successfully" cascade.
func TestProcessor_LocalMissingRefSurfacesError(t *testing.T) {
	p, _ := newTempProcessor(t)
	ref := "missing"
	rule := schema.RuleConfig{
		Name:   "local_missing",
		Output: clashOutput(),
		Sources: []schema.SourceConfig{
			{Type: "local", ContentRef: ref},
		},
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if len(res.Errors) == 0 {
		t.Fatalf("expected a local-source error, got none")
	}
	joined := strings.Join(res.Errors, "|")
	if !strings.Contains(joined, "Local source") || !strings.Contains(joined, ref) {
		t.Fatalf("error should reference 'Local source' and ref %q, got %v", ref, res.Errors)
	}
}

// TestProcessor_LocalEmptySourceSurfacesError covers the second case the
// D-3 fix added: a local source with neither inline content nor a
// contentRef must fail visibly instead of silently producing nothing.
func TestProcessor_LocalEmptySourceSurfacesError(t *testing.T) {
	p, _ := newTempProcessor(t)
	rule := schema.RuleConfig{
		Name:    "local_empty",
		Output:  clashOutput(),
		Sources: []schema.SourceConfig{{Type: "local"}},
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if len(res.Errors) == 0 {
		t.Fatalf("expected an empty-source error, got none")
	}
}

func TestProcessor_NoSources(t *testing.T) {
	p, _ := newTempProcessor(t)
	rule := schema.RuleConfig{Name: "r", Output: clashOutput()}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0], "no sources") {
		t.Fatalf("expected 'Rule has no sources' error, got %v", res.Errors)
	}
}

func TestProcessor_LocalInline(t *testing.T) {
	p, _ := newTempProcessor(t)
	rule := schema.RuleConfig{
		Name:    "r",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtr("DOMAIN,local.com")}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if res.Contents["clash_meta"] != "DOMAIN,local.com" {
		t.Fatalf("inline local: %q", res.Contents["clash_meta"])
	}
}

func TestProcessor_LocalContentRef(t *testing.T) {
	p, st := newTempProcessor(t)
	ref, err := st.WriteLocalSource(context.Background(), "my_ref", "DOMAIN,ref.com")
	if err != nil {
		t.Fatalf("WriteLocalSource: %v", err)
	}
	rule := schema.RuleConfig{
		Name:    "r",
		Sources: []schema.SourceConfig{{Type: "local", ContentRef: ref}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if res.Contents["clash_meta"] != "DOMAIN,ref.com" {
		t.Fatalf("local contentRef: %q", res.Contents["clash_meta"])
	}
}

func TestProcessor_RuleTransforms(t *testing.T) {
	p, _ := newTempProcessor(t)
	rule := schema.RuleConfig{
		Name:    "r",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtr("DOMAIN,old.com")}},
		Transforms: []schema.Transform{{
			Type:        "replace",
			Target:      mustTarget(t, `"all"`),
			Pattern:     "old",
			Replacement: "new",
		}},
		Output: clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	if res.Contents["clash_meta"] != "DOMAIN,new.com" {
		t.Fatalf("transform: %q", res.Contents["clash_meta"])
	}
}

func TestProcessor_MergeStrategies(t *testing.T) {
	p, _ := newTempProcessor(t)
	cases := []struct {
		name    string
		merge   *schema.MergeConfig
		sources []string
		want    string
	}{
		{
			name:    "default concat",
			sources: []string{"DOMAIN,a.com", "DOMAIN,b.com"},
			want:    "DOMAIN,a.com\nDOMAIN,b.com",
		},
		{
			name:    "union dedupes",
			merge:   &schema.MergeConfig{Strategy: "union", Dedupe: true},
			sources: []string{"DOMAIN,a.com\nDOMAIN,b.com", "DOMAIN,b.com\nDOMAIN,c.com"},
			want:    "DOMAIN,a.com\nDOMAIN,b.com\nDOMAIN,c.com",
		},
		{
			name:    "intersect keeps common",
			merge:   &schema.MergeConfig{Strategy: "intersect"},
			sources: []string{"DOMAIN,a.com\nDOMAIN,b.com", "DOMAIN,b.com\nDOMAIN,c.com"},
			want:    "DOMAIN,b.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcs := make([]schema.SourceConfig, 0, len(tc.sources))
			for _, s := range tc.sources {
				srcs = append(srcs, schema.SourceConfig{Type: "local", Content: strPtr(s)})
			}
			rule := schema.RuleConfig{Name: "r", Sources: srcs, Merge: tc.merge, Output: clashOutput()}
			res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
			if got := res.Contents["clash_meta"]; got != tc.want {
				t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestProcessor_RefFromCache(t *testing.T) {
	p, _ := newTempProcessor(t)
	cache := NewRuleContentsCache()
	cache.Set("base", map[string]string{"clash_meta": "DOMAIN,base.com"}, []string{"clash_meta"})
	rule := schema.RuleConfig{
		Name:    "downstream",
		Sources: []schema.SourceConfig{{Type: "ref", Ref: "base"}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, cache, schema.DefaultClients)
	if res.Contents["clash_meta"] != "DOMAIN,base.com" {
		t.Fatalf("ref: %q", res.Contents["clash_meta"])
	}
}

func TestProcessor_RefFallbackToAnyClient(t *testing.T) {
	p, _ := newTempProcessor(t)
	cache := NewRuleContentsCache()
	cache.Set("base", map[string]string{"shadowrocket": "DOMAIN,sr.com"}, []string{"shadowrocket"})
	rule := schema.RuleConfig{
		Name:    "downstream",
		Sources: []schema.SourceConfig{{Type: "ref", Ref: "base"}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, cache, schema.DefaultClients)
	if res.Contents["clash_meta"] != "DOMAIN,sr.com" {
		t.Fatalf("ref fallback: %q", res.Contents["clash_meta"])
	}
}

func TestProcessor_RefFallbackUsesCachedOrder(t *testing.T) {
	p, _ := newTempProcessor(t)
	cache := NewRuleContentsCache()
	cache.Set("base", map[string]string{
		"shadowrocket": "DOMAIN,sr.com",
		"clash_meta":   "DOMAIN,clash.com",
	}, []string{"shadowrocket", "clash_meta"})
	rule := schema.RuleConfig{
		Name:    "downstream",
		Sources: []schema.SourceConfig{{Type: "ref", Ref: "base"}},
		Output:  schema.OutputConfig{Clients: []string{"singbox"}},
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, cache, schema.DefaultClients)
	if res.Contents["singbox"] != "DOMAIN,sr.com" {
		t.Fatalf("ref fallback order: %q", res.Contents["singbox"])
	}
}

func TestProcessor_RefMissingErrors(t *testing.T) {
	p, _ := newTempProcessor(t)
	rule := schema.RuleConfig{
		Name:    "downstream",
		Sources: []schema.SourceConfig{{Type: "ref", Ref: "missing"}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), schema.DefaultClients)
	joined := strings.Join(res.Errors, "|")
	if !strings.Contains(joined, "not found in cache") {
		t.Fatalf("expected 'not found in cache' error, got %v", res.Errors)
	}
}

// TestProcessor_RefSkipsDestructiveClientTransform reproduces the
// regression where a pure-ref rule fed every line of an already-
// transformed sing-box JSON document back into the
// builtin:mihomo-classical-to-singbox-source converter and lost
// everything as "未在映射表中配置".
//
// The contract being asserted is: refs consume the upstream rule's
// PreClientContents (post-merge, pre-client-transform), so a downstream
// rule that targets the same destructive client only runs the
// classical→JSON conversion once — on the merged classical content —
// instead of attempting to re-parse the JSON output as classical lines.
func TestProcessor_RefSkipsDestructiveClientTransform(t *testing.T) {
	p, _ := newTempProcessor(t)
	clients := []schema.ClientConfig{{
		ID:          "singbox",
		DisplayName: "Sing Box",
		Transforms: []schema.Transform{{
			Type:   "use",
			Target: mustTarget(t, `"all"`),
			Use:    "builtin:mihomo-classical-to-singbox-source",
		}},
	}}
	output := schema.OutputConfig{Clients: []string{"singbox"}}

	netflix := schema.RuleConfig{
		Name:    "Netflix",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtr("DOMAIN-SUFFIX,netflix.com\nIP-CIDR,8.41.4.0/24,no-resolve")}},
		Output:  output,
	}
	youtube := schema.RuleConfig{
		Name:    "YouTube",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtr("DOMAIN-SUFFIX,youtube.com")}},
		Output:  output,
	}

	cache := NewRuleContentsCache()

	// Upstream rules run first; their results get cached. The cache MUST
	// hold the pre-client-transform classical content for refs to work.
	for _, rule := range []schema.RuleConfig{netflix, youtube} {
		r := rule
		res := p.ProcessRule(context.Background(), &r, nil, nil, cache, clients)
		if len(res.Errors) > 0 {
			t.Fatalf("%s: unexpected errors %v", r.Name, res.Errors)
		}
		if !strings.Contains(res.Contents["singbox"], `"version"`) {
			t.Fatalf("%s: expected sing-box JSON output, got %q", r.Name, res.Contents["singbox"])
		}
		cache.Set(r.Name, res.PreClientContents, res.ClientOrder)
	}

	proxyMedia := schema.RuleConfig{
		Name: "ProxyMedia",
		Sources: []schema.SourceConfig{
			{Type: "ref", Ref: "Netflix"},
			{Type: "ref", Ref: "YouTube"},
		},
		Output: output,
	}
	res := p.ProcessRule(context.Background(), &proxyMedia, nil, nil, cache, clients)
	if len(res.Errors) > 0 {
		t.Fatalf("ProxyMedia: unexpected errors %v", res.Errors)
	}
	got := res.Contents["singbox"]
	if !strings.Contains(got, `"domain_suffix"`) {
		t.Fatalf("ProxyMedia: domain_suffix bucket missing — refs likely fed JSON back into the singbox converter.\ngot:\n%s", got)
	}
	for _, want := range []string{"netflix.com", "youtube.com", "8.41.4.0/24"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ProxyMedia: missing %q in merged sing-box output:\n%s", want, got)
		}
	}
}

func TestProcessor_ClientGlobalTransformsApplied(t *testing.T) {
	p, _ := newTempProcessor(t)
	clients := []schema.ClientConfig{{
		ID:          "clash_meta",
		DisplayName: "Clash Meta",
		Transforms: []schema.Transform{{
			Type:        "replace",
			Target:      mustTarget(t, `"all"`),
			Pattern:     "test",
			Replacement: "client",
		}},
	}}
	rule := schema.RuleConfig{
		Name:    "r",
		Sources: []schema.SourceConfig{{Type: "local", Content: strPtr("DOMAIN,test.com")}},
		Output:  clashOutput(),
	}
	res := p.ProcessRule(context.Background(), &rule, nil, nil, NewRuleContentsCache(), clients)
	if res.Contents["clash_meta"] != "DOMAIN,client.com" {
		t.Fatalf("client-global transform: %q", res.Contents["clash_meta"])
	}
}
