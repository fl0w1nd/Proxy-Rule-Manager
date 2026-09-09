package serve

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	geositeQueryMaxLen  = 256
	geositeListLimitMax = 500
	geositeListLimitDef = 200
)

type geositeCatalogResponse struct {
	Provider  string                 `json:"provider"`
	Version   string                 `json:"version,omitempty"`
	FetchedAt string                 `json:"fetched_at,omitempty"`
	Query     string                 `json:"query,omitempty"`
	Match     string                 `json:"match,omitempty"`
	Lists     []geosite.ListOverview `json:"lists"`
	Total     int                    `json:"total"`
}

type geositeListResponse struct {
	Provider string          `json:"provider"`
	List     string          `json:"list"`
	Attr     string          `json:"attr,omitempty"`
	Query    string          `json:"query,omitempty"`
	Total    int             `json:"total"`
	Offset   int             `json:"offset"`
	Limit    int             `json:"limit"`
	Items    []geosite.Entry `json:"items"`
}

func (s *Server) handleGeositeCatalog(w http.ResponseWriter, r *http.Request) {
	cache, _, ok := s.geositeProviderCache(w, chi.URLParam(r, "provider"))
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validGeositeQuery(w, query) {
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
	lists := geosite.ListOverviews(cache)
	if query != "" {
		if match == "content" {
			lists = geosite.SelectListOverviews(lists, geosite.SearchListsByContent(cache, query))
		} else {
			lists = geosite.FilterListOverviews(lists, query)
		}
	}
	if lists == nil {
		lists = []geosite.ListOverview{}
	}
	writeJSON(w, http.StatusOK, geositeCatalogResponse{
		Provider:  cache.Provider,
		Version:   cache.ResolvedVersion,
		FetchedAt: cache.FetchedAt,
		Query:     query,
		Match:     match,
		Lists:     lists,
		Total:     len(lists),
	})
}

func (s *Server) handleGeositeList(w http.ResponseWriter, r *http.Request) {
	cache, provider, ok := s.geositeProviderCache(w, chi.URLParam(r, "provider"))
	if !ok {
		return
	}
	listName := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "list")))
	if err := util.EnsureSafeSegment(listName, "geosite list"); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_list", "列表名称无效", map[string]any{})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validGeositeQuery(w, query) {
		return
	}
	attr := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("attr")))
	var attrs []string
	if attr != "" {
		if err := util.EnsureSafeSegment(attr, "geosite attr"); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_attr", "变体名称无效", map[string]any{})
			return
		}
		attrs = []string{attr}
	}
	offset, limit, ok := geositeListPage(w, r)
	if !ok {
		return
	}
	entries, err := geosite.ResolveEntries(cache, listName, attrs)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "list_not_found", "找不到该 Geosite 列表", map[string]any{"provider": provider, "list": listName})
		return
	}
	entries = geosite.FilterEntries(entries, query)
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
		items = []geosite.Entry{}
	}
	writeJSON(w, http.StatusOK, geositeListResponse{
		Provider: provider,
		List:     listName,
		Attr:     attr,
		Query:    query,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		Items:    items,
	})
}

func (s *Server) geositeProviderCache(w http.ResponseWriter, rawName string) (*geosite.ProviderCache, string, bool) {
	name := strings.ToLower(strings.TrimSpace(rawName))
	if err := util.EnsureSafeSegment(name, "geosite provider"); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_provider", "提供商名称无效", map[string]any{})
		return nil, "", false
	}
	supported := false
	for _, item := range geosite.SupportedProviders {
		if item == name {
			supported = true
			break
		}
	}
	if !supported {
		writeAPIError(w, http.StatusBadRequest, "unsupported_provider", "不支持该 Geosite 提供商", map[string]any{"name": name})
		return nil, "", false
	}
	cfg := s.config()
	configured := false
	if cfg.Geosite != nil {
		for _, provider := range cfg.Geosite.Providers {
			if provider.Name == name {
				configured = true
				break
			}
		}
	}
	if !configured {
		writeAPIError(w, http.StatusNotFound, "provider_not_found", "未配置该 Geosite 提供商", map[string]any{"name": name})
		return nil, "", false
	}
	if s.Engine.Geosite == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "geosite_unavailable", "Geosite 缓存不可用", map[string]any{})
		return nil, "", false
	}
	cache, err := s.Engine.Geosite.Read(name)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "geosite_cache_read_failed", "读取 Geosite 缓存失败", map[string]any{})
		return nil, "", false
	}
	if cache == nil {
		writeAPIError(w, http.StatusNotFound, "geosite_cache_missing", "尚未下载该提供商数据，请先执行更新", map[string]any{"name": name})
		return nil, "", false
	}
	return cache, name, true
}

func validGeositeQuery(w http.ResponseWriter, query string) bool {
	if len(query) > geositeQueryMaxLen {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "查询字符串过长", map[string]any{})
		return false
	}
	return true
}

func geositeListPage(w http.ResponseWriter, r *http.Request) (offset, limit int, ok bool) {
	limit = geositeListLimitDef
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > geositeListLimitMax {
			writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit 必须在 1 到 500 之间", map[string]any{})
			return 0, 0, false
		}
		limit = parsed
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_offset", "offset 必须是非负整数", map[string]any{})
			return 0, 0, false
		}
		offset = parsed
	}
	return offset, limit, true
}
