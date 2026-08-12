package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

// TestRebuildSiteFromPersistedState verifies that pages rebuilt from persisted
// data preserve recorded rule outcomes and show an empty state for new rules.
func TestRebuildSiteFromPersistedState(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "updated", Name: "Updated Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"}},
			{ID: "never", Name: "Never Updated", Sources: []config.SourceConfig{{Content: "DOMAIN,never.com"}}, Outputs: []string{"surge"}},
		},
	}
	eng := newTestUpdateEngine(t, cfg)

	// Persisted state for "updated": snapshot + artifact on disk.
	if err := eng.State.SaveSnapshot("updated", []ir.Entry{{Kind: ir.KindDomain, Value: "example.com"}}); err != nil {
		t.Fatal(err)
	}
	eng.State.SetRuleCheck("updated", state.RuleUpdated, time.Date(2026, 8, 12, 7, 26, 0, 0, time.Local), true)
	artifact := filepath.Join(dataDir, "rules", "surge", "updated.list")
	if err := os.WriteFile(artifact, []byte("DOMAIN,example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := eng.RebuildSite(); err != nil {
		t.Fatalf("RebuildSite: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(dataDir, site.StaticDir, site.IndexFile))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(index), "Updated Rule") || !strings.Contains(string(index), "rules/surge/updated.list") {
		t.Error("rebuilt index should list the on-disk artifact")
	}
	if _, err := os.Stat(filepath.Join(dataDir, site.StaticDir, "admin.html")); !os.IsNotExist(err) {
		t.Fatalf("RebuildSite created public admin.html: %v", err)
	}
}

// TestEnsureSiteFirstRun verifies a fresh deployment gets a complete site
// (empty state) before any update, and that a second call is a no-op.
func TestEnsureSiteFirstRun(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "r", Name: "r", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"}},
		},
	}
	eng := newTestUpdateEngine(t, cfg)

	if err := eng.EnsureSite(); err != nil {
		t.Fatalf("EnsureSite: %v", err)
	}
	staticDir := filepath.Join(dataDir, site.StaticDir)
	for _, name := range []string{site.IndexFile} {
		if _, err := os.Stat(filepath.Join(staticDir, name)); err != nil {
			t.Errorf("%s missing after EnsureSite: %v", name, err)
		}
	}
	index, err := os.ReadFile(filepath.Join(staticDir, site.IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "prm.svg") {
		t.Error("index should reference the builtin prm.svg icon")
	}
	if _, err := os.Stat(filepath.Join(staticDir, "icons", "prm.svg")); err != nil {
		t.Error("prm.svg not written to icons dir")
	}

	// Second run: pages are fresh; ensure they are not rewritten.
	indexPath := filepath.Join(staticDir, site.IndexFile)
	st1, _ := os.Stat(indexPath)
	if err := eng.EnsureSite(); err != nil {
		t.Fatalf("EnsureSite (2nd): %v", err)
	}
	st2, _ := os.Stat(indexPath)
	if st1.ModTime() != st2.ModTime() {
		t.Error("EnsureSite rewrote pages although assets were unchanged")
	}
}

func TestEnsureSiteRemovesManagedLegacyAdmin(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules:   []config.RuleConfig{{ID: "r", Name: "r", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	staticDir := filepath.Join(dataDir, site.StaticDir)
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adminPath := filepath.Join(staticDir, "admin.html")
	if err := os.WriteFile(adminPath, []byte("legacy admin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := eng.EnsureSite(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(adminPath); !os.IsNotExist(err) {
		t.Fatalf("legacy admin remains: %v", err)
	}
}
