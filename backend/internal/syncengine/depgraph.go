package syncengine

import (
	"fmt"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// ExtractDependencies returns the set of rule names referenced by `rule`.
func ExtractDependencies(rule *schema.RuleConfig) map[string]struct{} {
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
func DetectCircularDependency(rules []schema.RuleConfig) []string {
	ruleMap := make(map[string]*schema.RuleConfig, len(rules))
	deps := make(map[string][]string, len(rules))
	for i := range rules {
		ruleMap[rules[i].Name] = &rules[i]
		set := ExtractDependencies(&rules[i])
		for k := range set {
			deps[rules[i].Name] = append(deps[rules[i].Name], k)
		}
	}

	visited := map[string]bool{}
	inStack := map[string]bool{}
	var path []string

	var dfs func(string) []string
	dfs = func(node string) []string {
		if inStack[node] {
			idx := -1
			for i, n := range path {
				if n == node {
					idx = i
					break
				}
			}
			cycle := append([]string(nil), path[idx:]...)
			cycle = append(cycle, node)
			return cycle
		}
		if visited[node] {
			return nil
		}
		visited[node] = true
		inStack[node] = true
		path = append(path, node)
		for _, dep := range deps[node] {
			if _, ok := ruleMap[dep]; !ok {
				continue
			}
			if c := dfs(dep); c != nil {
				return c
			}
		}
		path = path[:len(path)-1]
		inStack[node] = false
		return nil
	}

	for i := range rules {
		visited = map[string]bool{}
		inStack = map[string]bool{}
		path = path[:0]
		if cycle := dfs(rules[i].Name); cycle != nil {
			return cycle
		}
	}
	return nil
}

// TopologicalSort returns rules sorted so each rule appears after its deps.
// When skipMissingDepsCheck=false, an error is returned listing missing deps.
func TopologicalSort(rules []schema.RuleConfig, skipMissingDepsCheck bool) ([]schema.RuleConfig, error) {
	if cycle := DetectCircularDependency(rules); cycle != nil {
		return nil, fmt.Errorf("检测到循环依赖: %s", strings.Join(cycle, " → "))
	}
	ruleByName := make(map[string]int, len(rules))
	for i, r := range rules {
		ruleByName[r.Name] = i
	}
	dependencies := make(map[string]map[string]struct{}, len(rules))
	var missing []string
	for i := range rules {
		all := ExtractDependencies(&rules[i])
		in := map[string]struct{}{}
		for d := range all {
			if _, ok := ruleByName[d]; ok {
				in[d] = struct{}{}
			} else if !skipMissingDepsCheck {
				missing = append(missing, fmt.Sprintf(`规则 "%s" 引用了不存在的规则 "%s"`, rules[i].Name, d))
			}
		}
		dependencies[rules[i].Name] = in
	}
	if !skipMissingDepsCheck && len(missing) > 0 {
		return nil, fmt.Errorf("依赖缺失:\n%s", strings.Join(missing, "\n"))
	}

	// Build noDeps by iterating the original rules slice (not the map) so
	// that the initial queue order is deterministic (matches the TS Map
	// insertion-order semantics).
	var noDeps []string
	for _, rule := range rules {
		if set, ok := dependencies[rule.Name]; ok && len(set) == 0 {
			noDeps = append(noDeps, rule.Name)
		}
	}

	sorted := make([]schema.RuleConfig, 0, len(rules))
	for len(noDeps) > 0 {
		name := noDeps[0]
		noDeps = noDeps[1:]
		idx, ok := ruleByName[name]
		if ok {
			sorted = append(sorted, rules[idx])
		}
		// Iterate over the rules slice (not the map) so that newly
		// zero-dep nodes are appended in rules-slice order, matching TS.
		for _, otherRule := range rules {
			other := otherRule.Name
			set, exists := dependencies[other]
			if !exists {
				continue
			}
			if _, has := set[name]; has {
				delete(set, name)
				if len(set) == 0 && other != name {
					noDeps = append(noDeps, other)
					delete(dependencies, other)
				}
			}
		}
		delete(dependencies, name)
	}
	return sorted, nil
}

// CollectAffectedRules returns the set of rules transitively affected by seeds
// (i.e. seed names + everything that depends on them).
func CollectAffectedRules(rules []schema.RuleConfig, seeds []string) map[string]struct{} {
	dependents := map[string][]string{}
	for _, r := range rules {
		for _, src := range r.Sources {
			if src.SourceType() == "ref" && src.Ref != "" {
				dependents[src.Ref] = append(dependents[src.Ref], r.Name)
			}
		}
	}
	affected := map[string]struct{}{}
	queue := make([]string, 0, len(seeds))
	for _, s := range seeds {
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
