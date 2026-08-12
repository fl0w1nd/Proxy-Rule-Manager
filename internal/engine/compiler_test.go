package engine

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	defaulttemplates "github.com/fl0w1nd/proxy-rule-manager/templates"
	"gopkg.in/yaml.v3"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testRegistry(t *testing.T) *render.Registry {
	t.Helper()
	registry := render.NewRegistry()
	if err := registry.LoadEmbedded(defaulttemplates.FS); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	return registry
}

func TestCompileRuleFormatsOpsAndRendersCompletePipeline(t *testing.T) {
	rule := config.RuleConfig{
		ID:   "mixed",
		Name: "mixed",
		Sources: []config.SourceConfig{
			{Label: "classical", Content: strings.Join([]string{
				"DOMAIN-SUFFIX, Example.COM",
				"IP-CIDR,10.1.2.3/8,no-resolve",
				"DOMAIN-SUFFIX,ads.example.com",
			}, "\n")},
			{Label: "yaml", Content: "payload:\n  - DOMAIN-SUFFIX,example.com\n  - DOMAIN,Api.Example.COM\n"},
		},
		Ops:     []config.OpConfig{{Type: "filter_values", Mode: "keyword", Pattern: "ads"}},
		Merge:   &config.MergeConfig{Strategy: "union"},
		Outputs: []string{"mihomo", "singbox"},
	}
	clients := []config.ClientConfig{
		{ID: "mihomo", Template: "mihomo-yaml"},
		{ID: "singbox", Template: "singbox"},
	}

	got := CompileRule(
		context.Background(), rule, clients, NewFetcher(), NewPreprocessRunner(),
		testRegistry(t), nil, nil, testLogger(),
	)

	if got.OpsError != "" || len(got.RenderErrors) != 0 {
		t.Fatalf("compile errors: ops=%q render=%v", got.OpsError, got.RenderErrors)
	}
	wantEntries := []ir.Entry{
		{Kind: ir.KindDomainSuffix, Value: "example.com"},
		{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8", Flags: []string{ir.FlagNoResolve}},
		{Kind: ir.KindDomain, Value: "api.example.com"},
	}
	if !entriesEqual(got.Merged, wantEntries) {
		t.Fatalf("merged entries:\n got: %#v\nwant: %#v", got.Merged, wantEntries)
	}
	if len(got.PreOps) != 4 || len(got.PostOps) != 3 {
		t.Fatalf("stage counts: pre=%d post=%d", len(got.PreOps), len(got.PostOps))
	}

	var payload struct {
		Payload []string `yaml:"payload"`
	}
	if err := yaml.Unmarshal(got.Rendered["mihomo"], &payload); err != nil {
		t.Fatalf("mihomo output is invalid YAML: %v\n%s", err, got.Rendered["mihomo"])
	}
	wantPayload := []string{
		"DOMAIN-SUFFIX,example.com",
		"IP-CIDR,10.0.0.0/8,no-resolve",
		"DOMAIN,api.example.com",
	}
	if !stringsEqual(payload.Payload, wantPayload) {
		t.Fatalf("mihomo payload: got %v, want %v", payload.Payload, wantPayload)
	}

	var singboxDoc struct {
		Version int              `json:"version"`
		Rules   []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(got.Rendered["singbox"], &singboxDoc); err != nil {
		t.Fatalf("sing-box output is invalid JSON: %v", err)
	}
	if singboxDoc.Version != 3 || len(singboxDoc.Rules) != 1 {
		t.Fatalf("sing-box document: %+v", singboxDoc)
	}
	ruleObject := singboxDoc.Rules[0]
	if ruleObject["ip_cidr"] != "10.0.0.0/8" || ruleObject["domain"] == nil || ruleObject["domain_suffix"] != "example.com" {
		t.Fatalf("sing-box rule fields: %#v", ruleObject)
	}
}

func TestCompileRuleReportsDiagnosticsOpsAndRenderErrors(t *testing.T) {
	t.Run("parse diagnostic keeps valid entries visible", func(t *testing.T) {
		rule := config.RuleConfig{
			ID:      "diagnostic",
			Name:    "diagnostic",
			Sources: []config.SourceConfig{{Content: "DOMAIN,ok.example\nIP-CIDR,999.1.1.1/33"}},
		}
		got := CompileRule(context.Background(), rule, nil, NewFetcher(), NewPreprocessRunner(), testRegistry(t), nil, nil, testLogger())
		if len(got.Sources) != 1 || len(got.Sources[0].Diagnostics) != 1 {
			t.Fatalf("diagnostics: %+v", got.Sources)
		}
		if len(got.Merged) != 1 || got.Merged[0].Value != "ok.example" {
			t.Fatalf("valid entries: %+v", got.Merged)
		}
	})

	t.Run("operation error is part of the result", func(t *testing.T) {
		rule := config.RuleConfig{
			ID:      "bad-op",
			Name:    "bad-op",
			Sources: []config.SourceConfig{{Content: "example.com"}},
			Ops:     []config.OpConfig{{Type: "filter_values", Mode: "regex", Pattern: "(["}},
		}
		got := CompileRule(context.Background(), rule, nil, NewFetcher(), NewPreprocessRunner(), testRegistry(t), nil, nil, testLogger())
		if !strings.Contains(got.OpsError, "invalid filter regex") {
			t.Fatalf("ops error: %q", got.OpsError)
		}
	})

	t.Run("client errors stay isolated", func(t *testing.T) {
		rule := config.RuleConfig{
			ID:      "clients",
			Name:    "clients",
			Sources: []config.SourceConfig{{Content: "example.com"}},
			Outputs: []string{"missing-client", "bad-template", "valid"},
		}
		clients := []config.ClientConfig{
			{ID: "bad-template", Template: "missing-template"},
			{ID: "valid", Template: "surge"},
		}
		got := CompileRule(context.Background(), rule, clients, NewFetcher(), NewPreprocessRunner(), testRegistry(t), nil, nil, testLogger())
		if len(got.RenderErrors) != 2 || got.Rendered["valid"] == nil {
			t.Fatalf("render outcomes: errors=%v rendered=%v", got.RenderErrors, got.Rendered)
		}
	})
}

func TestCompileRuleReadsLocalFileSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.list")
	if err := os.WriteFile(path, []byte("payload:\n  - DOMAIN,example.com\n  - DOMAIN-SUFFIX,example.org\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := config.RuleConfig{
		ID:      "local-file",
		Name:    "local-file",
		Sources: []config.SourceConfig{{File: path}},
	}
	got := CompileRule(context.Background(), rule, nil, NewFetcher(), NewPreprocessRunner(), testRegistry(t), nil, nil, testLogger())
	if len(got.Sources) != 1 || got.Sources[0].Error != "" {
		t.Fatalf("source outcome: %+v", got.Sources)
	}
	if len(got.Merged) != 2 || got.Merged[0].Value != "example.com" || got.Merged[1].Value != "example.org" {
		t.Fatalf("merged entries: %+v", got.Merged)
	}
}

func TestCompileRuleFetchesSourcesConcurrentlyWithinLimit(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{}, 3)
	fetcher := NewFetcher()
	fetcher.Configure(time.Second, 1024, 2, 2, 0, time.Millisecond, "test")
	fetcher.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-release
		name := strings.TrimPrefix(req.URL.Path, "/")
		return response(http.StatusOK, "DOMAIN,"+name+".example"), nil
	})}
	rule := config.RuleConfig{
		ID:   "parallel",
		Name: "parallel",
		Sources: []config.SourceConfig{
			{URL: "https://one.example/one"},
			{URL: "https://two.example/two"},
			{URL: "https://three.example/three"},
		},
	}

	registry := testRegistry(t)
	done := make(chan CompileResult, 1)
	go func() {
		done <- CompileRule(
			context.Background(), rule, nil, fetcher, NewPreprocessRunner(),
			registry, nil, nil, testLogger(),
		)
	}()
	// globalLimit=2: exactly 2 of the 3 sources should start concurrently;
	// the third blocks until a slot is released. Wait for the first 2 before
	// releasing all so we can verify the concurrency cap holds.
	waitForStarts(t, started, 2)
	for i := 0; i < 3; i++ {
		release <- struct{}{}
	}
	result := <-done
	if len(result.Merged) != 3 || result.Merged[0].Value != "one.example" || result.Merged[2].Value != "three.example" {
		t.Fatalf("ordered merged entries: %+v", result.Merged)
	}
}

func entriesEqual(got, want []ir.Entry) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Key() != want[i].Key() {
			return false
		}
	}
	return true
}

func stringsEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
