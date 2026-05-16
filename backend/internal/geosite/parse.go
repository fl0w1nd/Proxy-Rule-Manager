package geosite

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
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

func matchesRequiredAttrs(entry Entry, required, excluded []string) bool {
	attrs := map[string]struct{}{}
	for _, a := range NormalizeAttrs(entry.Attrs) {
		attrs[a] = struct{}{}
	}
	for _, a := range required {
		if _, ok := attrs[a]; !ok {
			return false
		}
	}
	for _, a := range excluded {
		if _, ok := attrs[a]; ok {
			return false
		}
	}
	return true
}

func parseRuleType(token string) Entry {
	prefix, rest := "domain", token
	if idx := strings.Index(token, ":"); idx >= 0 {
		prefix, rest = token[:idx], token[idx+1:]
	}
	value := strings.TrimSpace(rest)
	switch strings.ToLower(strings.TrimSpace(prefix)) {
	case "full":
		return Entry{Type: EntryFull, Value: value}
	case "domain":
		return Entry{Type: EntryDomain, Value: value}
	case "keyword":
		return Entry{Type: EntryKeyword, Value: value}
	case "regexp":
		return Entry{Type: EntryRegexp, Value: value}
	default:
		return Entry{Type: EntryDomain, Value: strings.ToLower(strings.TrimSpace(token))}
	}
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.TrimSpace(line)
}

type rawInclude struct {
	List     string
	Required []string
	Excluded []string
}

type rawList struct {
	Entries  []Entry
	Includes []rawInclude
}

func ensureRawList(m map[string]*rawList, listName string) *rawList {
	normalized := normalizeName(listName)
	if existing, ok := m[normalized]; ok {
		return existing
	}
	v := &rawList{}
	m[normalized] = v
	return v
}

func parseInclude(firstToken string, restTokens []string) rawInclude {
	includeValue := strings.TrimSpace(firstToken[len("include:"):])
	parts := []string{}
	for _, part := range strings.Split(includeValue, "@") {
		if t := strings.TrimSpace(part); t != "" {
			parts = append(parts, t)
		}
	}
	list := ""
	if len(parts) > 0 {
		list = normalizeName(parts[0])
	}
	var attrTokens []string
	for _, p := range parts[1:] {
		attrTokens = append(attrTokens, "@"+p)
	}
	for _, t := range restTokens {
		if t := strings.TrimSpace(t); t != "" {
			attrTokens = append(attrTokens, t)
		}
	}
	var required, excluded []string
	for _, token := range attrTokens {
		if !strings.HasPrefix(token, "@") {
			continue
		}
		attr := strings.ToLower(strings.TrimSpace(token[1:]))
		if attr == "" {
			continue
		}
		if strings.HasPrefix(attr, "-") {
			excluded = append(excluded, attr[1:])
		} else {
			required = append(required, attr)
		}
	}
	return rawInclude{List: list, Required: NormalizeAttrs(required), Excluded: NormalizeAttrs(excluded)}
}

func parseV2flyRawLists(files map[string]string) map[string]*rawList {
	out := make(map[string]*rawList)
	for fileName, content := range files {
		list := ensureRawList(out, fileName)
		for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
			stripped := stripComment(line)
			if stripped == "" {
				continue
			}
			tokens := strings.Fields(stripped)
			if len(tokens) == 0 {
				continue
			}
			first, rest := tokens[0], tokens[1:]
			if strings.HasPrefix(first, "include:") {
				list.Includes = append(list.Includes, parseInclude(first, rest))
				continue
			}
			entry := parseRuleType(first)
			var attrs, affiliations []string
			for _, t := range rest {
				switch {
				case strings.HasPrefix(t, "@"):
					attrs = append(attrs, t[1:])
				case strings.HasPrefix(t, "&"):
					affiliations = append(affiliations, t[1:])
				}
			}
			entry.Attrs = NormalizeAttrs(attrs)
			list.Entries = append(list.Entries, entry)
			for _, aff := range affiliations {
				clone := Entry{Type: entry.Type, Value: entry.Value, Attrs: append([]string(nil), entry.Attrs...)}
				ensureRawList(out, aff).Entries = append(ensureRawList(out, aff).Entries, clone)
			}
		}
	}
	return out
}

func expandRawLists(rawLists map[string]*rawList) (map[string][]Entry, error) {
	memo := map[string][]Entry{}
	visiting := map[string]bool{}

	var expand func(string) ([]Entry, error)
	expand = func(listName string) ([]Entry, error) {
		normalized := normalizeName(listName)
		if v, ok := memo[normalized]; ok {
			return v, nil
		}
		if visiting[normalized] {
			return nil, fmt.Errorf("circular geosite include detected for list %q", normalized)
		}
		visiting[normalized] = true
		list, ok := rawLists[normalized]
		if !ok {
			list = &rawList{}
		}
		combined := make([]Entry, 0, len(list.Entries))
		for _, e := range list.Entries {
			combined = append(combined, Entry{Type: e.Type, Value: e.Value, Attrs: append([]string(nil), e.Attrs...)})
		}
		for _, inc := range list.Includes {
			included, err := expand(inc.List)
			if err != nil {
				return nil, err
			}
			for _, e := range included {
				if matchesRequiredAttrs(e, inc.Required, inc.Excluded) {
					combined = append(combined, Entry{Type: e.Type, Value: e.Value, Attrs: append([]string(nil), e.Attrs...)})
				}
			}
		}
		delete(visiting, normalized)
		deduped := dedupeEntries(combined)
		memo[normalized] = deduped
		return deduped, nil
	}

	out := map[string][]Entry{}
	for listName := range rawLists {
		entries, err := expand(listName)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			out[listName] = entries
		}
	}
	return out, nil
}

// BuildV2flyCacheFromRawFiles returns a ProviderCache from a map of fileName→content.
func BuildV2flyCacheFromRawFiles(files map[string]string, resolvedVersion string) (*ProviderCache, error) {
	expanded, err := expandRawLists(parseV2flyRawLists(files))
	if err != nil {
		return nil, err
	}
	catalog := make([]string, 0, len(expanded))
	for k := range expanded {
		catalog = append(catalog, k)
	}
	sort.Strings(catalog)
	return &ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: resolvedVersion,
		FetchedAt:       util.NowISO(),
		Catalog:         catalog,
		Entries:         expanded,
	}, nil
}

// DecodeLoyalsoldierGeositeDat parses the protobuf bytes from Loyalsoldier release.
func DecodeLoyalsoldierGeositeDat(payload []byte, resolvedVersion string) (*ProviderCache, error) {
	sites, err := decodeGeoSiteList(payload)
	if err != nil {
		return nil, fmt.Errorf("decode geosite.dat: %w", err)
	}
	entries := map[string][]Entry{}
	for _, site := range sites {
		listName := normalizeName(site.CountryCode)
		if listName == "" {
			continue
		}
		raw := make([]Entry, 0, len(site.Domains))
		for _, d := range site.Domains {
			t, ok := mapDomainType(d.Type)
			if !ok {
				continue
			}
			value := strings.TrimSpace(d.Value)
			if value == "" {
				continue
			}
			raw = append(raw, Entry{Type: t, Value: value, Attrs: NormalizeAttrs(d.Attribute)})
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
		Provider:        "loyalsoldier",
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
