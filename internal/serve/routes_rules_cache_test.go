package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
)

// Preview resolves geo sources from local cache, so a provider that was never
// downloaded must be reported instead of fetched.
func TestRulePreviewReportsMissingGeoData(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	s.Engine.Geosite = geosite.NewManager(filepath.Join(s.DataDir, "geosite"))

	rec := localFileJSON(s.Handler(), http.MethodPost, "/api/v1/rules/preview", `{
		"rule": {
			"id": "geo",
			"name": "Geo",
			"sources": [{"geosite": "v2fly/google"}],
			"outputs": ["surge"]
		}
	}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "geo_data_missing") {
		t.Fatalf("preview=%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "v2fly") || !strings.Contains(rec.Body.String(), "还没有本地数据") {
		t.Fatalf("message does not name the provider: %s", rec.Body.String())
	}
}

// The preview payload carries the cache timestamp so a stale preview is
// visible without opening the data directory.
func TestRulePreviewReportsGeoCacheAge(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	cacheDir := filepath.Join(s.DataDir, "geosite")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := &geosite.ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: "test-v7",
		FetchedAt:       "2026-01-02T03:04:05.000Z",
		Catalog:         []string{"google"},
		Entries: map[string][]geosite.Entry{
			"google": {{Type: geosite.EntryDomain, Value: "google.example"}},
		},
	}
	payload, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "v2fly.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	s.Engine.Geosite = geosite.NewManager(cacheDir)

	rec := localFileJSON(s.Handler(), http.MethodPost, "/api/v1/rules/preview", `{
		"rule": {
			"id": "geo",
			"name": "Geo",
			"sources": [{"geosite": "v2fly/google"}],
			"outputs": ["surge"]
		}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview=%d %s", rec.Code, rec.Body.String())
	}
	var report rulePreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Sources) != 1 {
		t.Fatalf("sources=%+v", report.Sources)
	}
	source := report.Sources[0]
	if source.CacheFetchedAt != cache.FetchedAt || source.CacheVersion != cache.ResolvedVersion {
		t.Fatalf("source=%+v", source)
	}
	if source.Entries != 1 {
		t.Fatalf("entries=%d, want the cached entry", source.Entries)
	}
}
