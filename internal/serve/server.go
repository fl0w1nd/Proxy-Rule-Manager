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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fl0w1nd/proxy-rule-manager/internal/admin"
	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

// Server is the HTTP server.
type Server struct {
	DataDir       string
	ConfigManager *config.Manager
	State         *state.Store
	Engine        *engine.UpdateEngine

	updates        *updates.Manager
	apiToken       string
	trustedProxies []netip.Prefix
	devMode        bool

	configFile string
}

// Options contains immutable HTTP server runtime settings.
type Options struct {
	DataDir        string
	APIToken       string
	ConfigFile     string
	ConfigManager  *config.Manager
	TrustedProxies []netip.Prefix
	DevMode        bool
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
	configManager := opts.ConfigManager
	if configManager == nil {
		configManager = config.NewMemoryManager(cfg)
	}
	s := &Server{
		DataDir: opts.DataDir, ConfigManager: configManager, State: st, Engine: eng,
		updates: updateManager, apiToken: opts.APIToken,
		trustedProxies: append([]netip.Prefix(nil), opts.TrustedProxies...),
		devMode:        opts.DevMode,
		configFile:     opts.ConfigFile,
	}
	return s
}

// config returns an independent snapshot of the current runtime config.
func (s *Server) config() *config.Config {
	cfg, _ := s.ConfigManager.Snapshot()
	return cfg
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
	assetsDir := filepath.Join(s.DataDir, "static", "assets")
	r.Handle("/static/assets/*", http.StripPrefix("/static/assets/", http.FileServer(http.Dir(assetsDir))))
	r.Get("/", s.handleSitePage("static/index.html"))
	r.Get("/index.html", s.handleSitePage("static/index.html"))

	// Admin board (Bearer or login query token; unauthorized browsers get a token gate)
	r.Get("/admin", s.handleAdminPage)
	r.Get("/admin.html", s.handleAdminPage)
	r.Get("/admin/*", s.handleAdminPage)

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
		r.Get("/config", s.handleConfig)
		r.Get("/config/raw", s.handleConfigRaw)
		r.Post("/config/raw", s.sameOriginMutation(s.handleConfigRawSave))
		r.Post("/config/validate", s.sameOriginMutation(s.handleConfigValidate))
		r.Post("/config/patch", s.sameOriginMutation(s.handleConfigPatch))
		r.Get("/config/dirty", s.handleConfigDirty)
		r.Post("/config/reload", s.sameOriginMutation(s.handleConfigReload))
		r.Get("/config/backups", s.handleConfigBackups)
		r.Get("/config/backups/{backupID}", s.handleConfigBackup)
		r.Post("/config/backups/{backupID}/restore", s.sameOriginMutation(s.handleConfigBackupRestore))
		r.Get("/local-files", s.handleLocalFiles)
		r.Post("/local-files", s.sameOriginMutation(s.handleLocalFileCreate))
		r.Get("/local-files/{name}", s.handleLocalFile)
		r.Put("/local-files/{name}", s.sameOriginMutation(s.handleLocalFileUpdate))
		r.Delete("/local-files/{name}", s.sameOriginMutation(s.handleLocalFileDelete))
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
	if s.devMode {
		return true
	}
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
	if s.devMode {
		admin.Handler().ServeHTTP(w, r)
		return
	}
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
	admin.Handler().ServeHTTP(w, r)
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
		if s.devMode {
			next(w, r)
			return
		}
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
body{background-color:#1c2820;background-image:radial-gradient(rgb(174 182 168 / 12%) .5px,transparent .65px);background-size:4px 4px;color:#eeeadd;font-family:"PingFang SC","Microsoft YaHei",system-ui,sans-serif;font-size:12px;line-height:20px;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:20px}
.card{background:#202522;border:1px solid #aeb6a8;border-radius:4px;box-shadow:0 26px 34px -24px rgb(0 0 0 / 56%);padding:24px;width:340px}
.k{font-size:12px;color:#fffdf1;margin-bottom:18px;padding-bottom:12px;border-bottom:1px solid #aeb6a8}
input{width:100%;min-height:32px;background:#202522;border:1px solid #aeb6a8;border-radius:3px;color:#eeeadd;font:inherit;padding:4px 8px;outline:none;box-shadow:inset 1px 1px 0 #111713;caret-color:#eeeadd}
input:focus{outline:2px solid #afc7ff;outline-offset:3px}
input::placeholder{color:#b1baab}
button{margin-top:16px;width:100%;min-height:40px;background:#4b6549;color:#eeeadd;border:1px solid #aeb6a8;border-radius:4px;box-shadow:inset 1px 1px 0 #5b645a,inset -1px -1px 0 #111713,0 1px 0 #aeb6a8;font:inherit;padding:0;cursor:pointer}
button:hover{background:#587653}button:active{box-shadow:inset 1px 1px 0 #111713,inset -1px -1px 0 #5b645a;transform:translateY(1px)}
.err{color:#eeeadd;background:#663d38;border:1px solid #aeb6a8;border-radius:3px;font-size:12px;margin-top:10px;padding:6px 8px;display:none}
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
// the generated pages embed inline scripts and styles, so a
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
