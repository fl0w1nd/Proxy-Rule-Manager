package geodata

// Source identifies a release asset.
type Source struct {
	Repository string
	Asset      string
	Checksum   bool
	Ref        string
}

type Cache[E any] struct {
	SHA256          string         `json:"sha256,omitempty"`
	Provider        string         `json:"provider"`
	ResolvedVersion string         `json:"resolvedVersion"`
	FetchedAt       string         `json:"fetchedAt"`
	Catalog         []string       `json:"catalog"`
	Entries         map[string][]E `json:"entries"`
}

// Status summarizes the locally cached provider data.
type Status struct {
	Provider        string  `json:"provider"`
	Ready           bool    `json:"ready"`
	FetchedAt       *string `json:"fetchedAt"`
	ResolvedVersion *string `json:"resolvedVersion"`
	CatalogCount    int     `json:"catalogCount"`
}

// SameContent compares cached content, including caches created before digests were stored.
func SameContent[E any](a, b *Cache[E]) bool {
	if a == nil || b == nil {
		return false
	}
	if a.SHA256 != "" && b.SHA256 != "" {
		return a.SHA256 == b.SHA256
	}
	return a.ResolvedVersion == b.ResolvedVersion
}
