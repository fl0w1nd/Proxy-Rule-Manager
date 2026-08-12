package config

import "sort"

// DetectCircularDependency returns one rule-reference cycle, including the
// repeated start node, or nil when the graph is acyclic.
func DetectCircularDependency(rules []RuleConfig) []string {
	dependencies := make(map[string][]string, len(rules))
	known := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		known[rule.ID] = struct{}{}
		seen := make(map[string]struct{})
		for _, source := range rule.Sources {
			if source.SourceType() == "ref" && source.Ref != "" {
				seen[source.Ref] = struct{}{}
			}
		}
		for dependency := range seen {
			dependencies[rule.ID] = append(dependencies[rule.ID], dependency)
		}
		sort.Strings(dependencies[rule.ID])
	}

	state := make(map[string]uint8, len(rules))
	var path []string
	var visit func(string) []string
	visit = func(id string) []string {
		if state[id] == 1 {
			for i, current := range path {
				if current == id {
					cycle := append([]string(nil), path[i:]...)
					return append(cycle, id)
				}
			}
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		path = append(path, id)
		for _, dependency := range dependencies[id] {
			if _, ok := known[dependency]; !ok {
				continue
			}
			if cycle := visit(dependency); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		state[id] = 2
		return nil
	}

	for _, rule := range rules {
		if state[rule.ID] == 0 {
			if cycle := visit(rule.ID); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}
