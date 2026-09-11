package engine

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geohost"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

func TestHostedKindsForScope(t *testing.T) {
	tests := []struct {
		scope string
		want  []string
	}{
		{"all", []string{"geosite", "geoip", "mmdb", "asn"}},
		{"geosite", []string{"geosite"}},
		{"geoip", []string{"geoip", "mmdb", "asn"}},
		{"mmdb", []string{"mmdb"}},
		{"asn", []string{"asn"}},
		{"rules", nil},
	}
	for _, tc := range tests {
		got := hostedKindsForScope(tc.scope)
		if len(got) != len(tc.want) {
			t.Errorf("hostedKindsForScope(%q) = %v, want %v", tc.scope, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("hostedKindsForScope(%q)[%d] = %s, want %s", tc.scope, i, got[i], tc.want[i])
			}
		}
	}
}

func TestHostedGeoFilePath(t *testing.T) {
	dataDir := t.TempDir()
	path, err := hostedGeoFilePath(dataDir, "mmdb", "loyalsoldier", "Country.mmdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(dataDir, "geo", "mmdb", "loyalsoldier", "Country.mmdb")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}

	// Path traversal check
	if _, err := hostedGeoFilePath(dataDir, "..", "loyalsoldier", "Country.mmdb"); err == nil {
		t.Error("expected error for traversal in kind")
	}
	if _, err := hostedGeoFilePath(dataDir, "mmdb", "../bad", "Country.mmdb"); err == nil {
		t.Error("expected error for traversal in provider")
	}
}

func TestHostedProviderSummariesReadsState(t *testing.T) {
	dataDir := t.TempDir()
	st, err := state.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if err := st.SetHostedGeoUpdate("mmdb", "loyalsoldier", state.ProviderUnchanged, now); err != nil {
		t.Fatal(err)
	}

	eng := &UpdateEngine{
		DataDir: dataDir,
		State:   st,
		MMDB:    geohost.NewManager(filepath.Join(dataDir, "mmdb"), geohost.KindMMDB),
	}
	cfg := &config.Config{
		MMDB: &config.HostedGeoConfig{Providers: []config.HostedGeoProvider{{Name: "loyalsoldier"}}},
	}
	eng.SetConfig(cfg)

	summaries := eng.HostedProviderSummaries(geohost.KindMMDB)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	s := summaries[0]
	if s.Name != "loyalsoldier" {
		t.Errorf("got name %q, want loyalsoldier", s.Name)
	}
	if s.Result != state.ProviderUnchanged {
		t.Errorf("got result %q, want unchanged", s.Result)
	}
	if !s.CheckedAt.Equal(now) {
		t.Errorf("got checkedAt %v, want %v", s.CheckedAt, now)
	}
}
