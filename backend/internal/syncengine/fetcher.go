// Package syncengine orchestrates rule synchronization.
package syncengine

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// Built-in defaults; admins can override via SystemSettings.
const (
	defaultMaxDownloadSize = 4 * 1024 * 1024
	defaultFetchTimeout    = 15 * time.Second
	defaultHostLimit       = 4
	defaultUserAgent       = "Proxy-Rule-Manager/1.0"
)

// MaxDownloadSize is the historical hard limit; kept exported so external
// tests can refer to it. Runtime requests use Fetcher's mutable cap instead.
const MaxDownloadSize = defaultMaxDownloadSize

// FetchTimeout is the historical hard limit kept exported for the same reason.
const FetchTimeout = defaultFetchTimeout

// Fetcher fetches rule sources with SSRF + size guards and per-host limits.
// All tunables live behind cfgMu so an admin update is observed by the next
// Fetch without requiring a server restart.
type Fetcher struct {
	Client *http.Client

	cfgMu      sync.RWMutex
	timeout    time.Duration
	maxBytes   int64
	hostLimit  int
	userAgent  string
	hostSemsMu sync.Mutex
	hostSems   map[string]chan struct{}
}

// NewFetcher constructs a fetcher with sane defaults.
//
// The HTTP transport uses a SSRF-aware DialContext so every dial (including
// after a redirect) validates the resolved IPs against the deny-list. The
// CheckRedirect callback additionally re-runs IsURLSafe on each hop so we
// reject redirects to non-HTTPS or denied hostnames before even attempting
// to dial.
func NewFetcher() *Fetcher {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext:           util.SafeDialContext(nil, dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Fetcher{
		// Timeout is enforced via per-request context; the http.Client
		// timeout is left generous so changing the runtime knob doesn't
		// require recreating the client.
		Client: &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 20 {
					return http.ErrUseLastResponse
				}
				if !util.IsURLSafe(req.URL.String()) {
					return fmt.Errorf("%w: redirect to %s", util.ErrUnsafeURL, req.URL.Redacted())
				}
				return nil
			},
		},
		timeout:   defaultFetchTimeout,
		maxBytes:  defaultMaxDownloadSize,
		hostLimit: defaultHostLimit,
		userAgent: defaultUserAgent,
		hostSems:  make(map[string]chan struct{}),
	}
}

// Configure swaps in new fetch tunables. Existing per-host semaphores keep
// their current capacity until the next first-use of that host (acceptable
// trade-off: simpler than resizing live channels and converges within minutes).
func (f *Fetcher) Configure(timeout time.Duration, maxBytes int64, hostLimit int, userAgent string) {
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxDownloadSize
	}
	if hostLimit <= 0 {
		hostLimit = defaultHostLimit
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	f.cfgMu.Lock()
	defer f.cfgMu.Unlock()
	f.timeout = timeout
	f.maxBytes = maxBytes
	f.userAgent = userAgent
	if hostLimit != f.hostLimit {
		f.hostLimit = hostLimit
		// Reset the semaphore registry; any in-flight fetch keeps using
		// its previously acquired slot, the next first-use of each host
		// rebuilds a slot with the new capacity.
		f.hostSemsMu.Lock()
		f.hostSems = make(map[string]chan struct{})
		f.hostSemsMu.Unlock()
	}
}

func (f *Fetcher) snapshot() (time.Duration, int64, string) {
	f.cfgMu.RLock()
	defer f.cfgMu.RUnlock()
	return f.timeout, f.maxBytes, f.userAgent
}

// HostLimit returns the current per-host concurrency cap. Exposed as a getter
// so callers (incl. tests) can introspect without taking the mutex themselves.
func (f *Fetcher) HostLimit() int {
	f.cfgMu.RLock()
	defer f.cfgMu.RUnlock()
	return f.hostLimit
}

// FetchResult mirrors the TS return shape.
type FetchResult struct {
	Content string
	Error   string
}

// Fetch downloads url, applying SSRF + size + timeout protections.
func (f *Fetcher) Fetch(ctx context.Context, raw string) FetchResult {
	if !util.IsURLSafe(raw) {
		return FetchResult{Error: "Unsafe URL: " + raw}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return FetchResult{Error: err.Error()}
	}
	host := parsed.Host
	sem := f.acquireHost(host)
	sem <- struct{}{}
	defer func() { <-sem }()

	timeout, maxBytes, userAgent := f.snapshot()
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, raw, nil)
	if err != nil {
		return FetchResult{Error: err.Error()}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.Client.Do(req)
	if err != nil {
		return FetchResult{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FetchResult{Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)}
	}
	if cl := resp.ContentLength; cl > maxBytes {
		return FetchResult{Error: fmt.Sprintf("Content too large (%d bytes > %d)", cl, maxBytes)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return FetchResult{Error: err.Error()}
	}
	if int64(len(body)) > maxBytes {
		return FetchResult{Error: fmt.Sprintf("Content too large (> %d bytes)", maxBytes)}
	}
	return FetchResult{Content: string(body)}
}

func (f *Fetcher) acquireHost(host string) chan struct{} {
	f.cfgMu.RLock()
	limit := f.hostLimit
	f.cfgMu.RUnlock()
	f.hostSemsMu.Lock()
	defer f.hostSemsMu.Unlock()
	if sem, ok := f.hostSems[host]; ok {
		return sem
	}
	sem := make(chan struct{}, limit)
	f.hostSems[host] = sem
	return sem
}
