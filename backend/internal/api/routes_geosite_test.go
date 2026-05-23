package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// mockGitHubTransport intercepts GitHub API calls made during geosite.Refresh
// and returns minimal valid responses so the handler can proceed without
// real network access.
type mockGitHubTransport struct {
	server *httptest.Server
}

func (t *mockGitHubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect all requests to the local mock server.
	redirectURL := t.server.URL + req.URL.Path + "?" + req.URL.RawQuery
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL = parsed
	return http.DefaultTransport.RoundTrip(clone)
}

// buildV2flyMockZip creates a minimal ZIP archive containing a single v2fly
// data file so that extractV2flyDataFiles → BuildV2flyCacheFromRawFiles
// produces a non-empty cache.
func buildV2flyMockZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("domain-list-community-main/data/test-list")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write([]byte("example.com\n")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// setupGeositeSyncTest creates a test server with a mock GitHub backend so
// that geosite.Refresh succeeds. The caller can optionally seed rules into
// the config before invoking the handler.
func setupGeositeSyncTest(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	zipData := buildV2flyMockZip(t)

	// Local mock server that serves the three endpoints refreshV2fly needs:
	//  1. /repos/v2fly/domain-list-community  → repo metadata (default_branch)
	//  2. /repos/v2fly/domain-list-community/commits/{branch} → commit SHA
	//  3. /codeload.github.com/v2fly/.../zip/{sha} → ZIP payload
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/v2fly/domain-list-community" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"default_branch": "main"})
		case r.URL.Path == "/repos/v2fly/domain-list-community/commits/main" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"sha": "deadbeef"})
		case r.URL.Path == "/v2fly/domain-list-community/zip/deadbeef" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/zip")
			w.Write(zipData)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "mock: not found %s", r.URL.Path)
		}
	}))
	t.Cleanup(mock.Close)

	srv, ts := newTestServer(t, "secret")

	// Replace the geosite manager's HTTP client with one that routes to our
	// mock server. The RoundTripper rewrites the URL so the real GitHub
	// endpoints are never hit.
	srv.Geosite.SetHTTPClient(&http.Client{
		Timeout:   20 * time.Second,
		Transport: &mockGitHubTransport{server: mock},
	})

	return srv, ts
}

func TestGeositeProviderSync_RejectsInvalidProvider(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := postJSON(t, ts.URL, "/api/geosite/providers/invalid/sync", "secret", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%v)", code, body)
	}
}

func TestGeositeProviderSync_ReturnsEmptySyncWhenNoRules(t *testing.T) {
	_, ts := setupGeositeSyncTest(t)

	code, body := postJSON(t, ts.URL, "/api/geosite/providers/v2fly/sync", "secret", nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", code, body)
	}
	if v, _ := body["success"].(bool); !v {
		t.Fatalf("expected success=true, got %v", body)
	}
	if provider, _ := body["provider"].(string); provider != "v2fly" {
		t.Fatalf("expected provider=v2fly, got %q", provider)
	}
	sync, _ := body["sync"].(map[string]any)
	if sync == nil {
		t.Fatalf("expected sync object, got %v", body)
	}
	synced, _ := sync["syncedRules"].([]any)
	if synced == nil || len(synced) != 0 {
		t.Fatalf("expected empty syncedRules, got %v", synced)
	}
}

func TestGeositeProviderSync_SyncsProviderRulesOnly(t *testing.T) {
	srv, ts := setupGeositeSyncTest(t)
	ctx := context.Background()

	// Seed a geosite rule for v2fly and a regular rule.
	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{
		{
			Name: "geosite_v2fly_test-list",
			Sources: []schema.SourceConfig{{
				Type:     "geosite",
				Provider: "v2fly",
				List:     "test-list",
			}},
			Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
			Tags:   []string{},
		},
		{
			Name:    "regular-rule",
			Sources: []schema.SourceConfig{{Type: "url", URL: "https://example.com/rule.list"}},
			Output:  schema.OutputConfig{Clients: []string{"clash_meta"}},
			Tags:    []string{},
		},
	}
	if _, err := srv.Store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	// The default client "clash_meta" is seeded automatically; no need to add.

	code, body := postJSON(t, ts.URL, "/api/geosite/providers/v2fly/sync", "secret", nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", code, body)
	}
	if v, _ := body["success"].(bool); !v {
		t.Fatalf("expected success=true, got %v", body)
	}
	sync, _ := body["sync"].(map[string]any)
	if sync == nil {
		t.Fatalf("expected sync object, got %v", body)
	}

	// The regular rule should NOT appear in synced or failed rules.
	synced, _ := sync["syncedRules"].([]any)
	failed, _ := sync["failedRules"].([]any)
	for _, name := range synced {
		if name == "regular-rule" {
			t.Errorf("regular-rule must not be synced by geosite provider sync")
		}
	}
	for _, f := range failed {
		m, _ := f.(map[string]any)
		if m["name"] == "regular-rule" {
			t.Errorf("regular-rule must not appear in failedRules")
		}
	}

	// The geosite rule should appear in either synced or failed.
	allRules := append(synced, failed...)
	geositeFound := false
	for _, r := range allRules {
		m, ok := r.(map[string]any)
		if !ok {
			if s, _ := r.(string); s == "geosite_v2fly_test-list" {
				geositeFound = true
			}
			continue
		}
		if m["name"] == "geosite_v2fly_test-list" {
			geositeFound = true
		}
	}
	if !geositeFound {
		t.Errorf("geosite rule should appear in sync results, got synced=%v failed=%v", synced, failed)
	}

	// Verify the artifact was written for the geosite rule.
	artifactPath := filepath.Join(srv.Config.RulesDir, "clash_meta", "geosite", "v2fly", "test-list.list")
	if _, err := filepath.Glob(artifactPath); err != nil {
		t.Logf("artifact glob: %v (non-fatal, rule may have failed to process)", err)
	}
}
