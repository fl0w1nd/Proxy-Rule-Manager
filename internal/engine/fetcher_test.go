package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetcherBoundsGlobalConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan struct{}, 5)
	release := make(chan struct{}, 5)

	fetcher := NewFetcher()
	fetcher.Configure(time.Second, 1024, 2, 2, 0, time.Millisecond, "test")
	fetcher.Client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		current := active.Add(1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return response(http.StatusOK, "example.com"), nil
	})}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result := fetcher.Fetch(context.Background(), fmt.Sprintf("https://host-%d.example/rules", index))
			if result.Error != "" {
				t.Errorf("fetch %d: %s", index, result.Error)
			}
		}(i)
	}
	waitForStarts(t, started, 2)
	for i := 0; i < 5; i++ {
		release <- struct{}{}
	}
	wg.Wait()
	if maximum.Load() != 2 {
		t.Fatalf("maximum concurrent requests = %d, want 2", maximum.Load())
	}
}

func TestFetcherBoundsPerHostConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{}, 4)

	fetcher := NewFetcher()
	fetcher.Configure(time.Second, 1024, 4, 2, 0, time.Millisecond, "test")
	fetcher.Client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		current := active.Add(1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return response(http.StatusOK, "example.com"), nil
	})}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result := fetcher.Fetch(context.Background(), fmt.Sprintf("https://same.example/rules/%d", index))
			if result.Error != "" {
				t.Errorf("fetch %d: %s", index, result.Error)
			}
		}(i)
	}
	waitForStarts(t, started, 2)
	for i := 0; i < 4; i++ {
		release <- struct{}{}
	}
	wg.Wait()
	if maximum.Load() != 2 {
		t.Fatalf("maximum per-host requests = %d, want 2", maximum.Load())
	}
}

func TestFetcherRetriesTransientFailures(t *testing.T) {
	var attempts atomic.Int32
	fetcher := NewFetcher()
	fetcher.Configure(time.Second, 1024, 1, 1, 2, time.Millisecond, "test-agent")
	fetcher.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("user agent = %q", req.Header.Get("User-Agent"))
		}
		if attempts.Add(1) < 3 {
			return response(http.StatusServiceUnavailable, "retry"), nil
		}
		return response(http.StatusOK, "DOMAIN,example.com"), nil
	})}

	result := fetcher.Fetch(context.Background(), "https://retry.example/rules")
	if result.Error != "" || result.Content != "DOMAIN,example.com" || attempts.Load() != 3 {
		t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
	}
}

func TestFetcherSkipsRetryForPermanentAndOversizeFailures(t *testing.T) {
	t.Run("permanent status", func(t *testing.T) {
		var attempts atomic.Int32
		fetcher := NewFetcher()
		fetcher.Configure(time.Second, 1024, 1, 1, 3, time.Millisecond, "test")
		fetcher.Client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts.Add(1)
			return response(http.StatusNotFound, "missing"), nil
		})}

		result := fetcher.Fetch(context.Background(), "https://missing.example/rules")
		if !strings.Contains(result.Error, "HTTP 404") || attempts.Load() != 1 {
			t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
		}
	})

	t.Run("stream exceeds maximum", func(t *testing.T) {
		var attempts atomic.Int32
		fetcher := NewFetcher()
		fetcher.Configure(time.Second, 4, 1, 1, 3, time.Millisecond, "test")
		fetcher.Client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts.Add(1)
			resp := response(http.StatusOK, "12345")
			resp.ContentLength = -1
			return resp, nil
		})}

		result := fetcher.Fetch(context.Background(), "https://large.example/rules")
		if !strings.Contains(result.Error, "content too large") || attempts.Load() != 1 {
			t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
		}
	})
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}
}

func waitForStarts(t *testing.T, started <-chan struct{}, count int) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case <-started:
		case <-timer.C:
			t.Fatalf("timed out waiting for %d concurrent requests", count)
		}
	}
}
