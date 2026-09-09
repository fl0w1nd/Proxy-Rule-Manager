package geoip

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"google.golang.org/protobuf/proto"
)

func fixtureData(t *testing.T) []byte {
	t.Helper()
	payload, err := proto.Marshal(&GeoIPList{Entry: []*GeoIP{
		{CountryCode: "CN", Cidr: []*CIDR{
			{Ip: []byte{1, 2, 3, 4}, Prefix: 24}, {Ip: []byte{1, 2, 3, 0}, Prefix: 24},
			{Ip: netip.MustParseAddr("2001:db8::1").AsSlice(), Prefix: 32},
		}},
		{CountryCode: "private", Cidr: []*CIDR{{Ip: []byte{10, 0, 0, 0}, Prefix: 8}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestDecodeAndSearch(t *testing.T) {
	cache, err := decode(fixtureData(t), "v2fly", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.Catalog) != 2 || len(cache.Entries["cn"]) != 2 {
		t.Fatalf("unexpected catalog: %+v", cache)
	}
	entries, err := ResolveIR(cache, "cn")
	if err != nil || len(entries) != 2 || entries[0].Kind != ir.KindIPCIDR || entries[1].Kind != ir.KindIPCIDR {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if entries[0].Value != "1.2.3.0/24" || entries[1].Value != "2001:db8::/32" {
		t.Fatalf("CIDRs=%v", entries)
	}
	for _, query := range []string{"1.2.3.4", "1.2.0.0/16", "2001:db8::123"} {
		lists := ListOverviews(cache, query, "content")
		if len(lists) != 1 || lists[0].Name != "cn" {
			t.Fatalf("query=%s lists=%v", query, lists)
		}
	}
	if lists := ListOverviews(cache, "192.0.2.1", "content"); len(lists) != 0 {
		t.Fatalf("unexpected match: %v", lists)
	}
	if _, err := ResolveEntries(cache, "missing"); err == nil {
		t.Fatal("missing list accepted")
	}
}

func TestRejectInvalidDat(t *testing.T) {
	for name, data := range map[string]*GeoIPList{
		"prefix":  {Entry: []*GeoIP{{CountryCode: "cn", Cidr: []*CIDR{{Ip: []byte{1, 2, 3, 4}, Prefix: 33}}}}},
		"address": {Entry: []*GeoIP{{CountryCode: "cn", Cidr: []*CIDR{{Ip: []byte{1, 2, 3}, Prefix: 24}}}}},
		"reverse": {Entry: []*GeoIP{{CountryCode: "cn", ReverseMatch: true}}},
		"path":    {Entry: []*GeoIP{{CountryCode: "../cn"}}},
		"empty":   {},
	} {
		t.Run(name, func(t *testing.T) {
			payload, err := proto.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = decode(payload, "v2fly", "v1"); err == nil {
				t.Fatal("invalid database accepted")
			}
		})
	}
	if _, err := decode([]byte{0xff}, "v2fly", "v1"); err == nil {
		t.Fatal("corrupt protobuf accepted")
	}
	for _, ref := range []string{"v2fly/cn@ipv4", "other/cn", "v2fly/", "v2fly/../cn"} {
		if _, err := ParseRef(ref); err == nil {
			t.Fatalf("invalid ref accepted: %s", ref)
		}
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestProvidersVerifyChecksumAndPreserveCache(t *testing.T) {
	for _, provider := range SupportedProviders {
		t.Run(provider, func(t *testing.T) {
			dir := t.TempDir()
			manager := NewManager(dir)
			payload := fixtureData(t)
			corrupt := false
			manager.SetHTTPClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				body := ""
				switch {
				case strings.HasSuffix(r.URL.Path, "/releases/latest"):
					expected := "/repos/v2fly/geoip/releases/latest"
					if provider == "loyalsoldier" {
						expected = "/repos/Loyalsoldier/geoip/releases/latest"
					}
					if r.URL.Path != expected {
						t.Errorf("unexpected repository: %s", r.URL.Path)
					}
					body = `{"tag_name":"v1","assets":[{"name":"geoip.dat","browser_download_url":"https://fixture.test/geoip.dat"},{"name":"geoip.dat.sha256sum","browser_download_url":"https://fixture.test/geoip.dat.sha256sum"}]}`
				case strings.HasSuffix(r.URL.Path, ".sha256sum"):
					body = fmt.Sprintf("%x  geoip.dat", sha256.Sum256(payload))
					if corrupt {
						body = strings.Repeat("0", 64)
					}
				default:
					body = string(payload)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})})
			cache, err := manager.Refresh(context.Background(), provider)
			if err != nil {
				t.Fatal(err)
			}
			disk, err := NewManager(dir).Read(provider)
			if err != nil || disk == nil || len(disk.Catalog) != 2 {
				t.Fatalf("cache reload=%v err=%v", disk, err)
			}
			corrupt = true
			if _, err := manager.Refresh(context.Background(), provider); err == nil {
				t.Fatal("checksum mismatch accepted")
			}
			saved, err := manager.Read(provider)
			if err != nil || saved != cache {
				t.Fatal("failed refresh replaced cached data")
			}
		})
	}
}
