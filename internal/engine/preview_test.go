package engine

import (
	"context"
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
