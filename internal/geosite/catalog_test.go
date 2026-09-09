package geosite

import "testing"

func sampleCatalogCache() *ProviderCache {
	return &ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: "catalog-test-v1",
		FetchedAt:       "2026-01-01T00:00:00.000Z",
		Catalog:         []string{"ads", "geolocation-other", "google"},
		Entries: map[string][]Entry{
			"google": {
				{Type: EntryFull, Value: "google.com", Attrs: []string{"cn"}},
				{Type: EntryDomain, Value: "google.com"},
				{Type: EntryFull, Value: "youtube.com"},
			},
			"ads": {
				{Type: EntryKeyword, Value: "doubleclick"},
			},
			"geolocation-other": {
				{Type: EntryDomain, Value: "example.org", Attrs: []string{"ads"}},
			},
		},
	}
}

func TestListOverviewsCountsVariants(t *testing.T) {
	lists := ListOverviews(sampleCatalogCache())
	if len(lists) != 3 || lists[2].Name != "google" || lists[2].Entries != 3 {
		t.Fatalf("lists=%+v", lists)
	}
	if len(lists[2].Variants) != 1 || lists[2].Variants[0].Attr != "cn" || lists[2].Variants[0].Entries != 1 {
		t.Fatalf("google variants=%+v", lists[2].Variants)
	}
}

func TestFilterListOverviewsByNameAndAttr(t *testing.T) {
	lists := ListOverviews(sampleCatalogCache())
	byName := FilterListOverviews(lists, "geo")
	if len(byName) != 1 || byName[0].Name != "geolocation-other" {
		t.Fatalf("name filter=%+v", byName)
	}
	byAttr := FilterListOverviews(lists, "cn")
	if len(byAttr) != 1 || byAttr[0].Name != "google" {
		t.Fatalf("attr filter=%+v", byAttr)
	}
}

func TestSearchListsByContentMatchesDomainAndSubstring(t *testing.T) {
	cache := sampleCatalogCache()
	domain := SearchListsByContent(cache, "www.google.com")
	if !contains(domain, "google") {
		t.Fatalf("domain search=%v", domain)
	}
	substring := SearchListsByContent(cache, "youtube.com")
	if !contains(substring, "google") {
		t.Fatalf("substring search=%v", substring)
	}
	keyword := SearchListsByContent(cache, "tracker.doubleclick.net")
	if !contains(keyword, "ads") {
		t.Fatalf("keyword search=%v", keyword)
	}
}

func TestFilterEntriesByValueAndAttr(t *testing.T) {
	entries := sampleCatalogCache().Entries["google"]
	byValue := FilterEntries(entries, "youtube")
	if len(byValue) != 1 || byValue[0].Value != "youtube.com" {
		t.Fatalf("value filter=%+v", byValue)
	}
	byAttr := FilterEntries(entries, "cn")
	if len(byAttr) != 1 || byAttr[0].Value != "google.com" {
		t.Fatalf("attr filter=%+v", byAttr)
	}
}

func TestSelectListOverviewsPreservesCatalogOrder(t *testing.T) {
	lists := ListOverviews(sampleCatalogCache())
	selected := SelectListOverviews(lists, []string{"google", "ads"})
	if len(selected) != 2 || selected[0].Name != "ads" || selected[1].Name != "google" {
		t.Fatalf("selected=%+v", selected)
	}
}
