package api

import (
	"context"
	"log"
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
	// Progress / cancel are part of the same sync surface so they live
	// here next to the trigger endpoint. GET /sync/progress is the only
	// admin endpoint a polling client hits — it is intentionally cheap
	// (in-memory state, no DB queries) so a 1-2s poll interval is fine.
	r.Get("/sync/progress", s.adminGuard(s.handleSyncProgress))
	r.Post("/sync/cancel", s.adminGuard(s.handleSyncCancel))
}

// handleSyncFull kicks off a full sync in the background. The request
// returns 202 Accepted as soon as the tracker slot is claimed; the
// client then polls /sync/progress to watch it run and POST /sync/cancel
// to abort. Two concurrent triggers receive 409 Conflict — the in-memory
// tracker is the single source of truth, separate from the DB-level
// global sync lock the engine will still acquire.
func (s *Server) handleSyncFull(w http.ResponseWriter, r *http.Request) {
	// Detached parent context: the goroutine must outlive the HTTP
	// request. RunningSync.Cancel() / SyncTracker.Cancel() drive
	// cancellation explicitly; we never want a transient client
	// disconnect to abort the sync.
	rs, syncCtx, ok := s.SyncTracker.Begin(context.Background(), "full_sync")
	if !ok {
		s.ErrorWith(w, http.StatusConflict, map[string]any{
			"error": "Another sync is already running",
			"code":  "SYNC_ALREADY_RUNNING",
		})
		return
	}
	rs.SetTrigger("manual")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[sync] async full sync panic: %v", rec)
				s.SyncTracker.End(syncengine.Result{}, asError(rec))
			}
		}()
		res, err := s.Engine.ExecuteFullSyncReport(syncCtx, rs)
		s.SyncTracker.End(res, err)
	}()
	s.JSON(w, http.StatusAccepted, map[string]any{
		"status":    "started",
		"jobType":   "full_sync",
		"startedAt": rs.Snapshot().StartedAt,
	})
}

// handleSyncProgress returns the current in-flight sync snapshot or, if
// none is running, an idle payload with the most recent completion
// summary embedded under "last". The shape is identical in both states
// so the frontend can render with a single component.
func (s *Server) handleSyncProgress(w http.ResponseWriter, r *http.Request) {
	if active := s.SyncTracker.Active(); active != nil {
		snap := active.Snapshot()
		snap.Last = s.SyncTracker.Last()
		s.JSON(w, http.StatusOK, snap)
		return
	}
	s.JSON(w, http.StatusOK, s.SyncTracker.IdleProgress())
}

// handleSyncCancel signals the running sync to stop. Idempotent: a
// second cancel during the same run is silently accepted because the
// engine may still be wrapping up. Returns 404 when no sync is active.
func (s *Server) handleSyncCancel(w http.ResponseWriter, r *http.Request) {
	if s.SyncTracker.Cancel("admin_request") {
		s.JSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	s.Error(w, http.StatusNotFound, "No sync is currently running")
}

// asError lifts a recovered panic value into an error so End can record it.
func asError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return panicErr{v: v}
}

type panicErr struct{ v any }

func (p panicErr) Error() string {
	if s, ok := p.v.(string); ok {
		return "panic: " + s
	}
	return "panic"
}

// handleSyncBatch kicks off a batch partial sync in the background. It
// mirrors handleSyncFull: the tracker slot is claimed synchronously, the
// engine runs in a goroutine, and the client gets 202 Accepted immediately
// so the batch dialog can close without blocking on a long sync. Progress
// is then polled through /sync/progress and the dashboard's
// SyncProgressPill, with a completion toast on the next idle snapshot.
//
// Returning 409 here keeps the front-end contract identical to /sync/full:
// the caller can detect a concurrent run with the same SYNC_ALREADY_RUNNING
// code instead of inventing a partial-sync-specific error path.
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

	// Snapshot the seeds before handing them to the goroutine so concurrent
	// modifications to the request body slice (highly unlikely but cheap to
	// guard against) cannot race the engine.
	seeds := append([]string(nil), body.RuleNames...)

	rs, syncCtx, ok := s.SyncTracker.Begin(context.Background(), "partial_sync")
	if !ok {
		s.ErrorWith(w, http.StatusConflict, map[string]any{
			"error": "Another sync is already running",
			"code":  "SYNC_ALREADY_RUNNING",
		})
		return
	}
	rs.SetTrigger("manual")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[sync] async partial sync panic: %v", rec)
				s.SyncTracker.End(syncengine.Result{}, asError(rec))
			}
		}()
		res, err := s.Engine.ExecuteBatchPartialSyncReport(syncCtx, seeds, rs)
		s.SyncTracker.End(res, err)
	}()
	s.JSON(w, http.StatusAccepted, map[string]any{
		"status":    "started",
		"jobType":   "partial_sync",
		"startedAt": rs.Snapshot().StartedAt,
		"seeds":     len(seeds),
	})
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
