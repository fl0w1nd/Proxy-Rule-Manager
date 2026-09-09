package geosite

import (
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geodata"
)

const (
	ProviderV2fly        = "v2fly"
	ProviderLoyalsoldier = "loyalsoldier"
)

var SupportedProviders = []string{ProviderV2fly, ProviderLoyalsoldier}

type Manager = geodata.Manager[Entry]

func NewManager(dir string) *Manager {
	return geodata.NewManager(dir, "geosite", map[string]geodata.Source{
		ProviderV2fly:        {Repository: "v2fly/domain-list-community", Asset: "dlc.dat", Checksum: true},
		ProviderLoyalsoldier: {Repository: "Loyalsoldier/v2ray-rules-dat", Asset: "geosite.dat"},
	}, decodeProviderGeositeDat, func(cache *ProviderCache) {
		lookupMu.Lock()
		defer lookupMu.Unlock()
		for key := range lookupCache {
			if strings.HasPrefix(key, cache.Provider+":") {
				delete(lookupCache, key)
			}
		}
	})
}
