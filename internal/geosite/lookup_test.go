package geosite

import "testing"

func TestLookupListsInEntries(t *testing.T) {
	cache := &ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: "lookup-test-v1",
		FetchedAt:       "2025-01-01T00:00:00.000Z",
		Entries: map[string][]Entry{
			"google": {
				{Type: EntryFull, Value: "google.com"},
				{Type: EntryDomain, Value: "google"},
			},
			"ads": {
				{Type: EntryKeyword, Value: "ads"},
			},
		},
	}

	// Suffix match: "www.google" matches domain "google" (ends with ".google")
	lists := LookupListsInEntries(cache, "www.google")
	if !contains(lists, "google") {
		t.Errorf("www.google should match google list, got %v", lists)
	}

	// Exact match: "google.com" matches full "google.com"
	lists = LookupListsInEntries(cache, "google.com")
	if !contains(lists, "google") {
		t.Errorf("google.com should match google list, got %v", lists)
	}

	// Keyword match: "ads.example.com" contains "ads"
	lists = LookupListsInEntries(cache, "ads.example.com")
	if !contains(lists, "ads") {
		t.Errorf("ads.example.com should match ads list, got %v", lists)
	}

	// No match: "example.org" matches nothing
	lists = LookupListsInEntries(cache, "example.org")
	if len(lists) != 0 {
		t.Errorf("example.org should match nothing, got %v", lists)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
