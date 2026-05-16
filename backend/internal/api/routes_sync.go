package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

func (s *Server) registerSyncRoutes(r chi.Router) {
	r.Post("/sync/full", s.adminGuard(s.handleSyncFull))
	r.Post("/sync/partial/batch", s.adminGuard(s.handleSyncBatch))
	r.Get("/sync/schedule", s.adminGuard(s.handleGetSyncSchedule))
	r.Put("/sync/schedule", s.adminGuard(s.handleUpdateSyncSchedule))
}

func (s *Server) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	res, err := s.Engine.ExecuteFullSync(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, normalizeSyncResult(res))
}

func (s *Server) handleSyncBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RuleNames []string `json:"ruleNames"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// Filter out blank entries, matching TS behaviour: item.trim().length > 0.
	filtered := body.RuleNames[:0]
	for _, name := range body.RuleNames {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	body.RuleNames = filtered
	if len(body.RuleNames) == 0 {
		s.Error(w, http.StatusBadRequest, "ruleNames is required")
		return
	}
	res, err := s.Engine.ExecuteBatchPartialSync(r.Context(), body.RuleNames)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, normalizeSyncResult(res))
}

func normalizeSyncResult(res syncengine.Result) syncengine.Result {
	if res.ChangedRules == nil {
		res.ChangedRules = []string{}
	}
	if res.FailedRules == nil {
		res.FailedRules = []schema.JobFailedRule{}
	}
	return res
}

func (s *Server) handleGetSyncSchedule(w http.ResponseWriter, r *http.Request) {
	sched, err := s.Store.GetSyncSchedule(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sched.NextSyncAt == nil {
		if next := syncengine.ComputeNextSyncAt(sched, sched.LastScheduledSyncAt); next != "" {
			sched.NextSyncAt = &next
		}
	}
	s.JSON(w, http.StatusOK, map[string]any{"schedule": sched})
}

func (s *Server) handleUpdateSyncSchedule(w http.ResponseWriter, r *http.Request) {
	var update schema.SyncSchedule
	if err := s.DecodeJSON(r, &update); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve mode: treat blank as "interval" (matches TS: mode === "cron" ? "cron" : "interval").
	resolvedMode := update.Mode
	if resolvedMode == "" {
		resolvedMode = "interval"
	}
	if resolvedMode != "interval" && resolvedMode != "cron" {
		s.Error(w, http.StatusBadRequest, "Invalid sync mode")
		return
	}

	if resolvedMode == "interval" {
		if update.IntervalHours < 1 {
			s.ErrorWith(w, http.StatusBadRequest, map[string]any{
				"error": "intervalHours must be a number >= 1",
				"code":  "VALIDATION_ERROR",
			})
			return
		}
	} else {
		cronExpr := strings.TrimSpace(update.CronExpression)
		if cronExpr == "" {
			s.ErrorWith(w, http.StatusBadRequest, map[string]any{
				"error": "cronExpression must be a non-empty string",
				"code":  "VALIDATION_ERROR",
			})
			return
		}
		if err := syncengine.ValidateCronExpression(cronExpr); err != nil {
			s.ErrorWith(w, http.StatusBadRequest, map[string]any{
				"error": "Invalid cron expression: " + err.Error(),
				"code":  "VALIDATION_ERROR",
			})
			return
		}
	}

	saved, err := s.Store.UpdateSyncSchedule(r.Context(), update)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if next := syncengine.ComputeNextSyncAt(saved, saved.LastScheduledSyncAt); next != "" {
		saved.NextSyncAt = &next
		saved, err = s.Store.UpdateSyncSchedule(r.Context(), schema.SyncSchedule{NextSyncAt: &next})
		if err != nil {
			s.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "schedule": saved})
}
