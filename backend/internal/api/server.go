// Package api provides the HTTP layer (chi router + handlers).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// Version is the application version reported by /api/status.
// set via -ldflags at build time: -ldflags "-X github.com/fl0w1nd/proxy-rule-manager/backend/internal/api.Version=1.2.3"
var Version = "dev"

// Server bundles together the dependencies handlers need.
type Server struct {
	Config      *config.Config
	Store       *store.Store
	Geosite     *geosite.Manager
	Engine      *syncengine.Engine
	RateLimiter *RateLimiter
	AdminToken  string
}

// ApplySystemSettings forwards the user-tunable knobs to the live components
// (fetcher, transformer, rate limiter). Safe to call from any goroutine —
// every component implementation guards its own state. Returns the post-merge
// settings so callers can echo the canonicalised values back to the UI.
func (s *Server) ApplySystemSettings(ss schema.SystemSettings) schema.SystemSettings {
	ss.MergeDefaults()
	if s.Engine != nil && s.Engine.Fetcher != nil {
		s.Engine.Fetcher.Configure(
			time.Duration(ss.Fetch.TimeoutSeconds)*time.Second,
			int64(ss.Fetch.MaxDownloadMB)*1024*1024,
			ss.Fetch.PerHostConcurrency,
			ss.Fetch.UserAgent,
		)
	}
	if s.Engine != nil && s.Engine.Transformer != nil && s.Engine.Transformer.JS != nil {
		s.Engine.Transformer.JS.Configure(transformer.ScriptOptions{
			Timeout:      time.Duration(ss.Transformer.TimeoutMs) * time.Millisecond,
			MaxOutputLen: ss.Transformer.MaxOutputMB * 1024 * 1024,
		})
	}
	if s.RateLimiter != nil {
		s.RateLimiter.Configure(
			time.Duration(ss.RateLimit.BaseDelaySeconds)*time.Second,
			time.Duration(ss.RateLimit.MaxBlockSeconds)*time.Second,
			time.Duration(ss.RateLimit.RecordMaxAgeHours)*time.Hour,
			ss.RateLimit.PermanentBanLimit,
		)
	}
	return ss
}

// New constructs a server.
func New(cfg *config.Config, st *store.Store) *Server {
	mgr := geosite.NewManager(cfg.GeositeDir)
	// Auto-refresh geosite caches that are older than 24h. This bridges the
	// gap between full syncs (which always force-refresh) and partial syncs
	// (which historically reused whatever was on disk forever), so list
	// removals upstream become observable on the next partial sync as well.
	mgr.SetCacheTTL(24 * time.Hour)
	engine := syncengine.NewEngine(st, mgr, cfg.RulesDir)
	return &Server{
		Config:      cfg,
		Store:       st,
		Geosite:     mgr,
		Engine:      engine,
		RateLimiter: NewRateLimiter(),
		AdminToken:  cfg.AdminToken,
	}
}

// Router builds the chi router with all routes registered.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(panicRecoverer)
	r.Use(s.corsMiddleware)

	// API routes — admin auth applied per-route (public paths skip it).
	r.Route("/api", func(api chi.Router) {
		s.registerAuthRoutes(api)
		s.registerStatusRoutes(api)
		s.registerConfigRoutes(api)
		s.registerRuleRoutes(api)
		s.registerSyncRoutes(api)
		s.registerActivityRoutes(api)
		s.registerClientRoutes(api)
		s.registerClientFileRoutes(api)
		s.registerWAFRoutes(api)
		s.registerCDNSettingsRoutes(api)
		s.registerGeositeRoutes(api)
		s.registerIconSetRoutes(api)
		s.registerInitRoutes(api)
		s.registerPreviewRoutes(api)
		s.registerSystemRoutes(api)
		s.registerConsistencyRoutes(api)
	})

	// Public rule file serving (no admin required, but CDN headers applied).
	r.Get("/Rules/{client}/{file}", s.handleRuleFile)
	r.Get("/Rules/{client}/geosite/{provider}/{file}", s.handleGeositeRuleFile)
	r.Get("/IconSet/{filename}", s.handleIconSet)
	r.Get("/client/{clientId}/{file}", s.handleClientFile)

	// SPA + static assets.
	r.HandleFunc("/*", s.serveStatic)
	return r
}

// JSON writes a JSON response with the given status code.
func (s *Server) JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// fall through; nothing we can do once headers are committed
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
	}
}

// JSONList writes a JSON object with key: slice. A nil or typed-nil slice is
// coerced to [] so the client always receives an array, never null.
// Other agents should use this helper instead of JSON for list responses.
func (s *Server) JSONList(w http.ResponseWriter, key string, slice any) {
	if isNilSlice(slice) {
		s.JSON(w, http.StatusOK, map[string]any{key: json.RawMessage("[]")})
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{key: slice})
}

func isNilSlice(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Slice && rv.IsNil()
}

// statusErrorCode returns the canonical API error code for an HTTP status.
func statusErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "VALIDATION_ERROR"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}

// Error writes a standard {"error": msg, "code": <inferred>} body.
// The msg is the human-readable display message; the code is derived from status.
// Frontend reads error.error as display message and error.code for typed handling.
func (s *Server) Error(w http.ResponseWriter, status int, msg string) {
	s.JSON(w, status, map[string]any{"error": msg, "code": statusErrorCode(status)})
}

// ErrorWithCode writes an error body with an explicit code override.
func (s *Server) ErrorWithCode(w http.ResponseWriter, status int, code, msg string) {
	s.JSON(w, status, map[string]any{"error": msg, "code": code})
}

// ErrorWith writes a custom error payload.
func (s *Server) ErrorWith(w http.ResponseWriter, status int, payload map[string]any) {
	s.JSON(w, status, payload)
}

// IP extracts the client IP for `r`.
func (s *Server) IP(r *http.Request) string {
	return util.ClientIP(func(name string) string { return r.Header.Get(name) })
}

// DecodeJSON reads a JSON body into target.
// Unknown fields are silently dropped to match TS Zod .strip() behaviour.
func (s *Server) DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("empty body")
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

// adminGuard wraps a handler so admin auth + rate limiting runs first.
func (s *Server) adminGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.applyAdminGuard(w, r) {
			return
		}
		h(w, r)
	}
}

// applyAdminGuard returns true if the request may continue.
func (s *Server) applyAdminGuard(w http.ResponseWriter, r *http.Request) bool {
	ip := s.IP(r)
	ctx := r.Context()
	blocked, retryAfter, _, err := s.RateLimiter.IsBlocked(ctx, s.Store, ip)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return false
	}
	if blocked {
		// Match TS: retryAfter || 60 — permanent bans return retryAfter=0 but
		// we must still send a valid Retry-After header per RFC 7231.
		ra := retryAfter
		if ra <= 0 {
			ra = 60
		}
		w.Header().Set("Retry-After", fmt.Sprintf("%d", ra))
		s.ErrorWith(w, http.StatusTooManyRequests, map[string]any{
			"error":      "Too many failed attempts",
			"code":       "RATE_LIMITED",
			"retryAfter": retryAfter,
			"message":    fmt.Sprintf("请在 %d 秒后重试", retryAfter),
		})
		return false
	}
	auth := r.Header.Get("Authorization")
	if !s.VerifyAdmin(auth) {
		if auth != "" {
			if err := s.RateLimiter.RecordFailure(ctx, s.Store, ip); err != nil {
				fmt.Fprintf(os.Stderr, "record failure: %v\n", err)
			}
		}
		// Emit {"error": "Unauthorized", "code": "UNAUTHORIZED"} — no extra message field.
		s.Error(w, http.StatusUnauthorized, "Unauthorized")
		return false
	}
	s.RateLimiter.Clear(ip)
	return true
}

// corsMiddleware sets CORS headers with two distinct modes:
//
//  1. ALLOWED_ORIGINS unset (default) — permissive: respond with
//     Access-Control-Allow-Origin: * and never enable credentials. This is
//     safe because all authenticated endpoints require a Bearer token in
//     the Authorization header, which browsers do not auto-attach
//     cross-origin; an attacker page therefore cannot piggyback on a
//     user's logged-in session.
//
//  2. ALLOWED_ORIGINS set — strict allow-list: only origins on the list
//     get their Origin echoed back, and only those origins receive
//     Access-Control-Allow-Credentials: true. Requests from other origins
//     get no CORS headers at all, so the browser blocks them.
//
// Note: cdn_headers.go must not overwrite Access-Control-Allow-Origin.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := s.Config.AllowedOrigins

		switch {
		case len(allowed) == 0:
			// Permissive default. `*` is incompatible with credentials
			// per the Fetch spec, which is exactly the safety we want.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case origin != "" && originAllowed(origin, allowed):
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		default:
			// Origin not on the allow-list: emit no Allow-Origin header
			// so the browser rejects the response. Still let preflight
			// terminate quickly rather than forwarding to the handler.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

// serveStatic serves files from cfg.OutDir, with SPA fallback to index.html.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.Error(w, http.StatusNotFound, "Not Found")
		return
	}
	clean := filepath.Clean(r.URL.Path)
	if clean == "/" || clean == "" {
		clean = "/index.html"
	}
	full, err := util.JoinInside(s.Config.OutDir, clean)
	if err != nil {
		s.Error(w, http.StatusBadRequest, "invalid path")
		return
	}
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	// SPA fallback.
	index := filepath.Join(s.Config.OutDir, "index.html")
	if _, err := os.Stat(index); err == nil {
		http.ServeFile(w, r, index)
		return
	}
	s.Error(w, http.StatusNotFound, "Not Found")
}

// ----- public route stubs (to be implemented per route file) -----

func (s *Server) handleRuleFile(w http.ResponseWriter, r *http.Request) { s.serveRuleFile(w, r, false) }
func (s *Server) handleGeositeRuleFile(w http.ResponseWriter, r *http.Request) {
	s.serveRuleFile(w, r, true)
}
func (s *Server) handleIconSet(w http.ResponseWriter, r *http.Request) { s.serveIconSet(w, r) }
func (s *Server) handleClientFile(w http.ResponseWriter, r *http.Request) {
	s.servePublicClientFile(w, r)
}

// PingDB ensures the database is reachable.
func (s *Server) PingDB(ctx context.Context) error {
	return s.Store.DB.PingContext(ctx)
}

// panicRecoverer is a chi-compatible middleware that catches panics, logs them
// (with full stack trace when DEBUG_PANIC=1), and writes a structured 500
// response without leaking internals to the client.
func panicRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if os.Getenv("DEBUG_PANIC") == "1" {
					buf := make([]byte, 64<<10)
					n := runtime.Stack(buf, false)
					log.Printf("PANIC [%s %s]: %v\n%s", r.Method, r.URL.Path, rec, buf[:n])
				} else {
					log.Printf("PANIC [%s %s]: %v", r.Method, r.URL.Path, rec)
				}
				// Write the structured error directly — response helpers may not be
				// available if the panic occurred inside them.
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"Internal server error","code":"INTERNAL_ERROR"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
