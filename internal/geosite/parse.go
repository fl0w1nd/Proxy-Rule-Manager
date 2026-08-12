package geosite

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NormalizeAttrs returns a sorted, deduplicated, lower-cased attr slice.
func NormalizeAttrs(attrs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		v := normalizeName(attr)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func entryKey(entry Entry) string {
	return string(entry.Type) + ":" + entry.Value + ":" + strings.Join(NormalizeAttrs(entry.Attrs), "@")
}

func dedupeEntries(entries []Entry) []Entry {
	seen := map[string]struct{}{}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		normalized := Entry{Type: e.Type, Value: e.Value, Attrs: NormalizeAttrs(e.Attrs)}
		k := entryKey(normalized)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

// decodeProviderGeositeDat parses a v2fly-compatible geosite protobuf file.
func decodeProviderGeositeDat(payload []byte, provider, resolvedVersion string) (*ProviderCache, error) {
	siteList, err := decodeGeoSiteList(payload)
	if err != nil {
		return nil, fmt.Errorf("decode geosite data: %w", err)
	}
	entries := map[string][]Entry{}
	for _, site := range siteList.Entry {
		listName := normalizeName(site.CountryCode)
		if listName == "" {
			continue
		}
		raw := make([]Entry, 0, len(site.Domains))
		for _, d := range site.Domains {
			t, ok := mapDomainType(int32(d.Type))
			if !ok {
				continue
			}
			value := strings.TrimSpace(d.Value)
			if value == "" {
				continue
			}
			var attrs []string
			for _, attr := range d.Attribute {
				attrs = append(attrs, attr.Key)
			}
			raw = append(raw, Entry{Type: t, Value: value, Attrs: NormalizeAttrs(attrs)})
		}
		deduped := dedupeEntries(raw)
		if len(deduped) > 0 {
			entries[listName] = deduped
		}
	}
	catalog := make([]string, 0, len(entries))
	for k := range entries {
		catalog = append(catalog, k)
	}
	sort.Strings(catalog)
	return &ProviderCache{
		Provider:        provider,
		ResolvedVersion: resolvedVersion,
		FetchedAt:       util.NowISO(),
		Catalog:         catalog,
		Entries:         entries,
	}, nil
}

func mapDomainType(value int32) (EntryType, bool) {
	switch value {
	case 0:
		return EntryKeyword, true
	case 1:
		return EntryRegexp, true
	case 2:
		return EntryDomain, true
	case 3:
		return EntryFull, true
	}
	return "", false
}

// ErrInvalidGeositeSource is returned when a source is incompletely configured.
var ErrInvalidGeositeSource = errors.New("invalid geosite source")
