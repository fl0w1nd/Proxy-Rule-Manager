package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
)

// failTransport fails the test when a preview reaches the network. Geo data is
// installed by updates, so a preview must resolve everything from local cache.
type failTransport struct{ t *testing.T }

func (f failTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Errorf("preview performed a network request: %s", req.URL)
	return nil, errors.New("network disabled")
}

func TestPreviewReadsGeoDataFromCacheOnly(t *testing.T) {
	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "geosite")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := &geosite.ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: "test-v9",
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
	// A fresh manager reads from disk; any download attempt is a test failure.
	manager := geosite.NewManager(cacheDir)
	manager.SetHTTPClient(&http.Client{Transport: failTransport{t: t}})

	cfg := &config.Config{
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule",
			Sources: []config.SourceConfig{{Geosite: "v2fly/google"}},
			Outputs: []string{"surge"},
		}},
	}
	report, err := Preview(
		context.Background(), cfg, dataDir, "rule", "", testRegistry(t), NewFetcher(),
		NewPreprocessRunner(), manager, nil, testLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Merged) != 1 {
		t.Fatalf("merged=%+v", report.Merged)
	}
	source := report.Sources[0]
	if source.Error != "" || len(source.Entries) != 1 {
		t.Fatalf("source=%+v", source)
	}
	if source.CacheFetchedAt != cache.FetchedAt || source.CacheVersion != cache.ResolvedVersion {
		t.Fatalf("cache metadata missing from source: %+v", source)
	}
}

func TestPreviewReportsMissingProviderData(t *testing.T) {
	dataDir := t.TempDir()
	manager := geosite.NewManager(filepath.Join(dataDir, "geosite"))
	manager.SetHTTPClient(&http.Client{Transport: failTransport{t: t}})

	cfg := &config.Config{
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule",
			Sources: []config.SourceConfig{{Geosite: "v2fly/google"}},
			Outputs: []string{"surge"},
		}},
	}
	_, err := Preview(
		context.Background(), cfg, dataDir, "rule", "", testRegistry(t), NewFetcher(),
		NewPreprocessRunner(), manager, nil, testLogger(),
	)
	var missing *MissingProviderDataError
	if !errors.As(err, &missing) {
		t.Fatalf("preview error = %v, want MissingProviderDataError", err)
	}
	if missing.Kind != "geosite" || missing.Provider != "v2fly" {
		t.Fatalf("missing=%+v", missing)
	}
}

func TestValidateGeoRefsReadsCacheOnly(t *testing.T) {
	dataDir := t.TempDir()
	manager := geosite.NewManager(filepath.Join(dataDir, "geosite"))
	manager.SetHTTPClient(&http.Client{Transport: failTransport{t: t}})

	cfg := &config.Config{Rules: []config.RuleConfig{{
		ID: "rule", Name: "Rule",
		Sources: []config.SourceConfig{{Geosite: "v2fly/google"}},
	}}}
	errs := ValidateGeositeRefs(context.Background(), cfg, manager, testLogger())
	if len(errs) == 0 {
		t.Fatal("missing cache produced no validation error")
	}
	joined := errs[0].Message
	if !strings.Contains(joined, "no local data") || !strings.Contains(joined, "v2fly") {
		t.Fatalf("message=%q", joined)
	}
}
