package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
)

// ExtractDependencies returns the set of rule IDs referenced by rule.
func ExtractDependencies(rule *config.RuleConfig) map[string]struct{} {
	deps := map[string]struct{}{}
	if rule == nil {
		return deps
	}
	for _, src := range rule.Sources {
		if src.SourceType() == "ref" && src.Ref != "" {
			deps[src.Ref] = struct{}{}
		}
	}
	return deps
}

// DetectCircularDependency returns the cycle path or nil if none.
func DetectCircularDependency(rules []config.RuleConfig) []string {
	return config.DetectCircularDependency(rules)
}

// TopologicalSort returns rules sorted so each rule appears after its deps.
func TopologicalSort(rules []config.RuleConfig, skipMissingDepsCheck bool) ([]config.RuleConfig, error) {
	if cycle := DetectCircularDependency(rules); cycle != nil {
		return nil, fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " → "))
	}
	ruleByID := make(map[string]int, len(rules))
	for i, r := range rules {
		ruleByID[r.ID] = i
	}

	inDegree := make([]int, len(rules))
	dependents := make([][]int, len(rules))
	var missing []string
	for i := range rules {
		deps := ExtractDependencies(&rules[i])
		for dep := range deps {
			j, ok := ruleByID[dep]
			if !ok {
				if !skipMissingDepsCheck {
					missing = append(missing, fmt.Sprintf("rule %q (%s) references unknown rule ID %q", rules[i].Name, rules[i].ID, dep))
				}
				continue
			}
			inDegree[i]++
			dependents[j] = append(dependents[j], i)
		}
	}
	if !skipMissingDepsCheck && len(missing) > 0 {
		return nil, fmt.Errorf("missing dependencies:\n%s", strings.Join(missing, "\n"))
	}
	for j := range dependents {
		if len(dependents[j]) > 1 {
			sort.Ints(dependents[j])
		}
	}

	queue := make([]int, 0, len(rules))
	for i := range rules {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	sorted := make([]config.RuleConfig, 0, len(rules))
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		sorted = append(sorted, rules[i])
		for _, j := range dependents[i] {
			inDegree[j]--
			if inDegree[j] == 0 {
				queue = append(queue, j)
			}
		}
	}
	return sorted, nil
}

// CollectAffectedRules returns the rule IDs transitively affected by seed IDs.
func CollectAffectedRules(rules []config.RuleConfig, seedIDs []string) map[string]struct{} {
	dependents := map[string][]string{}
	for _, r := range rules {
		for _, src := range r.Sources {
			if src.SourceType() == "ref" && src.Ref != "" {
				dependents[src.Ref] = append(dependents[src.Ref], r.ID)
			}
		}
	}
	affected := map[string]struct{}{}
	queue := make([]string, 0, len(seedIDs))
	for _, s := range seedIDs {
		affected[s] = struct{}{}
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dep := range dependents[current] {
			if _, ok := affected[dep]; ok {
				continue
			}
			affected[dep] = struct{}{}
			queue = append(queue, dep)
		}
	}
	return affected
}
