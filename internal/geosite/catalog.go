package geosite

import (
	"sort"
	"strings"
)

// ListOverviews returns catalog rows with per-attr entry counts.
func ListOverviews(cache *ProviderCache) []ListOverview {
	if cache == nil {
		return nil
	}
	out := make([]ListOverview, 0, len(cache.Catalog))
	for _, name := range cache.Catalog {
		entries := cache.Entries[name]
		attrCount := map[string]int{}
		for _, entry := range entries {
			for _, attr := range entry.Attrs {
				attrCount[attr]++
			}
		}
		variants := make([]VariantOverview, 0, len(attrCount))
		for attr, n := range attrCount {
			variants = append(variants, VariantOverview{Attr: attr, Entries: n})
		}
		sort.Slice(variants, func(i, j int) bool { return variants[i].Attr < variants[j].Attr })
		out = append(out, ListOverview{Name: name, Entries: len(entries), Variants: variants})
	}
	return out
}

// FilterListOverviews keeps lists whose name or variant attr contains query.
func FilterListOverviews(lists []ListOverview, query string) []ListOverview {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return lists
	}
	out := make([]ListOverview, 0)
	for _, list := range lists {
		if strings.Contains(list.Name, q) {
			out = append(out, list)
			continue
		}
		matched := false
		for _, variant := range list.Variants {
			if strings.Contains(variant.Attr, q) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, list)
		}
	}
	return out
}

// SelectListOverviews keeps catalog rows whose names appear in the given set,
// preserving catalog order.
func SelectListOverviews(lists []ListOverview, names []string) []ListOverview {
	if len(names) == 0 {
		return []ListOverview{}
	}
	wanted := map[string]struct{}{}
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	out := make([]ListOverview, 0, len(names))
	for _, list := range lists {
		if _, ok := wanted[list.Name]; ok {
			out = append(out, list)
		}
	}
	return out
}

// FilterEntries keeps entries whose value or attr contains query.
func FilterEntries(entries []Entry, query string) []Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return entries
	}
	out := make([]Entry, 0)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Value), q) {
			out = append(out, entry)
			continue
		}
		matched := false
		for _, attr := range entry.Attrs {
			if strings.Contains(attr, q) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, entry)
		}
	}
	return out
}
