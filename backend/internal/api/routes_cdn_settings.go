package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func (s *Server) registerCDNSettingsRoutes(r chi.Router) {
	r.Get("/cdn-settings", s.adminGuard(s.handleGetCDNSettings))
	r.Put("/cdn-settings", s.adminGuard(s.handlePutCDNSettings))
}

func (s *Server) handleGetCDNSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Store.GetCdnSettings(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (s *Server) handlePutCDNSettings(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if err := s.DecodeJSON(r, &raw); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	updates := schema.CdnSettings{}
	present := map[string]bool{}
	if v, ok := raw["enabled"]; ok {
		_ = json.Unmarshal(v, &updates.Enabled)
		present["enabled"] = true
	}
	if v, ok := raw["cacheMode"]; ok {
		_ = json.Unmarshal(v, &updates.CacheMode)
		if err := schema.ValidateCacheMode(updates.CacheMode); err != nil {
			s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		present["cacheMode"] = true
	}
	if v, ok := raw["staleIfErrorSeconds"]; ok {
		_ = json.Unmarshal(v, &updates.StaleIfErrorSeconds)
		if updates.StaleIfErrorSeconds < 0 {
			s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", "staleIfErrorSeconds must be >= 0")
			return
		}
		present["staleIfErrorSeconds"] = true
	}
	if v, ok := raw["customCacheControl"]; ok {
		_ = json.Unmarshal(v, &updates.CustomCacheControl)
		present["customCacheControl"] = true
	}
	if v, ok := raw["cloudflareCdnCacheControl"]; ok {
		_ = json.Unmarshal(v, &updates.CloudflareCdnCacheControl)
		present["cloudflareCdnCacheControl"] = true
	}
	if v, ok := raw["customHeaders"]; ok {
		_ = json.Unmarshal(v, &updates.CustomHeaders)
		updates.CustomHeaders = sanitizeHeaders(updates.CustomHeaders)
		present["customHeaders"] = true
	}
	saved, err := s.Store.UpdateCdnSettings(r.Context(), updates, present)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "settings": saved})
}
