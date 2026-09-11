package engine

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geohost"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

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
