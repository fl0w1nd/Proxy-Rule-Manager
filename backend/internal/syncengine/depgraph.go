package syncengine

import (
	"fmt"
	"sort"
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
//
// Complexity: O(V + E). The previous version re-scanned the whole rules
// slice for every node it processed, which degraded to O(V * V) on large
// configs (e.g. a full geosite import). We now build the reverse adjacency
// once and only walk a node's actual dependents when it's emitted.
//
// Determinism is preserved by sorting each dependents list once (by the
// dependent's index in the input slice), so when a single completion
// unblocks several nodes simultaneously they enter the queue in the same
// order the legacy algorithm produced.
func TopologicalSort(rules []schema.RuleConfig, skipMissingDepsCheck bool) ([]schema.RuleConfig, error) {
	if cycle := DetectCircularDependency(rules); cycle != nil {
		return nil, fmt.Errorf("检测到循环依赖: %s", strings.Join(cycle, " → "))
	}
	ruleByName := make(map[string]int, len(rules))
	for i, r := range rules {
		ruleByName[r.Name] = i
	}

	// inDegree[i] counts unresolved dependencies of rules[i]; dependents[i]
	// is the list of indices that depend on rules[i] (i.e. the reverse
	// edges), sorted ascending so iteration order matches the input slice.
	inDegree := make([]int, len(rules))
	dependents := make([][]int, len(rules))
	var missing []string
	for i := range rules {
		deps := ExtractDependencies(&rules[i])
		for dep := range deps {
			j, ok := ruleByName[dep]
			if !ok {
				if !skipMissingDepsCheck {
					missing = append(missing, fmt.Sprintf(`规则 "%s" 引用了不存在的规则 "%s"`, rules[i].Name, dep))
				}
				continue
			}
			inDegree[i]++
			dependents[j] = append(dependents[j], i)
		}
	}
	if !skipMissingDepsCheck && len(missing) > 0 {
		return nil, fmt.Errorf("依赖缺失:\n%s", strings.Join(missing, "\n"))
	}
	for j := range dependents {
		if len(dependents[j]) > 1 {
			sort.Ints(dependents[j])
		}
	}

	// Seed the queue with zero-indegree nodes in input-slice order so the
	// initial dequeue order is stable (matches the legacy implementation
	// and the determinism tests in depgraph_test.go).
	queue := make([]int, 0, len(rules))
	for i := range rules {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	sorted := make([]schema.RuleConfig, 0, len(rules))
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
