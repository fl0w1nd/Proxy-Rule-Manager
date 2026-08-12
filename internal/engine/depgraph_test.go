package engine

import (
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
)

func TestTopologicalSortSimple(t *testing.T) {
	rules := []config.RuleConfig{
		{ID: "B", Name: "B", Sources: []config.SourceConfig{{Type: "ref", Ref: "A"}}},
		{ID: "A", Name: "A", Sources: []config.SourceConfig{{URL: "https://example.com"}}},
	}
	sorted, err := TopologicalSort(rules, false)
	if err != nil {
		t.Fatal(err)
	}
	if sorted[0].ID != "A" {
		t.Errorf("first = %q, want A", sorted[0].ID)
	}
	if sorted[1].ID != "B" {
		t.Errorf("second = %q, want B", sorted[1].ID)
	}
}

func TestCircularDependency(t *testing.T) {
	rules := []config.RuleConfig{
		{ID: "A", Name: "A", Sources: []config.SourceConfig{{Type: "ref", Ref: "B"}}},
		{ID: "B", Name: "B", Sources: []config.SourceConfig{{Type: "ref", Ref: "A"}}},
	}
	cycle := DetectCircularDependency(rules)
	if cycle == nil {
		t.Error("expected cycle to be detected")
	}
}

func TestCollectAffectedRules(t *testing.T) {
	rules := []config.RuleConfig{
		{ID: "A", Name: "A", Sources: []config.SourceConfig{{URL: "https://example.com"}}},
		{ID: "B", Name: "B", Sources: []config.SourceConfig{{Type: "ref", Ref: "A"}}},
		{ID: "C", Name: "C", Sources: []config.SourceConfig{{Type: "ref", Ref: "B"}}},
		{ID: "D", Name: "D", Sources: []config.SourceConfig{{URL: "https://example.com"}}},
	}
	affected := CollectAffectedRules(rules, []string{"A"})
	if _, ok := affected["A"]; !ok {
		t.Error("A should be affected")
	}
	if _, ok := affected["B"]; !ok {
		t.Error("B should be affected (depends on A)")
	}
	if _, ok := affected["C"]; !ok {
		t.Error("C should be affected (depends on B)")
	}
	if _, ok := affected["D"]; ok {
		t.Error("D should not be affected")
	}
}

func TestTopologicalSortDiamondDependencyOrder(t *testing.T) {
	rules := []config.RuleConfig{
		{ID: "D", Name: "D", Sources: []config.SourceConfig{{Ref: "B"}, {Ref: "C"}}},
		{ID: "C", Name: "C", Sources: []config.SourceConfig{{Ref: "A"}}},
		{ID: "B", Name: "B", Sources: []config.SourceConfig{{Ref: "A"}}},
		{ID: "A", Name: "A", Sources: []config.SourceConfig{{Content: "a.example"}}},
	}
	sorted, err := TopologicalSort(rules, false)
	if err != nil {
		t.Fatal(err)
	}
	position := make(map[string]int, len(sorted))
	for i, rule := range sorted {
		position[rule.ID] = i
	}
	for _, edge := range [][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}} {
		if position[edge[0]] >= position[edge[1]] {
			t.Fatalf("dependency order %s->%s violated: %+v", edge[0], edge[1], sorted)
		}
	}
}
