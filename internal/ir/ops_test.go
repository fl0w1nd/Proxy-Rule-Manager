package ir

import (
	"testing"
)

func entriesOf(values ...string) []Entry {
	out := make([]Entry, len(values))
	for i, v := range values {
		out[i] = Entry{Kind: KindDomainSuffix, Value: v}
	}
	return out
}

func values(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Value
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSetOps(t *testing.T) {
	a := entriesOf("a.com", "b.com", "c.com")
	b := entriesOf("b.com", "d.com")

	if got := values(Union(a, b)); !eq(got, []string{"a.com", "b.com", "c.com", "d.com"}) {
		t.Errorf("union: %v", got)
	}
	if got := values(Intersect(a, b)); !eq(got, []string{"b.com"}) {
		t.Errorf("intersect: %v", got)
	}
	if got := values(Difference(a, b)); !eq(got, []string{"a.com", "c.com"}) {
		t.Errorf("difference: %v", got)
	}
	dup := append(entriesOf("x.com"), entriesOf("x.com", "y.com")...)
	if got := values(Dedupe(dup)); !eq(got, []string{"x.com", "y.com"}) {
		t.Errorf("dedupe: %v", got)
	}
}

func TestDedupeRespectsKindAndFlags(t *testing.T) {
	entries := []Entry{
		{Kind: KindDomain, Value: "x.com"},
		{Kind: KindDomainSuffix, Value: "x.com"}, // different kind: kept
		{Kind: KindIPCIDR, Value: "10.0.0.0/8"},
		{Kind: KindIPCIDR, Value: "10.0.0.0/8", Flags: []string{FlagNoResolve}}, // different flags: kept
		{Kind: KindDomain, Value: "x.com"},                                      // exact dup: removed
	}
	if got := len(Dedupe(entries)); got != 4 {
		t.Errorf("dedupe kept %d, want 4", got)
	}
}

func TestFilterKinds(t *testing.T) {
	entries := []Entry{
		{Kind: KindDomainSuffix, Value: "a.com"},
		{Kind: KindProcessName, Value: "curl"},
		{Kind: KindAnd, Sub: []Entry{{Kind: KindProcessName, Value: "wget"}, {Kind: KindNetwork, Value: "udp"}}},
	}
	removed := FilterKinds(entries, []Kind{KindProcessName}, FilterRemove)
	if len(removed) != 1 || removed[0].Kind != KindDomainSuffix {
		t.Errorf("remove process_name (incl. nested): %+v", removed)
	}
	kept := FilterKinds(entries, []Kind{KindDomainSuffix}, FilterKeep)
	if len(kept) != 1 || kept[0].Value != "a.com" {
		t.Errorf("keep domain_suffix: %+v", kept)
	}
}

func TestFilterValues(t *testing.T) {
	entries := entriesOf("ads.example.com", "cdn.example.com", "tracker.io")

	got, err := FilterValues(entries, MatchKeyword, "example", FilterKeep)
	if err != nil || len(got) != 2 {
		t.Errorf("keyword keep: %v %v", got, err)
	}
	got, err = FilterValues(entries, MatchSuffix, "example.com", FilterRemove)
	if err != nil || len(got) != 1 || got[0].Value != "tracker.io" {
		t.Errorf("suffix remove: %v %v", got, err)
	}
	got, err = FilterValues(entries, MatchRegex, `^(ads|tracker)\.`, FilterRemove)
	if err != nil || len(got) != 1 || got[0].Value != "cdn.example.com" {
		t.Errorf("regex remove: %v %v", got, err)
	}
	if _, err = FilterValues(entries, MatchRegex, "([", FilterKeep); err == nil {
		t.Error("invalid regex should error")
	}
}

func TestDiff(t *testing.T) {
	oldSet := []Entry{
		{Kind: KindDomainSuffix, Value: "keep.com"},
		{Kind: KindDomainSuffix, Value: "gone.com"},
		{Kind: KindIPCIDR, Value: "10.0.0.0/8"},
	}
	newSet := []Entry{
		{Kind: KindDomainSuffix, Value: "keep.com"},
		{Kind: KindDomainSuffix, Value: "new.com"},
		{Kind: KindIPCIDR, Value: "10.0.0.0/8"},
		{Kind: KindGeoIP, Value: "CN"},
	}
	d := Diff(oldSet, newSet)
	if d.AddedCount != 2 || d.RemovedCount != 1 {
		t.Fatalf("counts: +%d -%d", d.AddedCount, d.RemovedCount)
	}
	if len(d.Groups) != 2 {
		t.Fatalf("groups: %+v", d.Groups)
	}
	// Reordering only -> empty diff.
	if got := Diff(newSet, []Entry{newSet[3], newSet[0], newSet[2], newSet[1]}); !got.Empty() {
		t.Errorf("reorder should be empty diff: %+v", got)
	}

	// Canonical set semantics collapse duplicates and treat flags as identity.
	duplicates := Diff(nil, []Entry{
		{Kind: KindDomain, Value: "duplicate.example"},
		{Kind: KindDomain, Value: "duplicate.example"},
	})
	if duplicates.AddedCount != 1 {
		t.Fatalf("duplicate additions: %+v", duplicates)
	}
	flags := Diff(
		[]Entry{{Kind: KindIPCIDR, Value: "192.0.2.0/24"}},
		[]Entry{{Kind: KindIPCIDR, Value: "192.0.2.0/24", Flags: []string{FlagNoResolve}}},
	)
	if flags.AddedCount != 1 || flags.RemovedCount != 1 {
		t.Fatalf("flag change: %+v", flags)
	}
}

func TestCountKinds(t *testing.T) {
	counts := CountKinds([]Entry{
		{Kind: KindDomainSuffix, Value: "a"},
		{Kind: KindDomainSuffix, Value: "b"},
		{Kind: KindIPCIDR, Value: "10.0.0.0/8"},
	})
	if len(counts) != 2 || counts[0].Kind != KindDomainSuffix || counts[0].Count != 2 {
		t.Errorf("counts: %+v", counts)
	}
}
