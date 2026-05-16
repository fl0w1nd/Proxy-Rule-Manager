package api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

func (s *Server) registerWAFRoutes(r chi.Router) {
	// Public IP echo endpoint.
	r.Get("/waf/my-ip", func(w http.ResponseWriter, r *http.Request) {
		s.JSON(w, http.StatusOK, map[string]any{"ip": s.IP(r)})
	})
	r.Get("/waf/bans", s.adminGuard(s.handleListBans))
	r.Post("/waf/bans", s.adminGuard(s.handleCreateBan))
	r.Delete("/waf/bans/{ip}", s.adminGuard(s.handleDeleteBan))
	r.Get("/waf/stats", s.adminGuard(s.handleWAFStats))
	r.Get("/waf/failures", s.adminGuard(s.handleWAFFailures))
	r.Post("/waf/cleanup", s.adminGuard(s.handleWAFCleanup))
}

func (s *Server) handleListBans(w http.ResponseWriter, r *http.Request) {
	bans, err := s.Store.GetAllBans(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if bans == nil {
		bans = []schema.BanRecord{}
	}
	s.JSON(w, http.StatusOK, map[string]any{"bans": bans})
}

func (s *Server) handleCreateBan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP              string `json:"ip"`
		Reason          string `json:"reason"`
		Permanent       bool   `json:"permanent"`
		DurationSeconds int64  `json:"durationSeconds"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.IP == "" {
		s.Error(w, http.StatusBadRequest, "IP address is required")
		return
	}
	now := time.Now().UTC()
	rec := schema.BanRecord{
		IP:       body.IP,
		Reason:   firstNonEmpty(body.Reason, "manual_ban"),
		BannedAt: util.FormatISO(now),
	}
	if !body.Permanent {
		dur := body.DurationSeconds
		if dur <= 0 {
			dur = 3600
		}
		expires := util.FormatISO(now.Add(time.Duration(dur) * time.Second))
		rec.ExpiresAt = &expires
	}
	if err := s.Store.UpsertBan(r.Context(), rec); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "IP " + body.IP + " has been banned"})
}

func (s *Server) handleDeleteBan(w http.ResponseWriter, r *http.Request) {
	ip, _ := url.PathUnescape(chi.URLParam(r, "ip"))
	removed, err := s.Store.RemoveBan(r.Context(), ip)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !removed {
		s.Error(w, http.StatusNotFound, "IP not found in ban list")
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "IP " + ip + " has been unbanned"})
}

func (s *Server) handleWAFStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.Store.GetBanStats(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	failures := s.RateLimiter.Snapshot()
	now := time.Now()
	var currentlyBlocked int
	for _, rec := range failures {
		blockDur := s.RateLimiter.calculateBlockDuration(rec.Count)
		until := rec.LastFailedAt.Add(blockDur)
		if now.Before(until) {
			currentlyBlocked++
		}
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"bans": stats,
		"temporary": map[string]any{
			"totalTracked":     len(failures),
			"currentlyBlocked": currentlyBlocked,
		},
	})
}

func (s *Server) handleWAFFailures(w http.ResponseWriter, r *http.Request) {
	failures := s.RateLimiter.Snapshot()
	now := time.Now()
	type item struct {
		IP            string  `json:"ip"`
		FailCount     int     `json:"failCount"`
		LastFailedAt  string  `json:"lastFailedAt"`
		BlockDuration int64   `json:"blockDuration"`
		IsBlocked     bool    `json:"isBlocked"`
		BlockedUntil  *string `json:"blockedUntil"`
	}
	out := make([]item, 0, len(failures))
	for ip, rec := range failures {
		dur := s.RateLimiter.calculateBlockDuration(rec.Count)
		until := rec.LastFailedAt.Add(dur)
		blocked := now.Before(until)
		entry := item{
			IP:            ip,
			FailCount:     rec.Count,
			LastFailedAt:  util.FormatISO(rec.LastFailedAt),
			BlockDuration: int64(dur.Seconds()),
			IsBlocked:     blocked,
		}
		if blocked {
			ts := util.FormatISO(until)
			entry.BlockedUntil = &ts
		}
		out = append(out, entry)
	}
	s.JSON(w, http.StatusOK, map[string]any{"failures": out})
}

func (s *Server) handleWAFCleanup(w http.ResponseWriter, r *http.Request) {
	removed, err := s.Store.CleanupExpiredBans(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Cleaned up %d expired bans", removed),
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
