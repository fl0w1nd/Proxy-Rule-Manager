package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
	"google.golang.org/protobuf/proto"
)

func TestGeoUpdateRefreshesSelectedDatabaseAndPublications(t *testing.T) {
	for _, kind := range []string{"geosite", "geoip"} {
		for _, outcome := range []string{"success", "failed", "cancelled"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				eng := newTestUpdateEngine(t, &config.Config{
					Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
					Rules: []config.RuleConfig{{ID: "custom", Name: "Custom", Sources: []config.SourceConfig{{
						Content: "DOMAIN,new.example",
					}}, Outputs: []string{"surge"}}},
					Geosite: &config.GeositeConfig{Providers: []config.GeositeProvider{{Name: "v2fly", Clients: []string{"surge"}}}},
					GeoIP:   &config.GeoIPConfig{Providers: []config.GeoIPProvider{{Name: "v2fly", Clients: []string{"surge"}}}},
				})
				checkedAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
				eng.State.SetGeositeUpdate("v2fly", state.ProviderUpdated, checkedAt)
				eng.State.SetGeoIPUpdate("v2fly", state.ProviderUpdated, checkedAt)
				eng.State.SetRuleCheck("custom", state.RuleUpdated, checkedAt, true)
				caches := map[string]any{
					"geosite": &geosite.ProviderCache{Provider: "v2fly", ResolvedVersion: "v1", Catalog: []string{"test"}, Entries: map[string][]geosite.Entry{"test": {{Type: geosite.EntryDomain, Value: "old.example"}}}},
					"geoip":   &geoip.ProviderCache{Provider: "v2fly", ResolvedVersion: "v1", Catalog: []string{"test"}, Entries: map[string][]geoip.Entry{"test": {{Type: "ipv4", Value: "1.1.1.0/24"}}}},
				}
				for cacheKind, cache := range caches {
					payload, err := json.Marshal(cache)
					if err != nil {
						t.Fatal(err)
					}
					if err := util.AtomicWriteFile(filepath.Join(eng.DataDir, cacheKind, "v2fly.json"), payload); err != nil {
						t.Fatal(err)
					}
				}
				eng.Geosite = geosite.NewManager(filepath.Join(eng.DataDir, "geosite"))
				eng.GeoIP = geoip.NewManager(filepath.Join(eng.DataDir, "geoip"))
				otherKind := "geoip"
				asset, repository, wantContent := "dlc.dat", "v2fly/domain-list-community", "DOMAIN-SUFFIX,new.example"
				var database proto.Message = &geosite.GeoSiteList{Entry: []*geosite.GeoSite{{CountryCode: "TEST", Domains: []*geosite.Domain{{Type: geosite.Domain_Domain, Value: "new.example"}}}}}
				if kind == "geoip" {
					otherKind = "geosite"
					asset, repository, wantContent = "geoip.dat", "v2fly/geoip", "IP-CIDR,1.2.3.0/24"
					database = &geoip.GeoIPList{Entry: []*geoip.GeoIP{{CountryCode: "TEST", Cidr: []*geoip.CIDR{{Ip: []byte{1, 2, 3, 0}, Prefix: 24}}}}}
				}
				payload, err := proto.Marshal(database)
				if err != nil {
					t.Fatal(err)
				}
				requests := 0
				client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					status, body := http.StatusOK, ""
					switch req.URL.Path {
					case "/repos/" + repository + "/releases/latest":
						body = fmt.Sprintf(`{"tag_name":"v2","assets":[{"name":%q,"browser_download_url":"https://download.test/%s"},{"name":"%s.sha256sum","browser_download_url":"https://download.test/%s.sha256sum"}]}`, asset, asset, asset, asset)
					case "/" + asset:
						body = string(payload)
					case "/" + asset + ".sha256sum":
						body = fmt.Sprintf("%x", sha256.Sum256(payload))
					default:
						t.Errorf("unexpected request: %s", req.URL)
					}
					if outcome == "failed" {
						status, body = http.StatusNotFound, "missing"
					}
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})}
				eng.Geosite.SetHTTPClient(client)
				eng.GeoIP.SetHTTPClient(client)
				preserved := []string{"surge/custom.list", "surge/" + otherKind + "/v2fly/test.list", "old-client/" + otherKind + "/v2fly/test.list"}
				obsolete := []string{"surge/" + kind + "/v2fly/removed.list", "old-client/" + kind + "/v2fly/test.list"}
				for _, path := range append(append([]string{}, preserved...), obsolete...) {
					if err := util.AtomicWriteFile(filepath.Join(eng.DataDir, "rules", path), []byte("original")); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if outcome == "cancelled" {
					cancel()
				}
				result := eng.GeoUpdate(ctx, kind)
				if result.RulesTotal != 0 || len(result.EffectiveRuleIDs) != 0 || len(result.Changes) != 0 {
					t.Fatalf("unexpected rule updates: %+v", result)
				}
				if outcome == "success" {
					if len(result.Errors) > 0 || result.Artifacts != 1 || requests != 3 {
						t.Fatalf("result=%+v requests=%d", result, requests)
					}
					if content := readArtifact(t, eng.DataDir, "surge", kind+"/v2fly/test.list"); !strings.Contains(content, wantContent) {
						t.Fatalf("publication=%s", content)
					}
					for _, path := range obsolete {
						if _, err := os.Stat(filepath.Join(eng.DataDir, "rules", path)); !os.IsNotExist(err) {
							t.Fatalf("obsolete artifact %s: %v", path, err)
						}
					}
				} else {
					if len(result.Errors) == 0 {
						t.Fatalf("missing %s issue: %+v", outcome, result)
					}
					preserved = append(preserved, obsolete...)
				}
				for _, path := range preserved {
					content, err := os.ReadFile(filepath.Join(eng.DataDir, "rules", path))
					if err != nil || string(content) != "original" {
						t.Fatalf("preserved artifact %s: %s, %v", path, content, err)
					}
				}
				_, ruleCheckedAt, _, _ := eng.State.RuleUpdate("custom")
				if !ruleCheckedAt.Equal(checkedAt) {
					t.Fatalf("rule checked at=%v", ruleCheckedAt)
				}
				_, otherCheckedAt, _ := eng.State.GeoIPUpdate("v2fly")
				if otherKind == "geosite" {
					_, otherCheckedAt, _ = eng.State.GeositeUpdate("v2fly")
				}
				if !otherCheckedAt.Equal(checkedAt) {
					t.Fatalf("%s checked at=%v", otherKind, otherCheckedAt)
				}
			})
		}
	}
}
