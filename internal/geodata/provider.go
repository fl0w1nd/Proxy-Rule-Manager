package geodata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	defaultUserAgent       = "Proxy-Rule-Manager/2.0"
	defaultFetchTimeout    = 15 * time.Second
	defaultMaxDownloadSize = 50 * 1024 * 1024
	githubAcceptJSON       = "application/vnd.github+json"
)

// refreshState coordinates concurrent Refresh calls for the same provider.
// The first caller stores its result so waiters receive the same error
// instead of a silent nil.
type refreshState[E any] struct {
	done  chan struct{}
	cache *Cache[E]
	err   error
}

type retryableProviderError struct{ err error }

func (e *retryableProviderError) Error() string { return e.err.Error() }
func (e *retryableProviderError) Unwrap() error { return e.err }

func markProviderErrorRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &retryableProviderError{err: err}
}

func isRetryableProviderError(err error) bool {
	var retryable *retryableProviderError
	return errors.As(err, &retryable)
}

type providerRetryContextKey struct{}

type providerRetryConfig struct {
	retries    int
	retryDelay time.Duration
	onRetry    func(attempt, total int, delay time.Duration, err error)
}

// Manager coordinates provider caches between disk and memory.
type Manager[E any] struct {
	kind       string
	sources    map[string]Source
	decode     func([]byte, string, string) (*Cache[E], error)
	onWrite    func(*Cache[E])
	dir        string
	mu         sync.RWMutex
	memCache   map[string]*Cache[E]
	refresh    map[string]*refreshState[E]
	httpClient *http.Client
	timeout    time.Duration
	maxBytes   int64
	userAgent  string
}

// NewManager constructs a manager that persists caches under `dir`.
func NewManager[E any](dir, kind string, sources map[string]Source, decode func([]byte, string, string) (*Cache[E], error), onWrite func(*Cache[E])) *Manager[E] {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Manager[E]{
		dir:        dir,
		kind:       kind,
		sources:    sources,
		decode:     decode,
		onWrite:    onWrite,
		memCache:   make(map[string]*Cache[E]),
		refresh:    make(map[string]*refreshState[E]),
		httpClient: &http.Client{Transport: transport},
		timeout:    defaultFetchTimeout,
		maxBytes:   defaultMaxDownloadSize,
		userAgent:  defaultUserAgent,
	}
}

// Configure updates the network fetch parameters.
func (m *Manager[E]) Configure(timeout time.Duration, maxBytes int64, userAgent string) {
	if m == nil {
		return
	}
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxDownloadSize
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeout = timeout
	m.maxBytes = maxBytes
	m.userAgent = userAgent
}

func (m *Manager[E]) snapshot() (time.Duration, int64, string, *http.Client) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timeout, m.maxBytes, m.userAgent, m.httpClient
}

// SetHTTPClient replaces the HTTP client used for upstream fetches.
// Intended for tests that need to inject a mock transport.
func (m *Manager[E]) SetHTTPClient(c *http.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.httpClient = c
}

// Read returns the cached provider data, loading from disk if necessary.
func (m *Manager[E]) Read(provider string) (*Cache[E], error) {
	m.mu.RLock()
	cached, ok := m.memCache[provider]
	m.mu.RUnlock()
	if ok {
		return cached, nil
	}
	filePath, err := m.cachePath(provider)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cache Cache[E]
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.memCache[provider] = &cache
	m.mu.Unlock()
	return &cache, nil
}

// Refresh re-fetches the provider data from the upstream source.
func (m *Manager[E]) Refresh(ctx context.Context, provider string) (*Cache[E], error) {
	m.mu.Lock()
	if state, ok := m.refresh[provider]; ok {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-state.done:
			return state.cache, state.err
		}
	}
	state := &refreshState[E]{done: make(chan struct{})}
	m.refresh[provider] = state
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.refresh, provider)
		m.mu.Unlock()
		close(state.done)
	}()

	cache, err := m.download(ctx, provider)
	if err != nil {
		state.err = err
		return nil, err
	}
	if previous, _ := m.Read(provider); previous == cache {
		state.cache = cache
		return cache, nil
	}
	if err := m.write(cache); err != nil {
		state.err = err
		return nil, err
	}
	state.cache = cache
	return cache, nil
}

// RefreshWithRetry retries transient HTTP requests during a provider refresh
// using exponential backoff. onRetry receives a one-based retry number and
// the configured total.
func (m *Manager[E]) RefreshWithRetry(
	ctx context.Context,
	provider string,
	retries int,
	retryDelay time.Duration,
	onRetry func(attempt, total int, delay time.Duration, err error),
) (*Cache[E], error) {
	if retries < 0 {
		retries = 0
	}
	if retryDelay <= 0 {
		retryDelay = 500 * time.Millisecond
	}
	ctx = context.WithValue(ctx, providerRetryContextKey{}, providerRetryConfig{
		retries: retries, retryDelay: retryDelay, onRetry: onRetry,
	})
	return m.Refresh(ctx, provider)
}

func retryProviderRequest(
	ctx context.Context,
	request func() error,
) error {
	cfg, _ := ctx.Value(providerRetryContextKey{}).(providerRetryConfig)
	var lastErr error
	for attempt := 0; attempt <= cfg.retries; attempt++ {
		err := request()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt >= cfg.retries || !isRetryableProviderError(err) || ctx.Err() != nil {
			break
		}
		delay := cfg.retryDelay * time.Duration(1<<attempt)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		if cfg.onRetry != nil {
			cfg.onRetry(attempt+1, cfg.retries, delay, err)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if cfg.retries > 0 && isRetryableProviderError(lastErr) {
		return fmt.Errorf("after %d attempts: %w", cfg.retries+1, lastErr)
	}
	return lastErr
}

func (m *Manager[E]) write(cache *Cache[E]) error {
	if cache == nil {
		return nil
	}
	if err := util.EnsureDir(m.dir); err != nil {
		return err
	}
	payload, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	path, err := m.cachePath(cache.Provider)
	if err != nil {
		return err
	}
	if err := util.AtomicWriteFile(path, payload); err != nil {
		return err
	}
	m.mu.Lock()
	m.memCache[cache.Provider] = cache
	m.mu.Unlock()
	if m.onWrite != nil {
		m.onWrite(cache)
	}
	return nil
}

func (m *Manager[E]) cachePath(provider string) (string, error) {
	if err := util.EnsureSafeSegment(provider, m.kind+" provider"); err != nil {
		return "", err
	}
	return util.JoinInside(m.dir, provider+".json")
}

// Source returns the upstream asset descriptor for a provider.
func (m *Manager[E]) Source(provider string) (Source, bool) {
	source, ok := m.sources[provider]
	return source, ok
}

// RawPath is the on-disk original asset downloaded for a provider.
func (m *Manager[E]) RawPath(provider string) (string, error) {
	source, ok := m.sources[provider]
	if !ok {
		return "", fmt.Errorf("unsupported %s provider: %s", m.kind, provider)
	}
	if err := util.EnsureSafeSegment(provider, m.kind+" provider"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(source.Asset, m.kind+" asset"); err != nil {
		return "", err
	}
	return util.JoinInside(m.dir, "raw", provider, source.Asset)
}

func (m *Manager[E]) writeRaw(provider string, payload []byte) error {
	path, err := m.RawPath(provider)
	if err != nil {
		return err
	}
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return util.AtomicWriteFile(path, payload)
}

// ListStatus returns each supported provider in name order.
func (m *Manager[E]) ListStatus() []Status {
	out := make([]Status, 0, len(m.sources))
	for p := range m.sources {
		cache, _ := m.Read(p)
		status := Status{Provider: p}
		if cache != nil {
			status.Ready = true
			fetched := cache.FetchedAt
			status.FetchedAt = &fetched
			resolved := cache.ResolvedVersion
			status.ResolvedVersion = &resolved
			status.CatalogCount = len(cache.Catalog)
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

// ---- Refresh implementations ----

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		Digest             string `json:"digest"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r *githubRelease) assetURL(name string) string {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func (m *Manager[E]) latestRelease(ctx context.Context, repository string) (*githubRelease, error) {
	var release githubRelease
	if err := m.fetchJSON(ctx, "https://api.github.com/repos/"+repository+"/releases/latest", &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (m *Manager[E]) download(ctx context.Context, provider string) (*Cache[E], error) {
	source, ok := m.sources[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported %s provider: %s", m.kind, provider)
	}
	headers := map[string]string{"Accept": "application/octet-stream"}
	version, digest := "", ""
	assetURL := "https://github.com/" + source.Repository + "/releases/latest/download/" + source.Asset
	checksumURL := assetURL + ".sha256sum"
	if source.Ref != "" {
		assetURL = "https://raw.githubusercontent.com/" + source.Repository + "/" + source.Ref + "/" + source.Asset
		checksumURL = assetURL + ".sha256sum"
	} else if release, err := m.latestRelease(ctx, source.Repository); err == nil {
		version = release.TagName
		if url := release.assetURL(source.Asset); url != "" {
			assetURL = url
		}
		if url := release.assetURL(source.Asset + ".sha256sum"); url != "" {
			checksumURL = url
		}
		for _, asset := range release.Assets {
			if asset.Name == source.Asset {
				digest = parseSHA256(strings.TrimPrefix(asset.Digest, "sha256:"))
			}
		}
	}
	if source.Checksum && digest == "" {
		if checksum, err := m.fetchBytes(ctx, checksumURL, headers); err == nil {
			digest = parseSHA256(string(checksum))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	previous, _ := m.Read(provider)
	// Verify the local original as well as its parsed cache before reusing it.
	rawPath, err := m.RawPath(provider)
	if err != nil {
		return nil, err
	}
	local, localErr := os.ReadFile(rawPath)
	localDigest := fmt.Sprintf("%x", sha256.Sum256(local))
	reusable := localErr == nil && previous != nil && previous.SHA256 != "" && previous.SHA256 == localDigest
	if digest != "" && reusable && digest == previous.SHA256 {
		return previous, nil
	}
	payload, err := m.fetchBytes(ctx, assetURL, headers)
	if err != nil {
		return nil, err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(payload))
	if digest != "" && actual != digest {
		return nil, fmt.Errorf("checksum mismatch: got %s, want %s", actual, digest)
	}
	if previous != nil && previous.SHA256 == actual {
		if !reusable {
			if err := m.writeRaw(provider, payload); err != nil {
				return nil, err
			}
		}
		return previous, nil
	}
	if version == "" {
		version = actual[:12]
	}
	cache, err := m.decode(payload, provider, version)
	if err != nil {
		return nil, err
	}
	cache.SHA256 = actual
	if err := m.writeRaw(provider, payload); err != nil {
		return nil, err
	}
	return cache, nil
}

func parseSHA256(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return ""
	}
	return strings.ToLower(fields[0])
}

func (m *Manager[E]) fetchJSON(ctx context.Context, url string, target any) error {
	payload, err := m.fetchBytes(ctx, url, map[string]string{"Accept": githubAcceptJSON})
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func (m *Manager[E]) fetchBytes(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	var body []byte
	err := retryProviderRequest(ctx, func() error {
		payload, err := m.fetchBytesOnce(ctx, url, headers)
		if err == nil {
			body = payload
		}
		return err
	})
	return body, err
}

func (m *Manager[E]) fetchBytesOnce(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	timeout, maxBytes, userAgent, client := m.snapshot()

	ac := util.NewActivityController(ctx, timeout)
	defer ac.Close()

	req, err := http.NewRequestWithContext(ac.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", githubAcceptJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, markProviderErrorRetryable(ac.WrapErr(err))
	}
	defer func() { _ = resp.Body.Close() }()

	ac.Reset()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(ac.Reader(resp.Body), 1024))
		err := fmt.Errorf("fetch %s: HTTP %d (%s)", url, resp.StatusCode, string(body))
		if retryableStatus(resp.StatusCode) {
			return nil, markProviderErrorRetryable(err)
		}
		return nil, err
	}
	if cl := resp.ContentLength; cl > maxBytes {
		return nil, fmt.Errorf("provider asset too large: %d bytes (limit %d bytes)", cl, maxBytes)
	}
	body, err := io.ReadAll(io.LimitReader(ac.Reader(resp.Body), maxBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, markProviderErrorRetryable(ac.WrapErr(err))
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("provider asset too large: exceeds limit %d bytes", maxBytes)
	}
	return body, nil
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
