// Package geoip loads named IP networks from V2Ray-compatible GeoIP databases.
package geoip

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geodata"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
	"google.golang.org/protobuf/proto"
)

var SupportedProviders = []string{"loyalsoldier", "v2fly"}

type Entry struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ProviderCache = geodata.Cache[Entry]
type Manager = geodata.Manager[Entry]

type Ref struct{ Provider, List string }

type ListOverview struct {
	Name    string `json:"name"`
	Entries int    `json:"entries"`
}

func NewManager(dir string) *Manager {
	return geodata.NewManager[Entry](dir, "geoip", map[string]geodata.Source{
		"loyalsoldier": {Repository: "Loyalsoldier/geoip", Asset: "geoip.dat", Checksum: true},
		"v2fly":        {Repository: "v2fly/geoip", Asset: "geoip.dat", Checksum: true},
	}, decode, nil)
}

func ParseRef(value string) (Ref, error) {
	provider, list, ok := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "/")
	if !ok || strings.ContainsAny(list, "/@,!") {
		return Ref{}, fmt.Errorf("geoip reference %q must contain provider/list", value)
	}
	for _, part := range []string{provider, list} {
		if err := util.EnsureSafeSegment(part, "geoip reference"); err != nil {
			return Ref{}, err
		}
	}
	if !slices.Contains(SupportedProviders, provider) {
		return Ref{}, fmt.Errorf("unsupported geoip provider %q", provider)
	}
	return Ref{provider, list}, nil
}

func (r Ref) FormatRef() string { return r.Provider + "/" + r.List }

func decode(payload []byte, provider, version string) (*ProviderCache, error) {
	var data GeoIPList
	if err := proto.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("decode geoip: %w", err)
	}
	cache := &ProviderCache{Provider: provider, ResolvedVersion: version, FetchedAt: util.NowISO(), Entries: map[string][]Entry{}}
	for _, list := range data.Entry {
		name := strings.ToLower(strings.TrimSpace(list.CountryCode))
		if _, err := ParseRef(provider + "/" + name); err != nil {
			return nil, err
		}
		if list.ReverseMatch {
			return nil, fmt.Errorf("geoip %s uses unsupported reverse_match", name)
		}
		seen := map[netip.Prefix]bool{}
		for _, entry := range cache.Entries[name] {
			prefix, _ := netip.ParsePrefix(entry.Value)
			seen[prefix] = true
		}
		entries := cache.Entries[name]
		for _, cidr := range list.Cidr {
			addr, ok := netip.AddrFromSlice(cidr.Ip)
			if !ok || cidr.Prefix > uint32(addr.BitLen()) {
				return nil, fmt.Errorf("invalid CIDR in geoip %s", name)
			}
			prefix := netip.PrefixFrom(addr, int(cidr.Prefix)).Masked()
			if seen[prefix] {
				continue
			}
			seen[prefix] = true
			kind := "ipv6"
			if addr.Is4() {
				kind = "ipv4"
			}
			entries = append(entries, Entry{Type: kind, Value: prefix.String()})
		}
		slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Value, b.Value) })
		cache.Entries[name] = entries
	}
	for name := range cache.Entries {
		cache.Catalog = append(cache.Catalog, name)
	}
	slices.Sort(cache.Catalog)
	if len(cache.Catalog) == 0 {
		return nil, fmt.Errorf("geoip database is empty")
	}
	return cache, nil
}

func ResolveEntries(cache *ProviderCache, list string) ([]Entry, error) {
	if cache == nil {
		return nil, fmt.Errorf("geoip cache unavailable")
	}
	entries, ok := cache.Entries[strings.ToLower(list)]
	if !ok {
		return nil, fmt.Errorf("geoip list %q not found in provider %q", list, cache.Provider)
	}
	return entries, nil
}

func ResolveIR(cache *ProviderCache, list string) ([]ir.Entry, error) {
	entries, err := ResolveEntries(cache, list)
	if err != nil {
		return nil, err
	}
	out := make([]ir.Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, ir.Entry{Kind: ir.KindIPCIDR, Value: entry.Value})
	}
	return out, nil
}

func FilterEntries(entries []Entry, query string) []Entry {
	if query == "" {
		return entries
	}
	query = strings.ToLower(query)
	addr, addrErr := netip.ParseAddr(query)
	network, netErr := netip.ParsePrefix(query)
	out := make([]Entry, 0)
	for _, entry := range entries {
		prefix, err := netip.ParsePrefix(entry.Value)
		if err != nil {
			continue
		}
		if (addrErr == nil && prefix.Contains(addr)) || (netErr == nil && prefix.Overlaps(network)) || (addrErr != nil && netErr != nil && strings.Contains(entry.Value, query)) {
			out = append(out, entry)
		}
	}
	return out
}

func ListOverviews(cache *ProviderCache, query, match string) []ListOverview {
	out := make([]ListOverview, 0, len(cache.Catalog))
	for _, name := range cache.Catalog {
		entries := cache.Entries[name]
		if query != "" {
			if match == "content" {
				if len(FilterEntries(entries, query)) == 0 {
					continue
				}
			} else if !strings.Contains(name, strings.ToLower(query)) {
				continue
			}
		}
		out = append(out, ListOverview{Name: name, Entries: len(entries)})
	}
	return out
}
