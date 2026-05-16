package geosite

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrListNotFound is returned by ResolveEntries when the requested list does
// not exist in the provider cache. Callers can use errors.Is to distinguish
// "upstream removed this list" from generic processing errors and surface the
// fact through dedicated diagnostics instead of letting it drown in noise.
var ErrListNotFound = errors.New("geosite list not found")

// ResolveEntries returns the list entries filtered by attrs (all required).
func ResolveEntries(cache *ProviderCache, list string, attrs []string) ([]Entry, error) {
	if cache == nil {
		return nil, fmt.Errorf("provider cache is empty")
	}
	entries, ok := cache.Entries[normalizeName(list)]
	if !ok {
		return nil, fmt.Errorf("%w: %q for provider %q", ErrListNotFound, list, cache.Provider)
	}
	normalizedAttrs := NormalizeAttrs(attrs)
	out := make([]Entry, 0, len(entries))
	if len(normalizedAttrs) == 0 {
		for _, e := range entries {
			out = append(out, Entry{Type: e.Type, Value: e.Value, Attrs: append([]string(nil), e.Attrs...)})
		}
		return out, nil
	}
	for _, e := range entries {
		ok := true
		for _, attr := range normalizedAttrs {
			found := false
			for _, ea := range e.Attrs {
				if ea == attr {
					found = true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, Entry{Type: e.Type, Value: e.Value, Attrs: append([]string(nil), e.Attrs...)})
		}
	}
	return out, nil
}

// RenderEntries renders entries using the given profile (mihomo-classical only).
func RenderEntries(entries []Entry, profile string) (string, error) {
	if profile == "" {
		profile = "mihomo-classical"
	}
	if profile != "mihomo-classical" {
		return "", fmt.Errorf("unsupported geosite render profile: %s", profile)
	}
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte('\n')
		}
		switch e.Type {
		case EntryDomain:
			sb.WriteString("DOMAIN-SUFFIX,")
		case EntryFull:
			sb.WriteString("DOMAIN,")
		case EntryKeyword:
			sb.WriteString("DOMAIN-KEYWORD,")
		case EntryRegexp:
			sb.WriteString("DOMAIN-REGEX,")
		}
		sb.WriteString(e.Value)
	}
	return sb.String(), nil
}

// CatalogSummaries returns a sorted catalog summary derived from a cache.
func CatalogSummaries(cache *ProviderCache) []CatalogSummary {
	if cache == nil {
		return nil
	}
	out := make([]CatalogSummary, 0, len(cache.Catalog))
	for _, name := range cache.Catalog {
		entries := cache.Entries[name]
		attrSet := map[string]struct{}{}
		for _, e := range entries {
			for _, a := range e.Attrs {
				attrSet[a] = struct{}{}
			}
		}
		attrs := make([]string, 0, len(attrSet))
		for k := range attrSet {
			attrs = append(attrs, k)
		}
		sort.Strings(attrs)
		out = append(out, CatalogSummary{
			Name:       name,
			Attrs:      attrs,
			EntryCount: len(entries),
		})
	}
	return out
}
