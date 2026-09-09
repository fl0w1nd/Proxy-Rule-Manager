package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
)

func TestGeoIPConfigCatalogAndPagination(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	rec := patchConfig(h, `{"version":1,"ops":[{"op":"update_geoip","value":{"providers":[{"name":"loyalsoldier","clients":["surge"]}]}}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	dir := filepath.Join(s.DataDir, "geoip")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cache := &geoip.ProviderCache{Provider: "loyalsoldier", ResolvedVersion: "v1", Catalog: []string{"cn", "private"}, Entries: map[string][]geoip.Entry{
		"cn": {{Type: "ipv4", Value: "1.2.3.0/24"}, {Type: "ipv6", Value: "2001:db8::/32"}}, "private": {{Type: "ipv4", Value: "10.0.0.0/8"}},
	}}
	payload, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loyalsoldier.json"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	s.Engine.GeoIP = geoip.NewManager(dir)
	for _, test := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/geoip/providers", 200, `"name":"loyalsoldier"`},
		{"/geoip/providers/loyalsoldier/catalog?q=1.2.3.4&match=content", 200, `"total":1`},
		{"/geoip/providers/loyalsoldier/catalog?q=private", 200, `"name":"private"`},
		{"/geoip/providers/loyalsoldier/lists/cn?limit=1&offset=1", 200, `"value":"2001:db8::/32"`},
		{"/geoip/providers/loyalsoldier/lists/cn?q=1.2.3.4", 200, `"total":1`},
		{"/geoip/providers/loyalsoldier/lists/cn?offset=100", 200, `"items":[]`},
		{"/geoip/providers/loyalsoldier/lists/missing", 404, `list_not_found`},
		{"/geoip/providers/v2fly/catalog", 404, `provider_not_found`},
		{"/geoip/providers/loyalsoldier/lists/cn?limit=0", 400, `invalid_limit`},
		{"/geoip/providers/loyalsoldier/catalog?match=invalid", 400, `invalid_match`},
	} {
		t.Run(test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1"+test.path, nil))
			if rec.Code != test.status || !strings.Contains(rec.Body.String(), test.contains) {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
		})
	}
	rec = patchConfig(h, `{"version":2,"ops":[{"op":"remove_client","id":"surge"}]}`)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "geoip.providers") {
		t.Fatalf("referenced client removal: %d %s", rec.Code, rec.Body.String())
	}
	rec = patchConfig(h, `{"version":2,"ops":[{"op":"update_geoip","value":null}]}`)
	if rec.Code != 200 || s.config().GeoIP != nil {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
}
