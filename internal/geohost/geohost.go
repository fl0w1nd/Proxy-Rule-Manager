package geohost

import (
	"slices"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geodata"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	KindMMDB = "mmdb"
	KindASN  = "asn"

	ProviderLoyalsoldier = "loyalsoldier"
)

type Manager = geodata.Manager[struct{}]
type Cache = geodata.Cache[struct{}]

var (
	SupportedMMDB = []string{ProviderLoyalsoldier}
	SupportedASN  = []string{ProviderLoyalsoldier}
)

func Supports(kind, name string) bool {
	return slices.Contains(Supported(kind), name)
}

func Supported(kind string) []string {
	switch kind {
	case KindMMDB:
		return append([]string(nil), SupportedMMDB...)
	case KindASN:
		return append([]string(nil), SupportedASN...)
	default:
		return nil
	}
}

func NewManager(dir, kind string) *Manager {
	return geodata.NewManager(dir, kind, sources(kind), decode, nil)
}

func sources(kind string) map[string]geodata.Source {
	switch kind {
	case KindMMDB:
		return map[string]geodata.Source{
			ProviderLoyalsoldier: {Repository: "Loyalsoldier/geoip", Asset: "Country.mmdb", Checksum: true},
		}
	case KindASN:
		return map[string]geodata.Source{
			ProviderLoyalsoldier: {Repository: "Loyalsoldier/geoip", Asset: "GeoLite2-ASN.mmdb", Checksum: true, Ref: "release"},
		}
	default:
		return map[string]geodata.Source{}
	}
}

func decode(_ []byte, provider, version string) (*Cache, error) {
	return &Cache{
		Provider:        provider,
		ResolvedVersion: version,
		FetchedAt:       util.NowISO(),
	}, nil
}
