// Package geosite ports src/lib/geosite.ts to Go.
package geosite

import "github.com/fl0w1nd/proxy-rule-manager/internal/geodata"

// EntryType matches GeositeEntryType in TS.
type EntryType string

const (
	EntryDomain  EntryType = "domain"
	EntryFull    EntryType = "full"
	EntryKeyword EntryType = "keyword"
	EntryRegexp  EntryType = "regexp"
)

// Entry matches GeositeEntry in TS.
type Entry struct {
	Type  EntryType `json:"type"`
	Value string    `json:"value"`
	Attrs []string  `json:"attrs"`
}

// ProviderCache matches GeositeProviderCache in TS.
type ProviderCache = geodata.Cache[Entry]
type ProviderStatus = geodata.Status

// CatalogSummary matches GeositeCatalogSummary in TS.
type CatalogSummary struct {
	Name       string   `json:"name"`
	Attrs      []string `json:"attrs"`
	EntryCount int      `json:"entryCount"`
}

// VariantOverview is one attribute variant in a list catalog.
type VariantOverview struct {
	Attr    string `json:"attr"`
	Entries int    `json:"entries"`
}

// ListOverview is one geosite list with variant counts for the catalog API.
type ListOverview struct {
	Name     string            `json:"name"`
	Entries  int               `json:"entries"`
	Variants []VariantOverview `json:"variants"`
}
