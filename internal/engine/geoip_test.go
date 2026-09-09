package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

func geoIPTestEngine(t *testing.T) *UpdateEngine {
	t.Helper()
	cfg := &config.Config{
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}, {ID: "singbox", Template: "singbox"}, {ID: "mihomo", Template: "mihomo-yaml"}, {ID: "shadowrocket", Template: "shadowrocket"}},
		Rules:   []config.RuleConfig{{ID: "networks", Name: "Networks", Sources: []config.SourceConfig{{GeoIP: "loyalsoldier/cn"}}, Outputs: []string{"surge"}}},
		GeoIP:   &config.GeoIPConfig{Providers: []config.GeoIPProvider{{Name: "loyalsoldier", Clients: []string{"surge", "singbox", "mihomo", "shadowrocket"}}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	dir := filepath.Join(eng.DataDir, "geoip")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cache := &geoip.ProviderCache{Provider: "loyalsoldier", ResolvedVersion: "v1", Catalog: []string{"cn"}, Entries: map[string][]geoip.Entry{"cn": {{Type: "ipv4", Value: "1.2.3.0/24"}, {Type: "ipv6", Value: "2001:db8::/32"}}}}
	payload, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loyalsoldier.json"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	eng.GeoIP = geoip.NewManager(dir)
	return eng
}

func TestGeoIPPartialUsesCacheAndPreservesPublications(t *testing.T) {
	eng := geoIPTestEngine(t)
	eng.GeoIP.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("partial update fetched upstream")
		return nil, context.Canceled
	})})
	checkedAt := time.Now().UTC().Truncate(time.Millisecond)
	eng.State.SetGeoIPUpdate("loyalsoldier", state.ProviderUpdated, checkedAt)
	cache, err := eng.GeoIP.Read("loyalsoldier")
	if err != nil {
		t.Fatal(err)
	}
	publication := UpdateResult{}
	eng.updateGeoIPPublications(context.Background(), map[string]*geoip.ProviderCache{"loyalsoldier": cache}, &publication, map[string]struct{}{})
	if len(publication.Errors) > 0 || publication.Artifacts != 4 {
		t.Fatalf("publication=%+v", publication)
	}
	for _, target := range []struct{ id, ext, ipv6 string }{{"surge", ".list", "IP-CIDR6,2001:db8::/32"}, {"singbox", ".json", "2001:db8::/32"}, {"mihomo", ".yaml", "2001:db8::/32"}, {"shadowrocket", ".list", "2001:db8::/32"}} {
		data, err := os.ReadFile(filepath.Join(eng.DataDir, "rules", target.id, "geoip/loyalsoldier/cn"+target.ext))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "1.2.3.0/24") || !strings.Contains(string(data), target.ipv6) {
			t.Fatalf("%s output: %s", target.id, data)
		}
	}
	result := eng.PartialUpdate(context.Background(), []string{"networks"})
	if len(result.Errors) > 0 || result.RulesSucceeded != 1 {
		t.Fatalf("partial=%+v", result)
	}
	status, at, _ := eng.State.GeoIPUpdate("loyalsoldier")
	if status != state.ProviderUpdated || !at.Equal(checkedAt) {
		t.Fatalf("provider state changed: %s %s", status, at)
	}
	if _, err := os.Stat(filepath.Join(eng.DataDir, "rules/surge/geoip/loyalsoldier/cn.list")); err != nil {
		t.Fatal(err)
	}
	catalog := eng.rebuildGeoIPStats().catalog()
	if len(catalog) != 1 || len(catalog[0].Lists) != 1 || catalog[0].Lists[0].Entries != 2 {
		t.Fatalf("catalog=%+v", catalog)
	}
	if !eng.siteClients()[0].GeoIP {
		t.Fatal("client missing geoip capability")
	}
}

func TestGeoIPRefreshFailureUsesCacheAndReportsIssue(t *testing.T) {
	eng := geoIPTestEngine(t)
	eng.GeoIP.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("missing")), Header: make(http.Header)}, nil
	})})
	result := eng.FullUpdate(context.Background())
	if result.RulesSucceeded != 1 || len(result.Errors) == 0 {
		t.Fatalf("result=%+v", result)
	}
	status, _, _ := eng.State.GeoIPUpdate("loyalsoldier")
	if status != state.ProviderFailed {
		t.Fatalf("status=%s", status)
	}
	if len(result.Issues) == 0 || result.Issues[0].Stage != "geoip_refresh" {
		t.Fatalf("issues=%v", result.Issues)
	}
	if _, err := os.Stat(filepath.Join(eng.DataDir, "rules/surge/geoip/loyalsoldier/cn.list")); err != nil {
		t.Fatal(err)
	}
}

func TestGeoIPEmptyVariantOmittedFromSite(t *testing.T) {
	cfg := &config.Config{
		Clients: []config.ClientConfig{
			{ID: "surge", Template: "surge"},
			{ID: "singbox", Template: "singbox", Variants: []config.ClientVariantConfig{
				{ID: "singbox-non-ip", Name: "Non-IP", Ops: []config.OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}}},
			}},
		},
		GeoIP: &config.GeoIPConfig{Providers: []config.GeoIPProvider{{Name: "loyalsoldier", Clients: []string{"surge", "singbox"}}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	dir := filepath.Join(eng.DataDir, "geoip")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cache := &geoip.ProviderCache{Provider: "loyalsoldier", ResolvedVersion: "v1", Catalog: []string{"cn"}, Entries: map[string][]geoip.Entry{"cn": {{Type: "ipv4", Value: "1.2.3.0/24"}}}}
	payload, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loyalsoldier.json"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	eng.GeoIP = geoip.NewManager(dir)
	loaded, err := eng.GeoIP.Read("loyalsoldier")
	if err != nil {
		t.Fatal(err)
	}
	publication := UpdateResult{}
	eng.updateGeoIPPublications(context.Background(), map[string]*geoip.ProviderCache{"loyalsoldier": loaded}, &publication, map[string]struct{}{})
	if len(publication.Errors) > 0 {
		t.Fatalf("publication=%+v", publication)
	}
	if _, err := os.Stat(filepath.Join(eng.DataDir, "rules/surge/geoip/loyalsoldier/cn.list")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(eng.DataDir, "rules/singbox-non-ip/geoip/loyalsoldier/cn.json")); !os.IsNotExist(err) {
		t.Fatal("empty variant still published")
	}
	clients := eng.siteClients()
	if len(clients) != 2 || !clients[0].GeoIP || !clients[1].GeoIP {
		t.Fatalf("clients=%+v", clients)
	}
	if !clients[1].Options[0].GeoIP || clients[1].Options[1].GeoIP {
		t.Fatalf("singbox options=%+v", clients[1].Options)
	}
	catalog := eng.publishedGeoCatalog("geoip", eng.rebuildGeoIPStats().catalog())
	if len(catalog) != 1 || len(catalog[0].Lists) != 1 {
		t.Fatalf("catalog=%+v", catalog)
	}
	targets := catalog[0].Lists[0].Targets
	if len(targets) != 2 || !containsString(targets, "surge") || !containsString(targets, "singbox") || containsString(targets, "singbox-non-ip") {
		t.Fatalf("targets=%v", targets)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
