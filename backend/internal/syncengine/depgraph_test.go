package syncengine

import (
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func makeRule(name string, refs []string) schema.RuleConfig {
	var sources []schema.SourceConfig
	for _, ref := range refs {
		sources = append(sources, schema.SourceConfig{Type: "ref", Ref: ref})
	}
	return schema.RuleConfig{
		Name:    name,
		Sources: sources,
		Output:  schema.OutputConfig{Clients: []string{"clash_meta"}},
	}
}

func TestExtractDependencies(t *testing.T) {
	rule := makeRule("A", []string{"D", "E"})
	deps := ExtractDependencies(&rule)
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if _, ok := deps["D"]; !ok {
		t.Fatalf("expected D in deps")
	}
}

func TestDetectCircularDependency(t *testing.T) {
	rules := []schema.RuleConfig{
		makeRule("A", []string{"B"}),
		makeRule("B", []string{"A"}),
	}
	cycle := DetectCircularDependency(rules)
	if cycle == nil {
		t.Fatalf("expected cycle")
	}
	rules = []schema.RuleConfig{
		makeRule("A", []string{"B"}),
		makeRule("B", []string{"C"}),
		makeRule("C", nil),
	}
	if DetectCircularDependency(rules) != nil {
		t.Fatalf("expected no cycle for chain")
	}
	// Self-reference
	rules = []schema.RuleConfig{makeRule("A", []string{"A"})}
	if DetectCircularDependency(rules) == nil {
		t.Fatalf("expected self-reference to be a cycle")
	}
	// Non-existent reference ignored
	rules = []schema.RuleConfig{makeRule("A", []string{"NonExistent"})}
	if DetectCircularDependency(rules) != nil {
		t.Fatalf("expected no cycle when dep is external")
	}
}

func TestTopologicalSort(t *testing.T) {
	rules := []schema.RuleConfig{
		makeRule("A", []string{"B"}),
		makeRule("B", []string{"C"}),
		makeRule("C", nil),
	}
	sorted, err := TopologicalSort(rules, false)
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	names := make([]string, len(sorted))
	for i, r := range sorted {
		names[i] = r.Name
	}
	if indexOf(names, "C") > indexOf(names, "B") || indexOf(names, "B") > indexOf(names, "A") {
		t.Fatalf("order wrong: %v", names)
	}

	// Cycle
	cyclic := []schema.RuleConfig{makeRule("A", []string{"B"}), makeRule("B", []string{"A"})}
	if _, err := TopologicalSort(cyclic, false); err == nil || !strings.Contains(err.Error(), "循环依赖") {
		t.Fatalf("expected 循环依赖 error, got %v", err)
	}

	// Missing dep without skip
	missing := []schema.RuleConfig{makeRule("A", []string{"X"})}
	if _, err := TopologicalSort(missing, false); err == nil || !strings.Contains(err.Error(), "依赖缺失") {
		t.Fatalf("expected 依赖缺失 error, got %v", err)
	}
	if _, err := TopologicalSort(missing, true); err != nil {
		t.Fatalf("skip should suppress missing dep error: %v", err)
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// TestTopologicalSort_NoDepsOrder verifies that independent (dep-free) rules
// are always emitted in the same order as the input slice regardless of map
// iteration randomness. Run many times to surface any non-determinism.
func TestTopologicalSort_NoDepsOrder(t *testing.T) {
	rules := []schema.RuleConfig{
		makeRule("alpha", nil),
		makeRule("beta", nil),
		makeRule("gamma", nil),
		makeRule("delta", nil),
		makeRule("epsilon", nil),
	}

	var refOrder []string
	for i := 0; i < 200; i++ {
		sorted, err := TopologicalSort(rules, false)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		names := make([]string, len(sorted))
		for j, r := range sorted {
			names[j] = r.Name
		}
		if i == 0 {
			refOrder = names
			continue
		}
		for j, n := range names {
			if n != refOrder[j] {
				t.Fatalf("iter %d: order changed at index %d: got %v, want %v", i, j, names, refOrder)
			}
		}
	}
}

// TestTopologicalSort_NoDepsOrder_WithDeps verifies determinism when some rules
// are initial dep-free and others unlock after their dependency is processed.
func TestTopologicalSort_NoDepsOrder_WithDeps(t *testing.T) {
	// A and C have no deps; B depends on A; D depends on C.
	rules := []schema.RuleConfig{
		makeRule("A", nil),
		makeRule("B", []string{"A"}),
		makeRule("C", nil),
		makeRule("D", []string{"C"}),
	}

	var refOrder []string
	for i := 0; i < 200; i++ {
		sorted, err := TopologicalSort(rules, false)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		names := make([]string, len(sorted))
		for j, r := range sorted {
			names[j] = r.Name
		}
		if i == 0 {
			refOrder = names
		} else {
			for j, n := range names {
				if n != refOrder[j] {
					t.Fatalf("iter %d: order changed at index %d: got %v, want %v", i, j, names, refOrder)
				}
			}
		}
	}
	// Also verify the topological constraints hold.
	sorted, _ := TopologicalSort(rules, false)
	names := make([]string, len(sorted))
	for i, r := range sorted {
		names[i] = r.Name
	}
	if indexOf(names, "A") > indexOf(names, "B") {
		t.Errorf("A must precede B, got order: %v", names)
	}
	if indexOf(names, "C") > indexOf(names, "D") {
		t.Errorf("C must precede D, got order: %v", names)
	}
}
