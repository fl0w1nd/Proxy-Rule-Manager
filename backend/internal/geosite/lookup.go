package geosite

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

type lookupIndex struct {
	exact   map[string]map[string]struct{} // value -> set of list names
	suffix  []suffixHit
	keyword []keywordHit
	regex   []regexHit
}

type suffixHit struct {
	listName string
	value    string
}

type keywordHit struct {
	listName string
	value    string
}

type regexHit struct {
	listName string
	pattern  *regexp.Regexp
}

var (
	lookupMu    sync.RWMutex
	lookupCache = map[string]*lookupIndex{}
)

func providerCacheKey(c *ProviderCache) string {
	if c == nil {
		return ""
	}
	return c.Provider + ":" + c.ResolvedVersion + ":" + c.FetchedAt
}

func buildLookupIndex(entries map[string][]Entry) *lookupIndex {
	idx := &lookupIndex{exact: map[string]map[string]struct{}{}}
	for listName, list := range entries {
		for _, e := range list {
			value := strings.TrimSpace(strings.ToLower(e.Value))
			if value == "" {
				continue
			}
			switch e.Type {
			case EntryFull:
				set, ok := idx.exact[value]
				if !ok {
					set = map[string]struct{}{}
					idx.exact[value] = set
				}
				set[listName] = struct{}{}
			case EntryDomain:
				idx.suffix = append(idx.suffix, suffixHit{listName, value})
			case EntryKeyword:
				idx.keyword = append(idx.keyword, keywordHit{listName, value})
			case EntryRegexp:
				if re, err := regexp.Compile(e.Value); err == nil {
					idx.regex = append(idx.regex, regexHit{listName, re})
				}
			}
		}
	}
	return idx
}

// LookupListsInEntries mirrors lookupGeositeListsInEntries.
func LookupListsInEntries(cache *ProviderCache, domain string) []string {
	if cache == nil {
		return nil
	}
	normalizedDomain := strings.TrimRight(strings.ToLower(strings.TrimSpace(domain)), ".")
	if normalizedDomain == "" {
		return nil
	}
	key := providerCacheKey(cache)
	lookupMu.RLock()
	idx, ok := lookupCache[key]
	lookupMu.RUnlock()
	if !ok {
		built := buildLookupIndex(cache.Entries)
		lookupMu.Lock()
		lookupCache[key] = built
		lookupMu.Unlock()
		idx = built
	}

	matches := map[string]struct{}{}
	if set, ok := idx.exact[normalizedDomain]; ok {
		for k := range set {
			matches[k] = struct{}{}
		}
	}
	for _, hit := range idx.suffix {
		if normalizedDomain == hit.value || strings.HasSuffix(normalizedDomain, "."+hit.value) {
			matches[hit.listName] = struct{}{}
		}
	}
	for _, hit := range idx.keyword {
		if strings.Contains(normalizedDomain, hit.value) {
			matches[hit.listName] = struct{}{}
		}
	}
	for _, hit := range idx.regex {
		if hit.pattern.MatchString(normalizedDomain) {
			matches[hit.listName] = struct{}{}
		}
	}

	out := make([]string, 0, len(matches))
	for k := range matches {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
