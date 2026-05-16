// Package geosite ports src/lib/geosite.ts to Go.
package geosite

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
type ProviderCache struct {
	Provider        string             `json:"provider"`
	ResolvedVersion string             `json:"resolvedVersion"`
	FetchedAt       string             `json:"fetchedAt"`
	Catalog         []string           `json:"catalog"`
	Entries         map[string][]Entry `json:"entries"`
}

// ProviderStatus matches GeositeProviderStatus in TS.
type ProviderStatus struct {
	Provider        string  `json:"provider"`
	Ready           bool    `json:"ready"`
	FetchedAt       *string `json:"fetchedAt"`
	ResolvedVersion *string `json:"resolvedVersion"`
	CatalogCount    int     `json:"catalogCount"`
}

// CatalogSummary matches GeositeCatalogSummary in TS.
type CatalogSummary struct {
	Name       string   `json:"name"`
	Attrs      []string `json:"attrs"`
	EntryCount int      `json:"entryCount"`
}
