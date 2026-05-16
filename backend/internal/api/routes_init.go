package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

func (s *Server) registerInitRoutes(r chi.Router) {
	r.Get("/init", s.adminGuard(s.handleInitStatus))
	r.Post("/init", s.adminGuard(s.handleInitExecute))
}

func (s *Server) handleInitStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"initialized": len(cfg.Rules) > 0,
		"rulesCount":  len(cfg.Rules),
	})
}

func (s *Server) handleInitExecute(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(cfg.Rules) > 0 {
		s.JSON(w, http.StatusOK, map[string]any{
			"success":    false,
			"message":    "Already initialized with existing rules",
			"rulesCount": len(cfg.Rules),
		})
		return
	}
	templateData, err := s.loadInitialTemplate()
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var initial schema.RulesConfig
	if err := json.Unmarshal(templateData, &initial); err != nil {
		s.Error(w, http.StatusBadRequest, "Invalid config template: "+err.Error())
		return
	}
	initial.EnsureDefaults()
	rev, err := s.Store.SaveConfig(r.Context(), initial)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	sched, err := s.Store.GetSyncSchedule(r.Context())
	if err == nil {
		now := util.NowISO()
		next := syncengine.ComputeNextSyncAt(sched, &now)
		update := schema.SyncSchedule{LastScheduledSyncAt: &now}
		if next != "" {
			update.NextSyncAt = &next
		}
		_, _ = s.Store.UpdateSyncSchedule(r.Context(), update)
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"message":    fmt.Sprintf("Initialized with %d rules", len(initial.Rules)),
		"rulesCount": len(initial.Rules),
		"rev":        rev,
	})
}

func (s *Server) loadInitialTemplate() ([]byte, error) {
	candidates := []string{
		os.Getenv("INITIAL_CONFIG_PATH"),
		filepath.Join(s.Config.OutDir, "templates", "initial-config.json"),
		filepath.Join(s.Config.OutDir, "..", "public", "templates", "initial-config.json"),
	}
	var lastErr error
	for _, c := range candidates {
		if c == "" {
			continue
		}
		data, err := os.ReadFile(c)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("not found")
	}
	return nil, fmt.Errorf("initial config template not found: %w", lastErr)
}
