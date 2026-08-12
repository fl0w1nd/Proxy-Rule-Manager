package geosite

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestDecodeGeoSiteListRoundTrip(t *testing.T) {
	original := &GeoSiteList{
		Entry: []*GeoSite{
			{
				CountryCode: "google",
				Domains: []*Domain{
					{Type: Domain_Full, Value: "google.com", Attribute: []*DomainAttribute{{Key: "ads"}}},
					{Type: Domain_Domain, Value: "google"},
					{Type: Domain_Regex, Value: `^ads\..*`},
					{Type: Domain_Plain, Value: "keyword"},
				},
			},
			{
				CountryCode: "cn",
				Domains: []*Domain{
					{Type: Domain_Full, Value: "baidu.com"},
				},
			},
		},
	}
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := decodeGeoSiteList(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Entry) != 2 {
		t.Fatalf("entries: %d, want 2", len(decoded.Entry))
	}
	site := decoded.Entry[0]
	if site.CountryCode != "google" {
		t.Errorf("country code: %q, want google", site.CountryCode)
	}
	if len(site.Domains) != 4 {
		t.Fatalf("domains: %d, want 4", len(site.Domains))
	}
	if site.Domains[0].Value != "google.com" || site.Domains[0].Type != Domain_Full {
		t.Errorf("domain 0: type=%s value=%q", site.Domains[0].Type, site.Domains[0].Value)
	}
	if len(site.Domains[0].Attribute) != 1 || site.Domains[0].Attribute[0].Key != "ads" {
		t.Errorf("domain 0 attributes: %+v", site.Domains[0].Attribute)
	}
	if site.Domains[1].Type != Domain_Domain || site.Domains[1].Value != "google" {
		t.Errorf("domain 1: type=%s value=%q", site.Domains[1].Type, site.Domains[1].Value)
	}
}

func TestDecodeGeoSiteListCorrupt(t *testing.T) {
	_, err := decodeGeoSiteList([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	if err == nil {
		t.Fatal("expected error for corrupt protobuf data")
	}
}

func TestDecodeGeoSiteListEmpty(t *testing.T) {
	decoded, err := decodeGeoSiteList(nil)
	if err != nil {
		t.Fatalf("decode nil: %v", err)
	}
	if decoded == nil || len(decoded.Entry) != 0 {
		t.Fatalf("expected empty list, got %+v", decoded)
	}
}
