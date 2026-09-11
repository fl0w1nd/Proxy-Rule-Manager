package geodata

// Source identifies a release asset.
type Source struct {
	Repository string
	Asset      string
	Checksum   bool
	Ref        string
}

type Cache[E any] struct {
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
