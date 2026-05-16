package geosite

import "testing"

func TestBuildV2flyCacheFromRawFiles(t *testing.T) {
	files := map[string]string{
		"google":    "full:www.google.com @ads\ndomain:google.com\nkeyword:goog\nregexp:.*\\.gle$\ninclude:youtube",
		"youtube":   "domain:youtube.com",
		"empty.txt": "# comment only",
	}
	cache, err := BuildV2flyCacheFromRawFiles(files, "test")
	if err != nil {
		t.Fatalf("BuildV2flyCacheFromRawFiles: %v", err)
	}
	if got := cache.Entries["google"]; len(got) == 0 {
		t.Fatalf("expected google entries, got %#v", cache.Entries)
	}
	yt := cache.Entries["youtube"]
	if len(yt) != 1 || yt[0].Type != EntryDomain {
		t.Fatalf("youtube entry mismatch: %#v", yt)
	}
	// Include should have pulled youtube into google.
	hasYT := false
	for _, e := range cache.Entries["google"] {
		if e.Type == EntryDomain && e.Value == "youtube.com" {
			hasYT = true
		}
	}
	if !hasYT {
		t.Fatalf("include did not pull youtube into google: %#v", cache.Entries["google"])
	}
}

func TestResolveEntriesAttrs(t *testing.T) {
	cache := &ProviderCache{
		Provider: "test",
		Entries: map[string][]Entry{
			"news": {
				{Type: EntryFull, Value: "a.example.com", Attrs: []string{"cn"}},
				{Type: EntryFull, Value: "b.example.com", Attrs: []string{"!cn"}},
				{Type: EntryFull, Value: "c.example.com"},
			},
		},
	}
	got, err := ResolveEntries(cache, "news", []string{"cn"})
	if err != nil {
		t.Fatalf("ResolveEntries: %v", err)
	}
	if len(got) != 1 || got[0].Value != "a.example.com" {
		t.Fatalf("unexpected resolve: %#v", got)
	}
}

func TestRenderEntries(t *testing.T) {
	entries := []Entry{
		{Type: EntryFull, Value: "a.com"},
		{Type: EntryDomain, Value: "b.com"},
		{Type: EntryKeyword, Value: "key"},
		{Type: EntryRegexp, Value: ".*"},
	}
	out, err := RenderEntries(entries, "")
	if err != nil {
		t.Fatalf("RenderEntries: %v", err)
	}
	want := "DOMAIN,a.com\nDOMAIN-SUFFIX,b.com\nDOMAIN-KEYWORD,key\nDOMAIN-REGEX,.*"
	if out != want {
		t.Fatalf("render mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestLookupLists(t *testing.T) {
	cache := &ProviderCache{
		Provider: "test",
		Entries: map[string][]Entry{
			"a": {{Type: EntryDomain, Value: "example.com"}},
			"b": {{Type: EntryFull, Value: "exact.example.com"}},
			"c": {{Type: EntryKeyword, Value: "foo"}},
		},
	}
	if got := LookupListsInEntries(cache, "x.example.com"); len(got) != 1 || got[0] != "a" {
		t.Fatalf("subdomain lookup mismatch: %v", got)
	}
	if got := LookupListsInEntries(cache, "exact.example.com"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("exact lookup mismatch: %v", got)
	}
	if got := LookupListsInEntries(cache, "foofighter.example.org"); len(got) != 1 || got[0] != "c" {
		t.Fatalf("keyword lookup mismatch: %v", got)
	}
}
