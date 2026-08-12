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

func retryRequestTestContext(
	retries int,
	onRetry func(attempt, total int, delay time.Duration, err error),
) context.Context {
	return context.WithValue(context.Background(), providerRetryContextKey{}, providerRetryConfig{
		retries:    retries,
		retryDelay: time.Millisecond,
		onRetry:    onRetry,
	})
}

func TestRetryProviderRequestSucceedsAfterTransientFailures(t *testing.T) {
	attempts := 0
	var retries []int
	ctx := retryRequestTestContext(
		2,
		func(attempt, total int, _ time.Duration, _ error) {
			if total != 2 {
				t.Fatalf("retry total = %d", total)
			}
			retries = append(retries, attempt)
		},
	)
	err := retryProviderRequest(
		ctx,
		func() error {
			attempts++
			if attempts < 3 {
				return markProviderErrorRetryable(errors.New("TLS handshake timeout"))
			}
			return nil
		},
	)
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	if len(retries) != 2 || retries[0] != 1 || retries[1] != 2 {
		t.Fatalf("retry notices = %v", retries)
	}
}

func TestRetryProviderRequestStopsOnPermanentFailure(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return errors.New("release is missing dlc.dat")
		},
	)
	if err == nil || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestReportsExhaustedAttempts(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return markProviderErrorRetryable(errors.New("temporary network failure"))
		},
	)
	if err == nil || attempts != 3 || err.Error() != "after 3 attempts: temporary network failure" {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestHonorsCancellation(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := context.WithValue(baseCtx, providerRetryContextKey{}, providerRetryConfig{
		retries:    2,
		retryDelay: time.Millisecond,
		onRetry: func(_, _ int, _ time.Duration, _ error) {
			cancel()
		},
	})
	attempts := 0
	err := retryProviderRequest(ctx, func() error {
		attempts++
		return markProviderErrorRetryable(errors.New("temporary network failure"))
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

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
