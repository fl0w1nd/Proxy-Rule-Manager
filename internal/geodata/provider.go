package geodata

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	githubUserAgent     = "Proxy-Rule-Manager/2.0"
	githubAcceptJSON    = "application/vnd.github+json"
	fetchTimeout        = 20 * time.Second
	maxProviderDownload = 50 * 1024 * 1024
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
}

// NewManager constructs a manager that persists caches under `dir`.
func NewManager[E any](dir, kind string, sources map[string]Source, decode func([]byte, string, string) (*Cache[E], error), onWrite func(*Cache[E])) *Manager[E] {
	return &Manager[E]{dir: dir, kind: kind, sources: sources, decode: decode, onWrite: onWrite,
		memCache: make(map[string]*Cache[E]), refresh: make(map[string]*refreshState[E]),
		httpClient: &http.Client{Timeout: fetchTimeout}}
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
	release, err := m.latestRelease(ctx, source.Repository)
	if err != nil {
		return nil, err
	}
	assetURL := release.assetURL(source.Asset)
	if assetURL == "" {
		return nil, fmt.Errorf("%s release missing %s", provider, source.Asset)
	}
	payload, err := m.fetchBytes(ctx, assetURL, map[string]string{"Accept": "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	if source.Checksum {
		checksumURL := release.assetURL(source.Asset + ".sha256sum")
		if checksumURL == "" {
			return nil, fmt.Errorf("%s release missing checksum", provider)
		}
		checksum, err := m.fetchBytes(ctx, checksumURL, map[string]string{"Accept": "application/octet-stream"})
		if err != nil {
			return nil, err
		}
		if err := verifySHA256(payload, checksum); err != nil {
			return nil, err
		}
	}
	return m.decode(payload, provider, release.TagName)
}

func verifySHA256(payload, checksumFile []byte) error {
	fields := strings.Fields(string(checksumFile))
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return errors.New("invalid SHA256 checksum file")
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(payload))
	if !strings.EqualFold(actual, fields[0]) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, fields[0])
	}
	return nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", githubUserAgent)
	req.Header.Set("Accept", githubAcceptJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	m.mu.RLock()
	client := m.httpClient
	m.mu.RUnlock()
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, markProviderErrorRetryable(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		err := fmt.Errorf("fetch %s: HTTP %d (%s)", url, resp.StatusCode, string(body))
		if retryableStatus(resp.StatusCode) {
			return nil, markProviderErrorRetryable(err)
		}
		return nil, err
	}
	if cl := resp.ContentLength; cl > maxProviderDownload {
		return nil, fmt.Errorf("provider asset too large: %d bytes", cl)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderDownload+1))
	if err != nil {
		return nil, markProviderErrorRetryable(err)
	}
	if len(body) > maxProviderDownload {
		return nil, fmt.Errorf("provider asset too large: %d bytes", len(body))
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
