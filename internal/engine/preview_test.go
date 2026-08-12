package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
)

func TestPreviewResolvesRuleReferences(t *testing.T) {
	cfg := &config.Config{
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "base", Name: "Base Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,base.example"}}},
			{ID: "derived", Name: "Derived Rule", Sources: []config.SourceConfig{{Ref: "base"}, {Content: "DOMAIN,derived.example"}}},
		},
	}
	report, err := Preview(
		context.Background(), cfg, "derived", "", testRegistry(t), NewFetcher(),
		NewPreprocessRunner(), nil, testLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Sources) != 2 || report.Sources[0].Error != "" || len(report.Merged) != 2 {
		t.Fatalf("preview report: %+v", report)
	}
	if report.RuleID != "derived" || report.RuleName != "Derived Rule" {
		t.Fatalf("preview identity: %+v", report)
	}
}

func TestPreviewUsesExplicitOutputTarget(t *testing.T) {
	cfg := &config.Config{
		Clients: []config.ClientConfig{{
			ID: "mihomo",
			Formats: []config.ClientFormatConfig{
				{ID: "mihomo-classical", Name: "Classical", Template: "mihomo-classical"},
				{ID: "mihomo-yaml", Name: "YAML", Template: "mihomo-yaml"},
			},
		}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"mihomo"},
		}},
	}

	report, err := Preview(context.Background(), cfg, "rule", "mihomo-yaml", testRegistry(t), NewFetcher(), NewPreprocessRunner(), nil, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if report.RenderError != "" || report.RenderedTarget != "mihomo-yaml" || !strings.Contains(string(report.RenderedOutput), "payload:") {
		t.Fatalf("report=%+v output=%s", report, report.RenderedOutput)
	}

	report, err = Preview(context.Background(), cfg, "rule", "mihomo", testRegistry(t), NewFetcher(), NewPreprocessRunner(), nil, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if report.RenderError != "unknown output target: mihomo" {
		t.Fatalf("family id render error=%q", report.RenderError)
	}
}
