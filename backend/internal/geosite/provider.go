package geosite

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// Provider names supported by this backend.
const (
	ProviderV2fly        = "v2fly"
	ProviderLoyalsoldier = "loyalsoldier"
)

// SupportedProviders lists the providers we know about.
var SupportedProviders = []string{ProviderV2fly, ProviderLoyalsoldier}

const (
	githubUserAgent     = "Proxy-Rule-Manager/1.0"
	githubAcceptJSON    = "application/vnd.github+json"
	fetchTimeout        = 20 * time.Second
	maxProviderDownload = 50 * 1024 * 1024
)

// Manager coordinates provider caches between disk and memory.
type Manager struct {
	dir        string
	mu         sync.RWMutex
	memCache   map[string]*ProviderCache
	refresh    map[string]chan struct{}
	httpClient *http.Client
	// ttl, when > 0, makes Ensure auto-refresh caches whose FetchedAt is older
	// than ttl. Default is 0 (no auto-refresh, preserving the historical
	// behaviour where partial syncs may run on stale caches indefinitely).
	ttl time.Duration
}

// NewManager constructs a manager that persists caches under `dir`.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:        dir,
		memCache:   make(map[string]*ProviderCache),
		refresh:    make(map[string]chan struct{}),
		httpClient: &http.Client{Timeout: fetchTimeout},
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
	filePath := filepath.Join(m.dir, provider+".json")
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
		if err == nil && fresh != nil {
			return fresh, nil
		}
		// Fall back to the stale copy on refresh failure; callers will still
		// see the older catalog rather than nothing, and Refresh failures are
		// surfaced through the regular full-sync error path.
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
	if done, ok := m.refresh[provider]; ok {
		m.mu.Unlock()
		<-done
		return m.Read(provider)
	}
	done := make(chan struct{})
	m.refresh[provider] = done
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.refresh, provider)
		close(done)
		m.mu.Unlock()
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
		return nil, fmt.Errorf("unsupported geosite provider: %s", provider)
	}
	if err != nil {
		return nil, err
	}
	if err := m.write(cache); err != nil {
		return nil, err
	}
	return cache, nil
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
	if err := util.AtomicWriteFile(filepath.Join(m.dir, cache.Provider+".json"), payload); err != nil {
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

func (m *Manager) refreshV2fly(ctx context.Context) (*ProviderCache, error) {
	type repoMeta struct {
		DefaultBranch string `json:"default_branch"`
	}
	var meta repoMeta
	if err := m.fetchJSON(ctx, "https://api.github.com/repos/v2fly/domain-list-community", &meta); err != nil {
		return nil, err
	}
	type commitMeta struct {
		SHA string `json:"sha"`
	}
	var commit commitMeta
	if err := m.fetchJSON(ctx, "https://api.github.com/repos/v2fly/domain-list-community/commits/"+meta.DefaultBranch, &commit); err != nil {
		return nil, err
	}
	zipBytes, err := m.fetchBytes(ctx, "https://codeload.github.com/v2fly/domain-list-community/zip/"+commit.SHA, nil)
	if err != nil {
		return nil, err
	}
	files, err := extractV2flyDataFiles(zipBytes)
	if err != nil {
		return nil, err
	}
	cache, err := BuildV2flyCacheFromRawFiles(files, commit.SHA)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (m *Manager) refreshLoyalsoldier(ctx context.Context) (*ProviderCache, error) {
	type release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	var rel release
	if err := m.fetchJSON(ctx, "https://api.github.com/repos/Loyalsoldier/v2ray-rules-dat/releases/latest", &rel); err != nil {
		return nil, err
	}
	var assetURL string
	for _, asset := range rel.Assets {
		if asset.Name == "geosite.dat" {
			assetURL = asset.BrowserDownloadURL
		}
	}
	if assetURL == "" {
		return nil, errors.New("geosite.dat asset not found in Loyalsoldier release")
	}
	payload, err := m.fetchBytes(ctx, assetURL, map[string]string{"User-Agent": githubUserAgent})
	if err != nil {
		return nil, err
	}
	return DecodeLoyalsoldierGeositeDat(payload, rel.TagName)
}

var v2flyDataPattern = regexp.MustCompile(`^[^/]+/data/(.+)$`)

func extractV2flyDataFiles(zipBytes []byte) (map[string]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ReplaceAll(f.Name, "\\", "/")
		match := v2flyDataPattern.FindStringSubmatch(name)
		if len(match) != 2 {
			continue
		}
		fileName := match[1]
		if fileName == "" || strings.Contains(fileName, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[normalizeName(fileName)] = string(data)
	}
	return files, nil
}

func (m *Manager) fetchJSON(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", githubUserAgent)
	req.Header.Set("Accept", githubAcceptJSON)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("fetch %s: HTTP %d (%s)", url, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (m *Manager) fetchBytes(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fetch %s: HTTP %d (%s)", url, resp.StatusCode, string(body))
	}
	if cl := resp.ContentLength; cl > maxProviderDownload {
		return nil, fmt.Errorf("provider asset too large: %d bytes", cl)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderDownload+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxProviderDownload {
		return nil, fmt.Errorf("provider asset too large: %d bytes", len(body))
	}
	return body, nil
}
