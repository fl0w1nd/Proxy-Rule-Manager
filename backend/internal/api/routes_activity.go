package api

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

var dateKey = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (s *Server) registerActivityRoutes(r chi.Router) {
	r.Get("/activity/changes", s.adminGuard(s.handleActivityChanges))
	r.Get("/activity/changes/{date}/{fileName}", s.adminGuard(s.handleActivityChangeDiff))
	r.Get("/activity/failures", s.adminGuard(s.handleActivityFailures))
	r.Get("/activity/failing-sources", s.adminGuard(s.handleActivityFailingSources))
	r.Get("/activity/dates", s.adminGuard(s.handleActivityDates))
	r.Post("/activity/clear", s.adminGuard(s.handleActivityClear))
}

func parsePage(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	v, err := strconv.Atoi(value)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parsePageSize(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	v, err := strconv.Atoi(value)
	if err != nil || v <= 0 {
		return fallback
	}
	if v > 100 {
		return 100
	}
	return v
}

func (s *Server) handleActivityChanges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")
	if date != "" && !dateKey.MatchString(date) {
		s.Error(w, http.StatusBadRequest, "Invalid date format")
		return
	}
	page := parsePage(q.Get("page"), 1)
	pageSize := parsePageSize(q.Get("pageSize"), 20)
	days := 30
	if v := q.Get("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}
	client := q.Get("client")
	res, err := s.Store.ListChangeRecords(r.Context(), date, page, pageSize, client, days)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.Items == nil {
		res.Items = []schema.ChangeRecordSummary{}
	}
	s.JSON(w, http.StatusOK, res)
}

func (s *Server) handleActivityFailures(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")
	if date != "" && !dateKey.MatchString(date) {
		s.Error(w, http.StatusBadRequest, "Invalid date format")
		return
	}
	page := parsePage(q.Get("page"), 1)
	pageSize := parsePageSize(q.Get("pageSize"), 20)
	days := 30
	if v := q.Get("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}
	client := q.Get("client")
	res, err := s.Store.ListFailureRecords(r.Context(), date, page, pageSize, client, days)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.Items == nil {
		res.Items = []schema.FailureRecord{}
	}
	s.JSON(w, http.StatusOK, res)
}

func (s *Server) handleActivityChangeDiff(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	fileName, err := url.PathUnescape(chi.URLParam(r, "fileName"))
	if err != nil {
		s.Error(w, http.StatusBadRequest, "Invalid file name")
		return
	}
	if !dateKey.MatchString(date) {
		s.Error(w, http.StatusBadRequest, "Invalid date format")
		return
	}
	diff, err := s.Store.GetChangeDiff(r.Context(), date, fileName)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"diff": diff})
}

func (s *Server) handleActivityDates(w http.ResponseWriter, r *http.Request) {
	dates, err := s.Store.ListActivityDates(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dates == nil {
		dates = []string{}
	}
	s.JSON(w, http.StatusOK, map[string]any{"dates": dates})
}

// handleActivityFailingSources powers the "本周失败源 Top N" dashboard widget.
// Defaults to days=7 and limit=5 — both query params; both are clamped server-
// side (days to the activity retention window, limit to 100) to bound work.
func (s *Server) handleActivityFailingSources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	days := 7
	if v := q.Get("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			days = d
		}
	}
	limit := 5
	if v := q.Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	rows, err := s.Store.ListFailingSources(r.Context(), days, limit)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSONList(w, "sources", rows)
}

func (s *Server) handleActivityClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearActivity(r.Context()); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true})
}
