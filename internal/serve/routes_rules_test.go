package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
)

func TestRulePreviewCompilesDraftWithLocalFile(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	if err := os.MkdirAll(filepath.Join(s.DataDir, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.DataDir, "local", "custom.list"), []byte("DOMAIN,example.com\nIP-CIDR,1.1.1.1/32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	added := patchConfig(h, `{"version":1,"ops":[{"op":"add_client","value":{"id":"mihomo","name":"Mihomo","template":"mihomo-classical"}},{"op":"add_client","value":{"id":"sing-box","name":"sing-box","template":"singbox"}}]}`)
	if added.Code != 200 {
		t.Fatalf("clients: %d %s", added.Code, added.Body.String())
	}

	rec := localFileJSON(h, http.MethodPost, "/api/v1/rules/preview", `{
		"rule": {
			"id": "custom",
			"name": "Custom",
			"sources": [{"file": "custom.list", "label": "local"}],
			"ops": [{"type": "include_kinds", "kinds": ["domain"]}],
			"outputs": ["mihomo", "sing-box"]
		}
	}`)
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var report rulePreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.RuleID != "custom" || report.Merged != 1 || report.PreOps != 2 || report.PostOps != 1 {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Sources) != 1 || report.Sources[0].Label != "local" || report.Sources[0].Entries != 2 || report.Sources[0].DurationMs < 0 {
		t.Fatalf("sources=%+v", report.Sources)
	}
	if report.OpsDiff.Removed != 1 {
		t.Fatalf("ops_diff=%+v", report.OpsDiff)
	}
	if len(report.Outputs) != 2 {
		t.Fatalf("outputs=%+v", report.Outputs)
	}
	byID := map[string]rulePreviewArtifact{}
	for _, item := range report.Outputs {
		byID[item.ID] = item
		if item.ClientID != item.ID || item.ClientName == "" {
			t.Fatalf("missing client hierarchy: %+v", item)
		}
	}
	if !strings.Contains(byID["mihomo"].Output, "DOMAIN,example.com") || strings.Contains(byID["mihomo"].Output, "IP-CIDR") {
		t.Fatalf("mihomo=%q", byID["mihomo"].Output)
	}
	if !strings.Contains(byID["sing-box"].Output, `"example.com"`) {
		t.Fatalf("sing-box=%q", byID["sing-box"].Output)
	}
}

func TestRulePreviewRejectsCyclesAndMissingRule(t *testing.T) {
	s, _, _ := testServer(t)
	h := s.Handler()
	cycle := localFileJSON(h, http.MethodPost, "/api/v1/rules/preview", `{
		"rule": {"id":"apple","name":"Apple","sources":[{"ref":"child"}],"outputs":["surge"]}
	}`)
	if cycle.Code != http.StatusUnprocessableEntity || !strings.Contains(cycle.Body.String(), "circular dependency") {
		t.Fatalf("cycle: %d %s", cycle.Code, cycle.Body.String())
	}

	missing := localFileJSON(h, http.MethodPost, "/api/v1/rules/preview", `{"rule":{"id":"","name":"X","sources":[{"content":"DOMAIN,x.example"}],"outputs":["surge"]}}`)
	if missing.Code != http.StatusUnprocessableEntity || !strings.Contains(missing.Body.String(), "rule.id") {
		t.Fatalf("missing id: %d %s", missing.Code, missing.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/preview", strings.NewReader(`{"rule":{"id":"apple"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRulePreviewSourceNames(t *testing.T) {
	cfg := &config.Config{Rules: []config.RuleConfig{{ID: "example", Sources: []config.SourceConfig{
		{URL: "https://example.com/rules.list"},
		{File: "custom.list", Label: "自定义"},
		{Group: []config.SourceConfig{{Geosite: "v2fly/google@cn"}, {Ref: "base"}}, Label: "Google"},
	}}}}
	report := &engine.PreviewReport{RuleID: "example", Sources: []engine.SourceOutcome{
		{Label: "source[0]", Type: "url"}, {Label: "自定义", Type: "local"}, {Label: "Google", Type: "group"},
	}}
	response := buildRulePreviewResponse(cfg, report, 0, render.NewRegistry())
	if response.Sources[0].Label != "https://example.com/rules.list" {
		t.Fatalf("url source=%+v", response.Sources[0])
	}
	if response.Sources[1].Label != "自定义" || len(response.Sources[1].Details) != 1 || response.Sources[1].Details[0] != "custom.list" {
		t.Fatalf("file source=%+v", response.Sources[1])
	}
	group := response.Sources[2]
	if group.Label != "Google" || len(group.Details) != 2 || group.Details[0] != "v2fly/google@cn" || group.Details[1] != "base" {
		t.Fatalf("group=%+v", group)
	}
}
