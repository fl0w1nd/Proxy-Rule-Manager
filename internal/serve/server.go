// Package serve provides public file serving and the authenticated management API.
package serve

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

// Server is the HTTP server.
type Server struct {
	DataDir string
	Config  *config.Config
	State   *state.Store
	Engine  *engine.UpdateEngine

	updates        *updates.Manager
	apiToken       string
	trustedProxies []netip.Prefix

	configFile  string
	configMtime time.Time
	mu          sync.RWMutex // protects Config and configMtime
}

// Options contains immutable HTTP server runtime settings.
type Options struct {
	DataDir        string
	APIToken       string
	ConfigFile     string
	TrustedProxies []netip.Prefix
}

// NewServer creates a new HTTP server. Runtime settings stay fixed for the
// process lifetime while the business config may be hot-reloaded.
func NewServer(
	cfg *config.Config,
	st *state.Store,
	eng *engine.UpdateEngine,
	updateManager *updates.Manager,
	opts Options,
) *Server {
	s := &Server{
		DataDir: opts.DataDir, Config: cfg, State: st, Engine: eng,
		updates: updateManager, apiToken: opts.APIToken,
		trustedProxies: append([]netip.Prefix(nil), opts.TrustedProxies...),
		configFile:     opts.ConfigFile,
	}
	if opts.ConfigFile != "" {
		if fi, err := os.Stat(opts.ConfigFile); err == nil {
			s.configMtime = fi.ModTime()
		}
	}
	return s
}

// config returns a snapshot of the current config, safe for concurrent access.
// The returned pointer is immutable; reload swaps in a new *config.Config.
func (s *Server) config() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Config
}

// Handler returns the chi router with all routes configured.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	// proxyAware runs before middleware.RealIP so it sees the real TCP peer
	// (RealIP rewrites RemoteAddr from forwarded headers). It stashes the
	// proxy-aware HTTPS decision in the request context for cookie Secure and
	// same-origin checks.
	r.Use(s.proxyAware)
	r.Use(securityHeaders)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Public: static rule artifacts
	rulesDir := filepath.Join(s.DataDir, "rules")
	r.Handle("/rules/*", http.StripPrefix("/rules/", http.FileServer(http.Dir(rulesDir))))

	// Public: generated pages and static assets (icons, etc.)
	iconsDir := filepath.Join(s.DataDir, "static", "icons")
	r.Handle("/static/icons/*", http.StripPrefix("/static/icons/", http.FileServer(http.Dir(iconsDir))))
	r.Get("/", s.handleSitePage("static/index.html"))
	r.Get("/index.html", s.handleSitePage("static/index.html"))

	// Admin board (Bearer or login query token; unauthorized browsers get a token gate)
	r.Get("/admin", s.handleAdminPage)
	r.Get("/admin.html", s.handleAdminPage)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.bearerGuard)
		r.Use(noStore)
		r.Get("/status", s.handleStatus)
		r.Get("/rules", s.handleRules)
		r.Get("/geosite/providers", s.handleGeositeProviders)
		r.Get("/changes", s.handleChanges)
		r.Get("/updates", s.handleUpdates)
		r.Post("/updates", s.sameOriginMutation(s.handleCreateUpdate))
		r.Get("/updates/current", s.handleCurrentUpdate)
		r.Get("/updates/{updateID}", s.handleUpdateDetail)
		r.Get("/updates/{updateID}/events", s.handleUpdateEvents)
		r.Post("/updates/{updateID}/cancel", s.sameOriginMutation(s.handleCancelUpdate))
		r.Get("/config/dirty", s.handleConfigDirty)
		r.Post("/config/reload", s.sameOriginMutation(s.handleConfigReload))
	})

	return r
}

// tokenCookieName persists the admin token across browser sessions. Set when
// a valid ?token= reaches /admin; honoured by every protected route.
const tokenCookieName = "prm_token"

// requestToken extracts API authentication from a Bearer header or cookie.
func requestToken(r *http.Request) string {
	const prefix = "Bearer "
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, prefix) {
		return strings.TrimPrefix(auth, prefix)
	}
	if c, err := r.Cookie(tokenCookieName); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) validToken(token string) bool {
	return s.apiToken != "" &&
		subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken)) == 1
}

func (s *Server) tokenValid(r *http.Request) bool {
	return s.validToken(requestToken(r))
}

// setTokenCookie persists the token for 30 days. The Secure flag is set when
// the request is considered HTTPS, including the TLS-terminating-proxy case
// where the scheme is taken from a trusted proxy's Forwarded header.
func setTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isHTTPSRequest(r),
	})
}

// handleAdminPage serves the admin board. A valid ?token= is persisted as a
// cookie and the URL is cleaned via redirect; without a valid token a minimal
// gate page prompts for it.
func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	if q := r.URL.Query().Get("token"); q != "" {
		if s.validToken(q) {
			setTokenCookie(w, r, q)
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		s.serveAdminGate(w, true)
		return
	}
	if !s.tokenValid(r) {
		s.serveAdminGate(w, false)
		return
	}
	page, err := site.AdminPage()
	if err != nil {
		http.Error(w, "admin page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (s *Server) serveAdminGate(w http.ResponseWriter, invalid bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	page := adminGatePage
	if invalid {
		page = strings.Replace(page, `display:none`, `display:block`, 1)
	}
	_, _ = w.Write([]byte(page))
}

// handleSitePage serves one generated page from the data directory.
func (s *Server) handleSitePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(s.DataDir, name)
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			http.Error(w, name+" not found (site generation failed; check serve logs or run prm update)", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, p)
	}
}

// bearerGuard requires a valid API token on every request.
func (s *Server) bearerGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.tokenValid(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "需要有效的管理令牌", map[string]any{})
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sameOriginMutation(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, prefix) && s.validToken(strings.TrimPrefix(auth, prefix)) {
			next(w, r)
			return
		}
		origin, err := url.Parse(r.Header.Get("Origin"))
		expectedScheme := "http"
		if isHTTPSRequest(r) {
			expectedScheme = "https"
		}
		if err != nil || origin.Scheme != expectedScheme || origin.Host != r.Host {
			writeAPIError(w, http.StatusForbidden, "invalid_origin", "写操作需要同源请求", map[string]any{})
			return
		}
		next(w, r)
	}
}

// adminGatePage is the token prompt shown to unauthorized browsers hitting
// /admin. Intentionally standalone (no external assets).
const adminGatePage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>PRM · 管理</title>
<style>
*{box-sizing:border-box}
body{background-color:#11100d;background-image:radial-gradient(circle at 1px 1px,rgba(255,250,240,.05) .7px,transparent .8px);background-size:5px 5px;color:#d8d3c9;font-family:"Space Mono","SF Mono",monospace;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:20px}
.card{background:#181713;border:2px solid #777168;clip-path:polygon(4px 0,calc(100% - 4px) 0,100% 4px,100% calc(100% - 4px),calc(100% - 4px) 100%,4px 100%,0 calc(100% - 4px),0 4px);filter:drop-shadow(7px 7px 0 #000);padding:32px;width:340px}
.k{font-size:11px;letter-spacing:.1em;color:#fffaf0;margin-bottom:18px;padding-bottom:12px;border-bottom:1px dashed #777168}
input{width:100%;background:#0b0c09;border:1px solid #777168;border-radius:0;color:#fff;font:inherit;font-size:13px;padding:11px 12px;outline:none;box-shadow:2px 2px 0 #000}
input:focus{border-color:#ff6418;outline:2px solid #ff6418;outline-offset:2px}
button{margin-top:16px;width:100%;background:#fffaf0;color:#11100d;border:1px solid #fffaf0;border-radius:0;box-shadow:3px 3px 0 #ff6418;font:inherit;font-size:12px;font-weight:700;letter-spacing:.08em;padding:10px 0;cursor:pointer;transition:transform 80ms steps(2,end),box-shadow 80ms steps(2,end)}
button:hover{background:#ff6418;border-color:#ff6418}button:active{transform:translate(3px,3px);box-shadow:0 0 0 #000}
.err{color:#f04438;font-size:11px;margin-top:10px;display:none}
</style></head>
<body><div class="card">
<div class="k">PRM 管理看板 · 输入令牌</div>
<form method="get" action="/admin">
<input type="password" name="token" autofocus autocomplete="off" placeholder="PRM_ADMIN_TOKEN">
<button type="submit">进 入</button>
</form>
<div class="err" id="e">令牌无效</div>
</div>
</body></html>`

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// secureCtxKey is the context key for the proxy-aware HTTPS decision computed
// by proxyAware before chi's RealIP middleware rewrites RemoteAddr.
type secureCtxKey struct{}

// proxyAware determines whether the current request is HTTPS — accounting for
// TLS-terminating reverse proxies — and stores the result in the request
// context. It must run before middleware.RealIP so it inspects the original
// TCP peer rather than the forwarded-for value.
func (s *Server) proxyAware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		https := s.requestIsHTTPS(r)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), secureCtxKey{}, https)))
	})
}

// isHTTPSRequest reports the proxy-aware HTTPS decision stashed by proxyAware.
// Falls back to r.TLS != nil when the middleware did not run (e.g. direct unit
// tests), which preserves the pre-existing direct-TLS behavior.
func isHTTPSRequest(r *http.Request) bool {
	if v, ok := r.Context().Value(secureCtxKey{}).(bool); ok {
		return v
	}
	return r.TLS != nil
}

// requestIsHTTPS decides the effective request scheme. A direct TLS connection
// is HTTPS. Otherwise forwarded scheme headers are honored only when the TCP
// peer is a configured trusted proxy, so a public client cannot spoof HTTPS by
// sending X-Forwarded-Proto.
func (s *Server) requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !s.trustedProxy(clientHost(r.RemoteAddr)) {
		return false
	}
	return forwardedProto(r) == "https"
}

// trustedProxy reports whether peer falls within any configured trusted proxy
// range. With no ranges configured, nothing is trusted.
func (s *Server) trustedProxy(peer string) bool {
	if len(s.trustedProxies) == 0 || peer == "" {
		return false
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return false
	}
	for _, p := range s.trustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// clientHost strips the port from a RemoteAddr, returning the bare host. If
// SplitHostPort fails (no port), the input is returned unchanged.
func clientHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// forwardedProto extracts the origin scheme from RFC 7239 Forwarded (proto
// parameter) or, failing that, the first value of X-Forwarded-Proto. Only the
// leftmost X-Forwarded-Proto value is used, matching a single trusted proxy
// tier in front of prm.
func forwardedProto(r *http.Request) string {
	if f := r.Header.Get("Forwarded"); f != "" {
		for _, pair := range strings.Split(f, ";") {
			pair = strings.TrimSpace(pair)
			if len(pair) >= 6 && strings.EqualFold(pair[:6], "proto=") {
				return strings.Trim(pair[6:], "\"'")
			}
		}
	}
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
		if i := strings.IndexByte(xfp, ','); i >= 0 {
			xfp = xfp[:i]
		}
		return strings.TrimSpace(xfp)
	}
	return ""
}

// securityHeaders applies baseline defensive response headers to every
// response (public pages, admin board, and API). CSP is intentionally omitted:
// the generated pages embed inline scripts/styles and load Google Fonts, so a
// strict CSP would break them. Add CSP only after aligning the page assets.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
