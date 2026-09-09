package serve

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

type geoipCatalogResponse struct {
	Provider  string               `json:"provider"`
	Version   string               `json:"version,omitempty"`
	FetchedAt string               `json:"fetched_at,omitempty"`
	Query     string               `json:"query,omitempty"`
	Match     string               `json:"match,omitempty"`
	Lists     []geoip.ListOverview `json:"lists"`
	Total     int                  `json:"total"`
}

type geoipListResponse struct {
	Provider string        `json:"provider"`
	List     string        `json:"list"`
	Query    string        `json:"query,omitempty"`
	Total    int           `json:"total"`
	Offset   int           `json:"offset"`
	Limit    int           `json:"limit"`
	Items    []geoip.Entry `json:"items"`
}

func (s *Server) handleGeoIPCatalog(w http.ResponseWriter, r *http.Request) {
	cache, _, ok := s.geoipProviderCache(w, chi.URLParam(r, "provider"))
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validGeoQuery(w, query) {
		return
	}
	match := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("match")))
	if match == "" {
		match = "name"
	}
	if match != "name" && match != "content" {
		writeAPIError(w, http.StatusBadRequest, "invalid_match", "match 只能是 name 或 content", map[string]any{})
		return
	}
	lists := geoip.ListOverviews(cache, query, match)
	if lists == nil {
		lists = []geoip.ListOverview{}
	}
	writeJSON(w, http.StatusOK, geoipCatalogResponse{
		Provider:  cache.Provider,
		Version:   cache.ResolvedVersion,
		FetchedAt: cache.FetchedAt,
		Query:     query,
		Match:     match,
		Lists:     lists,
		Total:     len(lists),
	})
}

func (s *Server) handleGeoIPList(w http.ResponseWriter, r *http.Request) {
	cache, provider, ok := s.geoipProviderCache(w, chi.URLParam(r, "provider"))
	if !ok {
		return
	}
	listName := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "list")))
	if err := util.EnsureSafeSegment(listName, "geoip list"); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_list", "列表名称无效", map[string]any{})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validGeoQuery(w, query) {
		return
	}
	offset, limit, ok := geoListPage(w, r)
	if !ok {
		return
	}
	entries, err := geoip.ResolveEntries(cache, listName)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "list_not_found", "找不到该 GeoIP 列表", map[string]any{"provider": provider, "list": listName})
		return
	}
	entries = geoip.FilterEntries(entries, query)
	total := len(entries)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	items := entries[offset:end]
	if items == nil {
		items = []geoip.Entry{}
	}
	writeJSON(w, http.StatusOK, geoipListResponse{
		Provider: provider,
		List:     listName,
		Query:    query,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		Items:    items,
	})
}

func (s *Server) geoipProviderCache(w http.ResponseWriter, rawName string) (*geoip.ProviderCache, string, bool) {
	configured := false
	if cfg := s.config(); cfg.GeoIP != nil {
		for _, provider := range cfg.GeoIP.Providers {
			if strings.EqualFold(provider.Name, strings.TrimSpace(rawName)) {
				configured = true
				break
			}
		}
	}
	name, ok := s.resolveConfiguredProvider(w, rawName, "geoip", "GeoIP", geoip.SupportedProviders, configured)
	if !ok {
		return nil, "", false
	}
	if s.Engine.GeoIP == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "geoip_unavailable", "GeoIP 缓存不可用", map[string]any{})
		return nil, "", false
	}
	cache, err := s.Engine.GeoIP.Read(name)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "geoip_cache_read_failed", "读取 GeoIP 缓存失败", map[string]any{})
		return nil, "", false
	}
	if cache == nil {
		writeAPIError(w, http.StatusNotFound, "geoip_cache_missing", "尚未下载该提供商数据，请先执行更新", map[string]any{"name": name})
		return nil, "", false
	}
	return cache, name, true
}
