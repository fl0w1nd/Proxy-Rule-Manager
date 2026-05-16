package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

func (s *Server) registerConsistencyRoutes(r chi.Router) {
	r.Get("/consistency", s.adminGuard(s.handleGetConsistency))
}

// handleGetConsistency returns a read-only inventory of DB/filesystem drift.
// We intentionally do not expose a repair endpoint in this first version —
// the plan calls for detection-only until we trust the classifications.
func (s *Server) handleGetConsistency(w http.ResponseWriter, r *http.Request) {
	report, err := syncengine.CheckConsistency(r.Context(), s.Store, s.Config.RulesDir, s.Config.ClientFileDir)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"issues":  report.Issues,
		"checked": report.Checked,
	})
}
