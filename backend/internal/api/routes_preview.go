package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

// normalizePreviewResult ensures slice/map fields are never nil so that JSON
// serialisation always emits {} and [] rather than null.
func normalizePreviewResult(res *syncengine.PreviewResult) {
	if res.Contents == nil {
		res.Contents = make(map[string]string)
	}
	if res.Diagnostics.SourceResults == nil {
		res.Diagnostics.SourceResults = []syncengine.PreviewSource{}
	}
}

func (s *Server) registerPreviewRoutes(r chi.Router) {
	r.Post("/preview", s.adminGuard(s.handlePreviewRule))
}

type previewRequest struct {
	RuleName   string             `json:"ruleName"`
	Rule       *schema.RuleConfig `json:"rule"`
	LimitLines int                `json:"limitLines"`
}

// previewLimitLines clamps the requested limit to a safe range. <=0 means
// "use the default cap" so we never accidentally serve a multi-megabyte
// preview that locks up the browser.
const (
	previewDefaultLimitLines = 10000
	previewMaxLimitLines     = 50000
)

func resolvePreviewLimit(requested int) int {
	if requested <= 0 {
		return previewDefaultLimitLines
	}
	if requested > previewMaxLimitLines {
		return previewMaxLimitLines
	}
	return requested
}

func (s *Server) handlePreviewRule(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := s.DecodeJSON(r, &req); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var rule *schema.RuleConfig
	if req.RuleName != "" {
		for i := range cfg.Rules {
			if cfg.Rules[i].Name == req.RuleName {
				rule = &cfg.Rules[i]
				break
			}
		}
		if rule == nil {
			s.Error(w, http.StatusNotFound, "Rule \""+req.RuleName+"\" not found")
			return
		}
	} else if req.Rule != nil {
		if err := validateRulePayload(req.Rule); err != nil {
			s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		rule = req.Rule
	} else {
		s.Error(w, http.StatusBadRequest, "Either ruleName or rule config is required")
		return
	}
	res, err := s.Engine.PreviewRule(r.Context(), rule, cfg.Transformers, resolvePreviewLimit(req.LimitLines))
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	normalizePreviewResult(&res)
	s.JSON(w, http.StatusOK, map[string]any{
		"contents":    res.Contents,
		"diagnostics": res.Diagnostics,
	})
}
