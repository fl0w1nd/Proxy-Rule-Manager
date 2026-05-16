package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) registerAuthRoutes(r chi.Router) {
	r.Get("/auth/required", func(w http.ResponseWriter, r *http.Request) {
		s.JSON(w, http.StatusOK, map[string]any{"required": s.AdminToken != ""})
	})

	r.Post("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Token string `json:"token"`
		}
		if err := s.DecodeJSON(r, &payload); err != nil {
			s.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		ip := s.IP(r)
		ctx := r.Context()
		blocked, retryAfter, _, err := s.RateLimiter.IsBlocked(ctx, s.Store, ip)
		if err != nil {
			s.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if blocked {
			s.ErrorWith(w, http.StatusTooManyRequests, map[string]any{
				"error":      "Too many failed attempts",
				"retryAfter": retryAfter,
			})
			return
		}
		if s.AdminToken == "" {
			s.JSON(w, http.StatusOK, map[string]any{"success": true})
			return
		}
		if strings.TrimSpace(payload.Token) != s.AdminToken {
			_ = s.RateLimiter.RecordFailure(ctx, s.Store, ip)
			s.Error(w, http.StatusUnauthorized, "Invalid token")
			return
		}
		s.RateLimiter.Clear(ip)
		s.JSON(w, http.StatusOK, map[string]any{"success": true, "token": payload.Token})
	})

	r.Post("/auth/verify", s.adminGuard(func(w http.ResponseWriter, r *http.Request) {
		s.JSON(w, http.StatusOK, map[string]any{"success": true})
	}))
}
