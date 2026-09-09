package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
)

func TestGeositeCatalogAndListSearch(t *testing.T) {
	s := geositeCatalogServer(t)
	h := s.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"clients":["surge"]`) || !strings.Contains(rec.Body.String(), `"supported":["v2fly","loyalsoldier"]`) {
		t.Fatalf("providers: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/catalog", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog: %d %s", rec.Code, rec.Body.String())
	}
	var catalog geositeCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Total != 3 || catalog.Lists[2].Name != "google" || catalog.Lists[2].Entries != 3 || len(catalog.Lists[2].Variants) != 1 {
		t.Fatalf("catalog=%+v", catalog)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/catalog?q=geo&match=name", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"geolocation-!cn"`) || strings.Contains(rec.Body.String(), `"google"`) {
		t.Fatalf("name search: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/catalog?q=www.google.com&match=content", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"google"`) {
		t.Fatalf("content search: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/lists/google?limit=2&offset=0", nil))
	var listed geositeListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || listed.Total != 3 || len(listed.Items) != 2 || listed.Items[0].Value != "google.com" {
		t.Fatalf("list: %d %+v", rec.Code, listed)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/lists/google?attr=cn", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) || !strings.Contains(rec.Body.String(), `"google.com"`) {
		t.Fatalf("variant: %d %s", rec.Code, rec.Body.String())
	}

	encoded := "/api/v1/geosite/providers/v2fly/lists/" + url.PathEscape("geolocation-!cn")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, encoded, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"example.org"`) {
		t.Fatalf("special list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/lists/missing", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "list_not_found") {
		t.Fatalf("missing list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/loyalsoldier/catalog", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "provider_not_found") {
		t.Fatalf("unconfigured: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGeositeCatalogRequiresCache(t *testing.T) {
	s := geositeCatalogServer(t)
	if err := os.Remove(filepath.Join(s.DataDir, "geosite", "v2fly.json")); err != nil {
		t.Fatal(err)
	}
	s.Engine.Geosite = geosite.NewManager(filepath.Join(s.DataDir, "geosite"))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/geosite/providers/v2fly/catalog", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "geosite_cache_missing") {
		t.Fatalf("missing cache: %d %s", rec.Code, rec.Body.String())
	}
}

func geositeCatalogServer(t *testing.T) *Server {
	t.Helper()
	s, _ := fileBackedConfigServer(t, nil)
	rec := patchConfig(s.Handler(), `{"version":1,"ops":[{"op":"update_geosite","value":{"providers":[{"name":"v2fly","clients":["surge"]}]}}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update geosite: %d %s", rec.Code, rec.Body.String())
	}
	cacheDir := filepath.Join(s.DataDir, "geosite")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := &geosite.ProviderCache{
		Provider:        "v2fly",
		ResolvedVersion: "test-v1",
		FetchedAt:       "2026-01-01T00:00:00.000Z",
		Catalog:         []string{"ads", "geolocation-!cn", "google"},
		Entries: map[string][]geosite.Entry{
			"google": {
				{Type: geosite.EntryFull, Value: "google.com", Attrs: []string{"cn"}},
				{Type: geosite.EntryDomain, Value: "google.com"},
				{Type: geosite.EntryFull, Value: "youtube.com"},
			},
			"ads": {
				{Type: geosite.EntryKeyword, Value: "doubleclick"},
			},
			"geolocation-!cn": {
				{Type: geosite.EntryDomain, Value: "example.org", Attrs: []string{"ads"}},
			},
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
	return s
}
