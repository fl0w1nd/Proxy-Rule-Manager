package geosite

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestRefreshWithRetryKeepsSuccessfulAssetDownload(t *testing.T) {
	payload, err := proto.Marshal(&GeoSiteList{Entry: []*GeoSite{{
		CountryCode: "TEST",
		Domains:     []*Domain{{Type: Domain_Domain, Value: "example.com"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x  dlc.dat\n", sha256.Sum256(payload))
	requests := map[string]int{}
	manager := NewManager(t.TempDir())
	manager.SetHTTPClient(&http.Client{Transport: providerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests[req.URL.Path]++
		switch req.URL.Path {
		case "/repos/v2fly/domain-list-community/releases/latest":
			return providerResponse(http.StatusOK, `{
				"tag_name":"test-version",
				"assets":[
					{"name":"dlc.dat","browser_download_url":"https://download.test/dlc.dat"},
					{"name":"dlc.dat.sha256sum","browser_download_url":"https://download.test/dlc.dat.sha256sum"}
				]
			}`), nil
		case "/dlc.dat":
			return providerResponse(http.StatusOK, string(payload)), nil
		case "/dlc.dat.sha256sum":
			if requests[req.URL.Path] == 1 {
				return nil, errors.New("TLS handshake timeout")
			}
			return providerResponse(http.StatusOK, checksum), nil
		default:
			return providerResponse(http.StatusNotFound, "missing"), nil
		}
	})})

	cache, err := manager.RefreshWithRetry(context.Background(), ProviderV2fly, 2, time.Millisecond, nil)
	if err != nil || cache == nil || cache.ResolvedVersion != "test-version" {
		t.Fatalf("cache=%+v err=%v", cache, err)
	}
	if requests["/dlc.dat"] != 1 || requests["/dlc.dat.sha256sum"] != 2 {
		t.Fatalf("request counts = %v", requests)
	}
}

type providerRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn providerRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func providerResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
