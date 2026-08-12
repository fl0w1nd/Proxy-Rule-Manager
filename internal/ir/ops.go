package ir

import (
	"fmt"
	"regexp"
	"strings"
)

// Dedupe removes duplicate entries (by canonical key) preserving first-seen
// order.
func Dedupe(entries []Entry) []Entry {
	seen := make(map[string]bool, len(entries))
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		k := e.Key()
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// Union merges the sets in order, deduping by key.
func Union(sets ...[]Entry) []Entry {
	var all []Entry
	for _, s := range sets {
		all = append(all, s...)
	}
	return Dedupe(all)
}

// Intersect keeps entries (in first-set order) present in every set.
func Intersect(sets ...[]Entry) []Entry {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return Dedupe(sets[0])
	}
	present := make([]map[string]bool, len(sets)-1)
	for i, s := range sets[1:] {
		m := make(map[string]bool, len(s))
		for _, e := range s {
			m[e.Key()] = true
		}
		present[i] = m
	}
	var out []Entry
	seen := map[string]bool{}
	for _, e := range sets[0] {
		k := e.Key()
		if seen[k] {
			continue
		}
		seen[k] = true
		inAll := true
		for _, m := range present {
			if !m[k] {
				inAll = false
				break
			}
		}
		if inAll {
			out = append(out, e)
		}
	}
	return out
}

// Difference returns base minus every entry present in any subtract set.
func Difference(base []Entry, subtract ...[]Entry) []Entry {
	drop := map[string]bool{}
	for _, s := range subtract {
		for _, e := range s {
			drop[e.Key()] = true
		}
	}
	var out []Entry
	seen := map[string]bool{}
	for _, e := range base {
		k := e.Key()
		if seen[k] || drop[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// FilterAction says what happens to matching entries.
type FilterAction string

const (
	FilterKeep   FilterAction = "keep"   // keep only matching entries
	FilterRemove FilterAction = "remove" // remove matching entries
)

// ValueMatchMode selects how a value pattern matches.
type ValueMatchMode string

const (
	MatchKeyword ValueMatchMode = "keyword" // substring
	MatchSuffix  ValueMatchMode = "suffix"  // domain-label-aware suffix
	MatchPrefix  ValueMatchMode = "prefix"
	MatchExact   ValueMatchMode = "exact"
	MatchRegex   ValueMatchMode = "regex"
)

// FilterKinds keeps or removes entries by kind. Logical entries match when
// any nested leaf matches.
func FilterKinds(entries []Entry, kinds []Kind, action FilterAction) []Entry {
	want := make(map[Kind]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		matched := entryTouchesKinds(e, want)
		if (action == FilterKeep) == matched {
			out = append(out, e)
		}
	}
	return out
}

func entryTouchesKinds(e Entry, want map[Kind]bool) bool {
	if want[e.Kind] {
		return true
	}
	for _, s := range e.Sub {
		if entryTouchesKinds(s, want) {
			return true
		}
	}
	return false
}

// FilterValues keeps or removes entries whose value matches the pattern.
// Logical entries match when any nested leaf value matches.
func FilterValues(entries []Entry, mode ValueMatchMode, pattern string, action FilterAction) ([]Entry, error) {
	match, err := valueMatcher(mode, pattern)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		matched := entryValueMatches(e, match)
		if (action == FilterKeep) == matched {
			out = append(out, e)
		}
	}
	return out, nil
}

func entryValueMatches(e Entry, match func(string) bool) bool {
	if e.Value != "" && match(e.Value) {
		return true
	}
	for _, s := range e.Sub {
		if entryValueMatches(s, match) {
			return true
		}
	}
	return false
}

func valueMatcher(mode ValueMatchMode, pattern string) (func(string) bool, error) {
	switch mode {
	case MatchKeyword:
		return func(v string) bool { return strings.Contains(v, pattern) }, nil
	case MatchPrefix:
		return func(v string) bool { return strings.HasPrefix(v, pattern) }, nil
	case MatchExact:
		return func(v string) bool { return v == pattern }, nil
	case MatchSuffix:
		p := strings.TrimPrefix(pattern, ".")
		return func(v string) bool {
			return v == p || strings.HasSuffix(v, "."+p)
		}, nil
	case MatchRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid filter regex: %v", err)
		}
		return re.MatchString, nil
	default:
		return nil, fmt.Errorf("unknown value match mode %q", mode)
	}
}
