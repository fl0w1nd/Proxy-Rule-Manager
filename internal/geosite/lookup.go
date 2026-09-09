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
	values  []contentValue
}

type contentValue struct {
	value string
	lists []string
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
	valueLists := map[string]map[string]struct{}{}
	for listName, list := range entries {
		for _, e := range list {
			value := strings.TrimSpace(strings.ToLower(e.Value))
			if value == "" {
				continue
			}
			set, ok := valueLists[value]
			if !ok {
				set = map[string]struct{}{}
				valueLists[value] = set
			}
			set[listName] = struct{}{}
			switch e.Type {
			case EntryFull:
				exact, ok := idx.exact[value]
				if !ok {
					exact = map[string]struct{}{}
					idx.exact[value] = exact
				}
				exact[listName] = struct{}{}
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
	idx.values = make([]contentValue, 0, len(valueLists))
	for value, lists := range valueLists {
		names := make([]string, 0, len(lists))
		for name := range lists {
			names = append(names, name)
		}
		sort.Strings(names)
		idx.values = append(idx.values, contentValue{value: value, lists: names})
	}
	return idx
}

func lookupIndexFor(cache *ProviderCache) *lookupIndex {
	if cache == nil {
		return nil
	}
	key := providerCacheKey(cache)
	lookupMu.RLock()
	idx, ok := lookupCache[key]
	lookupMu.RUnlock()
	if ok {
		return idx
	}
	built := buildLookupIndex(cache.Entries)
	lookupMu.Lock()
	lookupCache[key] = built
	lookupMu.Unlock()
	return built
}

// LookupListsInEntries mirrors lookupGeositeListsInEntries.
func LookupListsInEntries(cache *ProviderCache, domain string) []string {
	normalizedDomain := strings.TrimRight(strings.ToLower(strings.TrimSpace(domain)), ".")
	if cache == nil || normalizedDomain == "" {
		return nil
	}
	idx := lookupIndexFor(cache)
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
	return sortedKeys(matches)
}

// SearchListsByContent returns lists whose entries match query by domain
// membership or by substring on entry values.
func SearchListsByContent(cache *ProviderCache, query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if cache == nil || q == "" {
		return nil
	}
	idx := lookupIndexFor(cache)
	matches := map[string]struct{}{}
	for _, name := range LookupListsInEntries(cache, q) {
		matches[name] = struct{}{}
	}
	for _, item := range idx.values {
		if !strings.Contains(item.value, q) {
			continue
		}
		for _, name := range item.lists {
			matches[name] = struct{}{}
		}
	}
	return sortedKeys(matches)
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
