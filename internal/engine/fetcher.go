// Package engine orchestrates rule updates: fetch, parse, ops, merge,
// render, and artifact publishing.
package engine

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	defaultMaxDownloadSize = 4 * 1024 * 1024
	defaultFetchTimeout    = 15 * time.Second
	defaultGlobalLimit     = 4
	defaultHostLimit       = 2
	defaultRetryDelay      = 500 * time.Millisecond
	defaultUserAgent       = "Proxy-Rule-Manager/2.0"
	maxBackoff             = 30 * time.Second
)

// Fetcher fetches rule sources with SSRF, size, concurrency and retry guards.
type Fetcher struct {
	Client *http.Client

	cfgMu       sync.RWMutex
	timeout     time.Duration
	maxBytes    int64
	globalLimit int
	hostLimit   int
	retries     int
	retryDelay  time.Duration
	userAgent   string
	globalSem   chan struct{}
	hostSemsMu  sync.Mutex
	hostSems    map[string]chan struct{}
}

// NewFetcher constructs a fetcher with sane defaults.
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
		timeout:     defaultFetchTimeout,
		maxBytes:    defaultMaxDownloadSize,
		globalLimit: defaultGlobalLimit,
		hostLimit:   defaultHostLimit,
		retryDelay:  defaultRetryDelay,
		userAgent:   defaultUserAgent,
		globalSem:   make(chan struct{}, defaultGlobalLimit),
		hostSems:    make(map[string]chan struct{}),
	}
}

// Configure swaps in new fetch tunables.
func (f *Fetcher) Configure(
	timeout time.Duration,
	maxBytes int64,
	globalLimit int,
	hostLimit int,
	retries int,
	retryDelay time.Duration,
	userAgent string,
) {
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxDownloadSize
	}
	if globalLimit <= 0 {
		globalLimit = defaultGlobalLimit
	}
	if globalLimit > 64 {
		globalLimit = 64
	}
	if hostLimit <= 0 || hostLimit > globalLimit {
		hostLimit = defaultHostLimit
		if hostLimit > globalLimit {
			hostLimit = globalLimit
		}
	}
	if retries < 0 {
		retries = 0
	}
	if retries > 10 {
		retries = 10
	}
	if retryDelay <= 0 {
		retryDelay = defaultRetryDelay
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	f.cfgMu.Lock()
	defer f.cfgMu.Unlock()
	f.timeout = timeout
	f.maxBytes = maxBytes
	f.retries = retries
	f.retryDelay = retryDelay
	f.userAgent = userAgent
	if globalLimit != f.globalLimit {
		f.globalLimit = globalLimit
		f.globalSem = make(chan struct{}, globalLimit)
	}
	if hostLimit != f.hostLimit {
		f.hostLimit = hostLimit
		f.hostSemsMu.Lock()
		f.hostSems = make(map[string]chan struct{})
		f.hostSemsMu.Unlock()
	}
}

func (f *Fetcher) snapshot() (time.Duration, int64, int, time.Duration, string) {
	f.cfgMu.RLock()
	defer f.cfgMu.RUnlock()
	return f.timeout, f.maxBytes, f.retries, f.retryDelay, f.userAgent
}

// FetchResult holds the outcome of one URL fetch.
type FetchResult struct {
	Content string
	Error   string
}

// Fetch downloads url, applying SSRF + size + timeout protections.
func (f *Fetcher) Fetch(ctx context.Context, raw string) FetchResult {
	if !util.IsURLSafe(raw) {
		return FetchResult{Error: "unsafe URL: " + raw}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return FetchResult{Error: err.Error()}
	}
	timeout, maxBytes, retries, retryDelay, userAgent := f.snapshot()
	var lastErr string
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			delay := retryDelay * time.Duration(1<<(attempt-1))
			if delay > maxBackoff {
				delay = maxBackoff
			}
			// ±25% jitter to avoid thundering herd on concurrent retries.
			jitter := time.Duration(rand.Int63n(int64(delay/4) + 1))
			if rand.Intn(2) == 0 {
				delay -= jitter
			} else {
				delay += jitter
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return FetchResult{Error: ctx.Err().Error()}
			case <-timer.C:
			}
		}

		content, retryable, errText := f.fetchOnce(ctx, raw, parsed.Host, timeout, maxBytes, userAgent)
		if errText == "" {
			return FetchResult{Content: content}
		}
		lastErr = errText
		if !retryable {
			return FetchResult{Error: errText}
		}
	}
	return FetchResult{Error: fmt.Sprintf("after %d attempts: %s", retries+1, lastErr)}
}

func (f *Fetcher) fetchOnce(
	ctx context.Context,
	raw string,
	host string,
	timeout time.Duration,
	maxBytes int64,
	userAgent string,
) (content string, retryable bool, errText string) {
	hostSem := f.acquireHost(host)
	if !acquireFetchSlot(ctx, hostSem) {
		return "", false, ctx.Err().Error()
	}
	defer releaseFetchSlot(hostSem)

	globalSem := f.acquireGlobal()
	if !acquireFetchSlot(ctx, globalSem) {
		return "", false, ctx.Err().Error()
	}
	defer releaseFetchSlot(globalSem)

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, raw, nil)
	if err != nil {
		return "", false, err.Error()
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", ctx.Err() == nil, err.Error()
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return "", retryableStatus(resp.StatusCode), fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	if cl := resp.ContentLength; cl > maxBytes {
		_ = resp.Body.Close()
		return "", false, fmt.Sprintf("content too large (%d bytes > %d)", cl, maxBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		return "", true, err.Error()
	}
	if int64(len(body)) > maxBytes {
		return "", false, fmt.Sprintf("content too large (> %d bytes)", maxBytes)
	}
	return string(body), false, ""
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func acquireFetchSlot(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func releaseFetchSlot(sem chan struct{}) { <-sem }

func (f *Fetcher) acquireGlobal() chan struct{} {
	f.cfgMu.RLock()
	defer f.cfgMu.RUnlock()
	return f.globalSem
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
