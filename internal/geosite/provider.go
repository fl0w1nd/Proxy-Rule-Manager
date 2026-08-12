package geosite

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// Provider names supported by this backend.
const (
	ProviderV2fly        = "v2fly"
	ProviderLoyalsoldier = "loyalsoldier"
)

// SupportedProviders lists the providers we know about.
var SupportedProviders = []string{ProviderV2fly, ProviderLoyalsoldier}

const (
	githubUserAgent     = "Proxy-Rule-Manager/2.0"
	githubAcceptJSON    = "application/vnd.github+json"
	fetchTimeout        = 20 * time.Second
	maxProviderDownload = 50 * 1024 * 1024
	defaultCacheTTL     = 24 * time.Hour
)

// refreshState coordinates concurrent Refresh calls for the same provider.
// The first caller stores its result so waiters receive the same error
// instead of a silent nil.
type refreshState struct {
	done  chan struct{}
	cache *ProviderCache
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
type Manager struct {
	dir        string
	mu         sync.RWMutex
	memCache   map[string]*ProviderCache
	refresh    map[string]*refreshState
	httpClient *http.Client
	// ttl, when > 0, makes Ensure auto-refresh caches whose FetchedAt is older
	// than ttl.
	ttl time.Duration
}

// NewManager constructs a manager that persists caches under `dir`.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:        dir,
		memCache:   make(map[string]*ProviderCache),
		refresh:    make(map[string]*refreshState),
		httpClient: &http.Client{Timeout: fetchTimeout},
		ttl:        defaultCacheTTL,
	}
}

// SetHTTPClient replaces the HTTP client used for upstream fetches.
// Intended for tests that need to inject a mock transport.
func (m *Manager) SetHTTPClient(c *http.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.httpClient = c
}

// SetCacheTTL controls Ensure's auto-refresh behaviour. ttl<=0 disables it.
func (m *Manager) SetCacheTTL(ttl time.Duration) {
	m.mu.Lock()
	m.ttl = ttl
	m.mu.Unlock()
}

// CacheTTL returns the configured TTL.
func (m *Manager) CacheTTL() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ttl
}

// Read returns the cached provider data, loading from disk if necessary.
func (m *Manager) Read(provider string) (*ProviderCache, error) {
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
	var cache ProviderCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.memCache[provider] = &cache
	m.mu.Unlock()
	return &cache, nil
}

// Ensure returns the cache, fetching it from network if it does not exist.
// When a TTL is configured, caches older than the TTL are auto-refreshed; if
// the refresh fails, the stale cache is still returned so reads remain
// available even when the upstream is temporarily down.
func (m *Manager) Ensure(ctx context.Context, provider string) (*ProviderCache, error) {
	existing, err := m.Read(provider)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return m.Refresh(ctx, provider)
	}
	ttl := m.CacheTTL()
	if ttl > 0 && cacheExpired(existing.FetchedAt, ttl) {
		fresh, err := m.Refresh(ctx, provider)
		if err != nil {
			return existing, fmt.Errorf("refresh stale %s cache: %w", provider, err)
		}
		if fresh == nil {
			return existing, fmt.Errorf("refresh stale %s cache returned no data", provider)
		}
		return fresh, nil
	}
	return existing, nil
}

// cacheExpired returns true when fetchedAt is older than ttl.
func cacheExpired(fetchedAt string, ttl time.Duration) bool {
	if fetchedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > ttl
}

// Refresh re-fetches the provider data from the upstream source.
func (m *Manager) Refresh(ctx context.Context, provider string) (*ProviderCache, error) {
	m.mu.Lock()
	if state, ok := m.refresh[provider]; ok {
		m.mu.Unlock()
		<-state.done
		return state.cache, state.err
	}
	state := &refreshState{done: make(chan struct{})}
	m.refresh[provider] = state
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.refresh, provider)
		m.mu.Unlock()
		close(state.done)
	}()

	var (
		cache *ProviderCache
		err   error
	)
	switch provider {
	case ProviderV2fly:
		cache, err = m.refreshV2fly(ctx)
	case ProviderLoyalsoldier:
		cache, err = m.refreshLoyalsoldier(ctx)
	default:
		err = fmt.Errorf("unsupported geosite provider: %s", provider)
	}
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
func (m *Manager) RefreshWithRetry(
	ctx context.Context,
	provider string,
	retries int,
	retryDelay time.Duration,
	onRetry func(attempt, total int, delay time.Duration, err error),
) (*ProviderCache, error) {
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

func (m *Manager) write(cache *ProviderCache) error {
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
	// Invalidate domain lookup index for the previous version.
	lookupMu.Lock()
	for k := range lookupCache {
		if strings.HasPrefix(k, cache.Provider+":") {
			delete(lookupCache, k)
		}
	}
	lookupMu.Unlock()
	return nil
}

func (m *Manager) cachePath(provider string) (string, error) {
	if err := util.EnsureSafeSegment(provider, "geosite provider"); err != nil {
		return "", err
	}
	return util.JoinInside(m.dir, provider+".json")
}

// ListStatus mirrors listGeositeProviders.
func (m *Manager) ListStatus() []ProviderStatus {
	out := make([]ProviderStatus, 0, len(SupportedProviders))
	for _, p := range SupportedProviders {
		cache, _ := m.Read(p)
		status := ProviderStatus{Provider: p}
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

func (m *Manager) latestRelease(ctx context.Context, repository string) (*githubRelease, error) {
	var release githubRelease
	if err := m.fetchJSON(ctx, "https://api.github.com/repos/"+repository+"/releases/latest", &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (m *Manager) refreshV2fly(ctx context.Context) (*ProviderCache, error) {
	release, err := m.latestRelease(ctx, "v2fly/domain-list-community")
	if err != nil {
		return nil, err
	}
	datURL := release.assetURL("dlc.dat")
	checksumURL := release.assetURL("dlc.dat.sha256sum")
	if datURL == "" || checksumURL == "" {
		return nil, errors.New("v2fly release is missing dlc.dat or dlc.dat.sha256sum")
	}
	payload, err := m.fetchBytes(ctx, datURL, map[string]string{"Accept": "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	checksum, err := m.fetchBytes(ctx, checksumURL, map[string]string{"Accept": "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	if err := verifySHA256(payload, checksum); err != nil {
		return nil, fmt.Errorf("verify dlc.dat: %w", err)
	}
	return decodeProviderGeositeDat(payload, ProviderV2fly, release.TagName)
}

func (m *Manager) refreshLoyalsoldier(ctx context.Context) (*ProviderCache, error) {
	release, err := m.latestRelease(ctx, "Loyalsoldier/v2ray-rules-dat")
	if err != nil {
		return nil, err
	}
	assetURL := release.assetURL("geosite.dat")
	if assetURL == "" {
		return nil, errors.New("geosite.dat asset not found in Loyalsoldier release")
	}
	payload, err := m.fetchBytes(ctx, assetURL, map[string]string{"Accept": "application/octet-stream"})
	if err != nil {
		return nil, err
	}
	return decodeProviderGeositeDat(payload, ProviderLoyalsoldier, release.TagName)
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

func (m *Manager) fetchJSON(ctx context.Context, url string, target any) error {
	payload, err := m.fetchBytes(ctx, url, map[string]string{"Accept": githubAcceptJSON})
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func (m *Manager) fetchBytes(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
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

func (m *Manager) fetchBytesOnce(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", githubUserAgent)
	req.Header.Set("Accept", githubAcceptJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := m.httpClient.Do(req)
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
		if retryableProviderStatus(resp.StatusCode) {
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

func retryableProviderStatus(status int) bool {
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
