package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func (s *Server) registerGeositeRoutes(r chi.Router) {
	r.Get("/geosite/providers", s.adminGuard(s.handleGeositeProviders))
	r.Post("/geosite/providers/{provider}/refresh", s.adminGuard(s.handleGeositeRefresh))
	r.Post("/geosite/providers/{provider}/sync", s.adminGuard(s.handleGeositeProviderSync))
	r.Get("/geosite/catalog", s.adminGuard(s.handleGeositeCatalog))
	r.Get("/geosite/domain-lookup", s.adminGuard(s.handleGeositeDomainLookup))
	r.Post("/geosite/import-all", s.adminGuard(s.handleGeositeImportAll))
	r.Post("/geosite/import-selected", s.adminGuard(s.handleGeositeImportSelected))
	r.Get("/geosite/preview", s.adminGuard(s.handleGeositePreview))
}

func (s *Server) handleGeositeProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.Geosite.ListStatus()
	s.JSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// validateProvider returns (name, true) when name is a supported geosite provider.
// It delegates to schema.ValidateGeositeProvider as the single source of truth.
func validateProvider(name string) (string, bool) {
	if err := schema.ValidateGeositeProvider(name); err != nil {
		return "", false
	}
	return name, true
}

func (s *Server) handleGeositeRefresh(w http.ResponseWriter, r *http.Request) {
	provider, ok := validateProvider(chi.URLParam(r, "provider"))
	if !ok {
		s.Error(w, http.StatusBadRequest, "Unsupported geosite provider")
		return
	}
	cache, err := s.Geosite.Refresh(r.Context(), provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":         true,
		"provider":        provider,
		"resolvedVersion": cache.ResolvedVersion,
		"fetchedAt":       cache.FetchedAt,
		"catalogCount":    len(cache.Catalog),
	})
}

// handleGeositeProviderSync syncs only the geosite rules that belong to the
// given provider. It refreshes the upstream cache first, then runs a partial
// batch sync over the matching rule names. This avoids triggering a full sync
// (which would also re-process all non-geosite rules) when the user only
// wants to update geosite content for one provider.
func (s *Server) handleGeositeProviderSync(w http.ResponseWriter, r *http.Request) {
	provider, ok := validateProvider(chi.URLParam(r, "provider"))
	if !ok {
		s.Error(w, http.StatusBadRequest, "Unsupported geosite provider")
		return
	}
	// Refresh upstream cache so the sync picks up any catalog changes.
	cache, err := s.Geosite.Refresh(r.Context(), provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Collect rule names belonging to this provider.
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var ruleNames []string
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if !schema.IsGeositeRule(rule) {
			continue
		}
		src := schema.PrimaryGeositeSource(rule)
		if src == nil || src.Provider != provider {
			continue
		}
		ruleNames = append(ruleNames, rule.Name)
	}
	if len(ruleNames) == 0 {
		s.JSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"provider":     provider,
			"catalogCount": len(cache.Catalog),
			"sync": map[string]any{
				"syncedRules": []string{},
				"failedRules": []schema.JobFailedRule{},
			},
		})
		return
	}
	res, err := s.Engine.ExecuteBatchPartialSync(r.Context(), ruleNames)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !res.Success && res.JobID == "" {
		s.ErrorWithCode(w, http.StatusConflict, "SYNC_ALREADY_RUNNING", res.FailedRules[0].Error)
		return
	}
	failedRules := res.FailedRules
	if failedRules == nil {
		failedRules = []schema.JobFailedRule{}
	}
	syncedRules := make([]string, 0, len(ruleNames))
	failedSet := make(map[string]struct{}, len(failedRules))
	for _, f := range failedRules {
		failedSet[f.Name] = struct{}{}
	}
	for _, name := range ruleNames {
		if _, bad := failedSet[name]; !bad {
			syncedRules = append(syncedRules, name)
		}
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"provider":     provider,
		"catalogCount": len(cache.Catalog),
		"sync": map[string]any{
			"syncedRules": syncedRules,
			"failedRules": failedRules,
		},
	})
}

func (s *Server) handleGeositeCatalog(w http.ResponseWriter, r *http.Request) {
	provider, ok := validateProvider(r.URL.Query().Get("provider"))
	if !ok {
		s.Error(w, http.StatusBadRequest, "Unsupported geosite provider")
		return
	}
	cache, err := s.Geosite.Read(provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cache == nil {
		cache, err = s.Geosite.Ensure(r.Context(), provider)
		if err != nil {
			s.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	catalog := geosite.CatalogSummaries(cache)
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	imported := map[string]struct {
		RuleName string
		Clients  []string
	}{}
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if !schema.IsGeositeRule(rule) {
			continue
		}
		src := schema.PrimaryGeositeSource(rule)
		if src == nil || src.Provider != provider || src.List == "" {
			continue
		}
		imported[src.List] = struct {
			RuleName string
			Clients  []string
		}{RuleName: rule.Name, Clients: append([]string{}, rule.Output.Clients...)}
	}
	type item struct {
		Name       string   `json:"name"`
		Imported   bool     `json:"imported"`
		RuleName   *string  `json:"ruleName"`
		Clients    []string `json:"clients"`
		Attrs      []string `json:"attrs"`
		EntryCount int      `json:"entryCount"`
	}
	out := make([]item, 0, len(catalog))
	catalogSet := make(map[string]struct{}, len(catalog))
	for _, c := range catalog {
		catalogSet[c.Name] = struct{}{}
		entry := item{
			Name:       c.Name,
			Attrs:      c.Attrs,
			EntryCount: c.EntryCount,
			Clients:    []string{},
		}
		if info, ok := imported[c.Name]; ok {
			entry.Imported = true
			n := info.RuleName
			entry.RuleName = &n
			entry.Clients = info.Clients
		}
		out = append(out, entry)
	}

	type staleImport struct {
		Name     string   `json:"name"`
		RuleName string   `json:"ruleName"`
		Clients  []string `json:"clients"`
	}
	staleNames := make([]string, 0)
	for name := range imported {
		if _, ok := catalogSet[name]; !ok {
			staleNames = append(staleNames, name)
		}
	}
	sort.Strings(staleNames)
	stale := make([]staleImport, 0, len(staleNames))
	for _, name := range staleNames {
		info := imported[name]
		clients := info.Clients
		if clients == nil {
			clients = []string{}
		}
		stale = append(stale, staleImport{
			Name:     name,
			RuleName: info.RuleName,
			Clients:  clients,
		})
	}

	s.JSON(w, http.StatusOK, map[string]any{
		"provider":        provider,
		"resolvedVersion": cache.ResolvedVersion,
		"fetchedAt":       cache.FetchedAt,
		"catalog":         out,
		"staleImports":    stale,
	})
}

func (s *Server) handleGeositeDomainLookup(w http.ResponseWriter, r *http.Request) {
	provider, ok := validateProvider(r.URL.Query().Get("provider"))
	if !ok {
		s.Error(w, http.StatusBadRequest, "Unsupported geosite provider")
		return
	}
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if len(domain) < 2 {
		s.JSON(w, http.StatusOK, map[string]any{"matches": []string{}})
		return
	}
	cache, err := s.Geosite.Ensure(r.Context(), provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	matches := geosite.LookupListsInEntries(cache, domain)
	if matches == nil {
		matches = []string{}
	}
	s.JSON(w, http.StatusOK, map[string]any{"matches": matches})
}

type geositeImportAllRequest struct {
	Provider string `json:"provider"`
	ClientID string `json:"clientId"`
}

func (s *Server) handleGeositeImportAll(w http.ResponseWriter, r *http.Request) {
	var body geositeImportAllRequest
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	provider, ok := validateProvider(body.Provider)
	if !ok || body.ClientID == "" {
		s.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}
	clients, err := s.Store.GetClients(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasClient(clients, body.ClientID) {
		s.Error(w, http.StatusBadRequest, "Client \""+body.ClientID+"\" not found")
		return
	}
	cache, err := s.Geosite.Ensure(r.Context(), provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	selections := make([]geosite.ImportSelection, 0, len(cache.Catalog))
	for _, name := range cache.Catalog {
		selections = append(selections, geosite.ImportSelection{List: name})
	}
	result, err := s.applyGeositeSelections(r, provider, body.ClientID, selections)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	syncRes, err := s.syncImported(r, result.RuleNames)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"created":   result.Created,
		"updated":   result.Updated,
		"skipped":   result.Skipped,
		"total":     result.Total,
		"ruleNames": result.RuleNames,
		"sync":      syncRes,
	})
}

type geositeImportSelectedRequest struct {
	Provider string            `json:"provider"`
	ClientID string            `json:"clientId"`
	Lists    []json.RawMessage `json:"lists"`
}

func (s *Server) handleGeositeImportSelected(w http.ResponseWriter, r *http.Request) {
	var body geositeImportSelectedRequest
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	provider, ok := validateProvider(body.Provider)
	if !ok || body.ClientID == "" || len(body.Lists) == 0 {
		s.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}
	clients, err := s.Store.GetClients(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasClient(clients, body.ClientID) {
		s.Error(w, http.StatusBadRequest, "Client \""+body.ClientID+"\" not found")
		return
	}
	selections := make([]geosite.ImportSelection, 0, len(body.Lists))
	for _, raw := range body.Lists {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			selections = append(selections, geosite.ImportSelection{List: str})
			continue
		}
		var obj geosite.ImportSelection
		if err := json.Unmarshal(raw, &obj); err == nil && obj.List != "" {
			selections = append(selections, obj)
		}
	}
	if len(selections) == 0 {
		s.Error(w, http.StatusBadRequest, "No valid selections")
		return
	}

	// Ensure the provider cache is loaded (same as import-all), then validate
	// that every requested list name actually exists in the catalog.
	cache, err := s.Geosite.Ensure(r.Context(), provider)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	catalogSet := make(map[string]struct{}, len(cache.Catalog))
	for _, name := range cache.Catalog {
		catalogSet[name] = struct{}{}
	}
	for _, sel := range selections {
		if _, ok := catalogSet[sel.List]; !ok {
			s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR",
				fmt.Sprintf("Geosite list %q not found for provider %q", sel.List, provider))
			return
		}
	}

	result, err := s.applyGeositeSelections(r, provider, body.ClientID, selections)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	syncRes, err := s.syncImported(r, result.RuleNames)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"created":   result.Created,
		"updated":   result.Updated,
		"skipped":   result.Skipped,
		"total":     result.Total,
		"ruleNames": result.RuleNames,
		"sync":      syncRes,
	})
}

func (s *Server) applyGeositeSelections(r *http.Request, provider, clientID string, selections []geosite.ImportSelection) (geosite.ImportResult, error) {
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		return geosite.ImportResult{}, err
	}
	result := geosite.UpsertImportedGeositeRules(&cfg, provider, clientID, selections)
	if _, err := s.Store.SaveConfig(r.Context(), cfg); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Server) syncImported(r *http.Request, names []string) (map[string]any, error) {
	unique := uniqueStrings(names)
	if len(unique) == 0 {
		return map[string]any{"syncedRules": []string{}, "failedRules": []any{}}, nil
	}
	res, err := s.Engine.ExecuteBatchPartialSync(r.Context(), unique)
	if err != nil {
		return nil, err
	}
	failed := make(map[string]struct{}, len(res.FailedRules))
	for _, f := range res.FailedRules {
		failed[f.Name] = struct{}{}
	}
	synced := make([]string, 0, len(unique))
	for _, n := range unique {
		if _, bad := failed[n]; !bad {
			synced = append(synced, n)
		}
	}
	failedRules := res.FailedRules
	if failedRules == nil {
		failedRules = []schema.JobFailedRule{}
	}
	out := map[string]any{
		"syncedRules": synced,
		"failedRules": failedRules,
	}
	return out, nil
}

func (s *Server) handleGeositePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	provider, ok := validateProvider(q.Get("provider"))
	if !ok {
		s.Error(w, http.StatusBadRequest, "Unsupported geosite provider")
		return
	}
	list := q.Get("list")
	clientID := q.Get("client")
	attrs := []string{}
	if raw := strings.TrimSpace(q.Get("attrs")); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			if v := strings.TrimSpace(item); v != "" {
				attrs = append(attrs, v)
			}
		}
	}
	if list == "" || clientID == "" {
		s.Error(w, http.StatusBadRequest, "provider, list and client are required")
		return
	}
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	limit = resolvePreviewLimit(limit)
	clients, err := s.Store.GetClients(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasClient(clients, clientID) {
		s.Error(w, http.StatusBadRequest, "Client \""+clientID+"\" not found")
		return
	}
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	apply := s.Engine.Transformer.ApplyNewTransforms
	res, err := geosite.Preview(r.Context(), s.Geosite, provider, list, clientID, attrs, "", clients, cfg.Transformers, apply, limit)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"content":      res.Content,
		"totalEntries": res.TotalEntries,
		"totalLines":   res.TotalLines,
		"truncated":    res.Truncated,
	})
}

func hasClient(clients []schema.ClientConfig, id string) bool {
	for _, c := range clients {
		if c.ID == id {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
