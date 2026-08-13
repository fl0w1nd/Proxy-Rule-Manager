package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
	"github.com/fl0w1nd/proxy-rule-manager/version"
)

func testServer(t *testing.T) (*Server, *config.Config, *state.Store) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{
		Clients: []config.ClientConfig{{ID: "surge", Name: "Surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "apple", Name: "Apple", Sources: []config.SourceConfig{{Content: "DOMAIN,apple.example"}}, Outputs: []string{"surge"}},
			{ID: "child", Name: "Child", Sources: []config.SourceConfig{{Ref: "apple"}}, Outputs: []string{"surge"}},
		},
	}
	cfg.Defaults()
	st, err := state.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.EnsureArtifactDirs(dataDir, []string{"surge"}); err != nil {
		t.Fatal(err)
	}
	eng := &engine.UpdateEngine{DataDir: dataDir, Config: cfg, Registry: render.NewRegistry(), Fetcher: engine.NewFetcher(), Preprocessor: engine.NewPreprocessRunner(), State: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	manager, err := updates.NewManager(cfg, dataDir, st, eng, eng.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(cfg, st, eng, manager, Options{DataDir: dataDir, APIToken: "abc"}), cfg, st
}

func authorized(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer abc")
	return req
}

func TestAPITokenGuardAcceptsBearerAndCookie(t *testing.T) {
	s, _, _ := testServer(t)
	for _, test := range []struct {
		name    string
		prepare func(*http.Request)
		want    int
	}{
		{name: "bearer", prepare: func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") }, want: http.StatusOK},
		{name: "cookie", prepare: func(r *http.Request) { r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"}) }, want: http.StatusOK},
		{name: "raw authorization", prepare: func(r *http.Request) { r.Header.Set("Authorization", "abc") }, want: http.StatusUnauthorized},
		{name: "query token", prepare: func(r *http.Request) {}, want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := "/api/v1/status"
			if test.name == "query token" {
				target += "?token=abc"
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			test.prepare(req)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != test.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("cache control=%q", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestAdminGateSetsHttpOnlyCookieAndServesEmbeddedApp(t *testing.T) {
	s, _, _ := testServer(t)
	legacy := filepath.Join(s.DataDir, "static", "admin.html")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("LEGACY"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `name="token"`) {
		t.Fatalf("gate=%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin?token=abc", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("redirect=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie=%+v", cookies)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "LEGACY") || !strings.Contains(rec.Body.String(), "var API = '/api/v1'") {
		t.Fatalf("admin=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control=%q", rec.Header().Get("Cache-Control"))
	}
}

func TestPublicRoutesAreNarrowed(t *testing.T) {
	s, _, _ := testServer(t)
	staticDir := filepath.Join(s.DataDir, "static")
	if err := os.MkdirAll(filepath.Join(staticDir, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "admin.html"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "icons", "prm.svg"), []byte("icon"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/static/admin.html", "/api/status", "/api/logs", "/api/update"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, authorized(http.MethodGet, target, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", target, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/icons/prm.svg", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "icon" {
		t.Fatalf("icon=%d %q", rec.Code, rec.Body.String())
	}
}

func TestCookieMutationRequiresSameOrigin(t *testing.T) {
	s, _, _ := testServer(t)
	makeRequest := func(origin string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/updates", strings.NewReader(`{"scope":"all"}`))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		return req
	}
	for _, origin := range []string{"", "https://attacker.example"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, makeRequest(origin))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("origin %q status=%d body=%s", origin, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, makeRequest("http://example.com"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("same origin=%d body=%s", rec.Code, rec.Body.String())
	}
	if job := s.updates.Current(); job != nil {
		select {
		case <-job.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("update did not finish")
		}
	}
}

func TestStatusAndRulesHaveFocusedContracts(t *testing.T) {
	s, _, st := testServer(t)
	checked := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	st.SetLastCheck(checked)
	st.SetRuleCheck("apple", state.RuleUpdated, checked, true)
	st.SetRuleEntryCount("apple", 42)
	if err := os.WriteFile(filepath.Join(s.DataDir, "rules", "surge", "apple.list"), []byte("DOMAIN,apple.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/status", nil))
	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status) != 4 || status["published_artifacts"].(float64) != 1 || status["last_check"] != "2026-08-12T09:30:00.000Z" {
		t.Fatalf("status=%v", status)
	}
	if status["version"] != version.Current() {
		t.Fatalf("status version=%v, want %v", status["version"], version.Current())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/rules", nil))
	var rules rulesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatal(err)
	}
	if rules.Total != 2 || rules.Items[0].ID != "apple" || rules.Items[0].Entries != 42 || rules.Items[0].LastCheck == nil {
		t.Fatalf("rules=%+v", rules)
	}
}

func TestCreateUpdateStrictValidationAndConflictShape(t *testing.T) {
	s, _, _ := testServer(t)
	for _, test := range []struct {
		name, body, contentType, code string
		want                          int
	}{
		{name: "unknown field", body: `{"scope":"all","extra":1}`, contentType: "application/json", want: 422, code: "invalid_request"},
		{name: "unknown rule", body: `{"scope":"rules","rule_ids":["missing"]}`, contentType: "application/json", want: 422, code: "invalid_rule_ids"},
		{name: "rules empty", body: `{"scope":"rules","rule_ids":[]}`, contentType: "application/json", want: 422, code: "invalid_rule_ids"},
		{name: "all with ids", body: `{"scope":"all","rule_ids":["apple"]}`, contentType: "application/json", want: 422, code: "invalid_update_scope"},
		{name: "content type", body: `{"scope":"all"}`, contentType: "text/plain", want: 415, code: "unsupported_media_type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := authorized(http.MethodPost, "/api/v1/updates", strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != test.want || !strings.Contains(rec.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdateHistoryDetailChangesAndCursor(t *testing.T) {
	s, cfg, st := testServer(t)
	now := time.Now().UTC()
	for i, id := range []string{"first", "second", "third"} {
		finished := now.Add(time.Duration(i) * time.Minute)
		record := state.UpdateHistoryRecord{ID: id, Origin: "cli", Scope: "rules", RequestedRuleIDs: []string{"apple"}, EffectiveRuleIDs: []string{"apple", "child"}, Status: "completed", StartedAt: finished.Add(-time.Second).Format(time.RFC3339), FinishedAt: finished.Format(time.RFC3339), RulesTotal: 2, RulesSucceeded: 2}
		if id == "third" {
			record.Changes = []state.RuleChangeRecord{{
				RuleID: "apple", RuleName: "Apple",
				Files:        []state.ArtifactChangeRecord{{ClientID: "surge", Path: "rules/surge/apple.list", Change: "updated"}},
				Added:        1,
				AddedSamples: []string{"domain,apple.example"},
			}}
		}
		st.PutUpdateHistory(record, time.Duration(cfg.Update.HistoryRetention), cfg.Update.HistoryLimit, now.Add(5*time.Minute))
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates?limit=2", nil))
	var page updatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "third" || page.NextCursor == "" {
		t.Fatalf("page=%+v", page)
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates?limit=2&cursor="+page.NextCursor, nil))
	var next updatesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &next)
	if len(next.Items) != 1 || next.Items[0].ID != "first" {
		t.Fatalf("next=%+v", next)
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates/third", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"effective_rule_ids":["apple","child"]`) {
		t.Fatalf("detail=%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/changes", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"change":"updated"`) || !strings.Contains(rec.Body.String(), `"added_samples":["domain,apple.example"]`) {
		t.Fatalf("changes=%d %s", rec.Code, rec.Body.String())
	}
}

func TestCurrentIdleAndExpiredEvents(t *testing.T) {
	s, cfg, st := testServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates/current", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("current status=%d body=%s", rec.Code, rec.Body.String())
	}
	now := time.Now().UTC()
	st.PutUpdateHistory(state.UpdateHistoryRecord{ID: "expired", Status: "completed", StartedAt: now.Add(-time.Second).Format(time.RFC3339), FinishedAt: now.Format(time.RFC3339)}, time.Duration(cfg.Update.HistoryRetention), cfg.Update.HistoryLimit, now)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates/expired/events", nil))
	if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), `"code":"update_events_expired"`) {
		t.Fatalf("expired status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/updates/expired", nil))
	for _, field := range []string{`"requested_rule_ids":[]`, `"effective_rule_ids":[]`, `"warnings":[]`, `"issues":[]`, `"changes":[]`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("detail missing %s: %s", field, rec.Body.String())
		}
	}
}

type blockingAPIRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingAPIRunner) FullUpdate(ctx context.Context) engine.UpdateResult {
	close(r.started)
	start := time.Now()
	<-ctx.Done()
	<-r.release
	return engine.UpdateResult{StartTime: start, EndTime: time.Now(), Errors: []string{"update cancelled"}, Issues: []engine.UpdateIssue{{Stage: "cancel", Subject: "update", Message: "update cancelled"}}}
}
func (r *blockingAPIRunner) PartialUpdate(ctx context.Context, _ []string) engine.UpdateResult {
	return r.FullUpdate(ctx)
}

func TestCreateConflictAndCancelAPI(t *testing.T) {
	s, cfg, st := testServer(t)
	runner := &blockingAPIRunner{started: make(chan struct{}), release: make(chan struct{})}
	manager, err := updates.NewManager(cfg, s.DataDir, st, runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.updates = manager
	job, err := manager.Start(updates.Request{Scope: "all"}, "scheduled")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started

	req := authorized(http.MethodPost, "/api/v1/updates", strings.NewReader(`{"scope":"all"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"current_update_id":"`+job.ID+`"`) {
		t.Fatalf("conflict=%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodPost, "/api/v1/updates/"+job.ID+"/cancel", nil))
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"status":"cancelling"`) {
		t.Fatalf("cancel=%d %s", rec.Code, rec.Body.String())
	}
	close(runner.release)
	select {
	case <-job.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancel timeout")
	}
}

func TestUpdateEventsResumeFromLastEventID(t *testing.T) {
	s, _, _ := testServer(t)
	job, err := s.updates.Start(updates.Request{Scope: "rules", RuleIDs: []string{"apple"}}, "web")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("update timeout")
	}
	req := authorized(http.MethodGet, "/api/v1/updates/"+job.ID+"/events", nil)
	req.Header.Set("Last-Event-ID", "1")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "id: 1\n") || !strings.Contains(body, "id: 2\n") || !strings.Contains(body, "event: complete") {
		t.Fatalf("events=%d %s", rec.Code, body)
	}
}

func TestRulesRouteUsesPublishedPathLayout(t *testing.T) {
	s, _, _ := testServer(t)
	targetDir := filepath.Join(s.DataDir, "rules", "Clash Meta", "geosite", "v2fly")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "geolocation-!cn@ads.list"), []byte("DOMAIN,example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rules/Clash%20Meta/geosite/v2fly/geolocation-!cn%40ads.list", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "DOMAIN,example.com\n" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, 422, "invalid_rule_ids", "请求包含未知规则", map[string]any{"rule_ids": []string{"bad"}})
	var body map[string]map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"]["code"] != "invalid_rule_ids" || body["error"]["message"] != "请求包含未知规则" {
		t.Fatalf("body=%v", body)
	}
}

// withTrustedProxy returns a Server whose trusted proxy list includes the
// loopback range, simulating a TLS-terminating reverse proxy in front of prm.
func withTrustedProxy(s *Server) *Server {
	prefix := netip.MustParsePrefix("127.0.0.0/8")
	s.trustedProxies = []netip.Prefix{prefix}
	return s
}

func TestProxyAwareTrustedProxyHonorsForwardedProto(t *testing.T) {
	s, _, _ := testServer(t)
	withTrustedProxy(s)
	// Cookie auth + https Origin should be accepted when a trusted proxy
	// forwarded the https scheme; the resulting cookie must carry Secure.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/updates", strings.NewReader(`{"scope":"all"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "https://example.com")
	req.RemoteAddr = "127.0.0.1:54321"
	req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("trusted https forwarded: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if job := s.updates.Current(); job != nil {
		select {
		case <-job.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("update did not finish")
		}
	}
	// /admin?token= must set a Secure cookie under the same proxy condition.
	adminReq := httptest.NewRequest(http.MethodGet, "/admin?token=abc", nil)
	adminReq.Header.Set("X-Forwarded-Proto", "https")
	adminReq.RemoteAddr = "127.0.0.1:54321"
	adminRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusFound {
		t.Fatalf("admin redirect=%d", adminRec.Code)
	}
	cookies := adminRec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("expected Secure cookie behind trusted https proxy, got %+v", cookies)
	}
}

func TestProxyAwareUntrustedPeerIgnoresForwardedProto(t *testing.T) {
	s, _, _ := testServer(t) // no trusted proxies configured
	// An untrusted peer sending X-Forwarded-Proto: https must not make the
	// request look HTTPS: the http Origin mismatches and the write is rejected.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/updates", strings.NewReader(`{"scope":"all"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "https://example.com")
	req.RemoteAddr = "127.0.0.1:54321"
	req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("untrusted forwarded proto: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// Cookie set without a trusted proxy must NOT carry Secure.
	adminReq := httptest.NewRequest(http.MethodGet, "/admin?token=abc", nil)
	adminReq.Header.Set("X-Forwarded-Proto", "https")
	adminReq.RemoteAddr = "127.0.0.1:54321"
	adminRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(adminRec, adminReq)
	cookies := adminRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Secure {
		t.Fatalf("expected non-Secure cookie without trusted proxy, got %+v", cookies)
	}
}

func TestProxyAwareHTTPProxyDoesNotSetSecureCookie(t *testing.T) {
	s, _, _ := testServer(t)
	withTrustedProxy(s)
	// A trusted proxy that forwards http (not https) must keep the cookie
	// without Secure, since the upstream scheme is plain HTTP.
	adminReq := httptest.NewRequest(http.MethodGet, "/admin?token=abc", nil)
	adminReq.Header.Set("X-Forwarded-Proto", "http")
	adminReq.RemoteAddr = "127.0.0.1:54321"
	adminRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(adminRec, adminReq)
	cookies := adminRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Secure {
		t.Fatalf("expected non-Secure cookie for http forwarded proxy, got %+v", cookies)
	}
}

func TestSecurityHeadersPresentOnAllRoutes(t *testing.T) {
	s, _, _ := testServer(t)
	check := func(target string, auth bool) {
		t.Helper()
		var req *http.Request
		if auth {
			req = authorized(http.MethodGet, target, nil)
		} else {
			req = httptest.NewRequest(http.MethodGet, target, nil)
		}
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
			if rec.Header().Get(h) == "" {
				t.Errorf("%s: missing %s (status=%d)", target, h, rec.Code)
			}
		}
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options=%q want DENY", target, got)
		}
	}
	check("/", false)             // public page (404 in test, headers still apply)
	check("/admin", false)        // admin gate (401)
	check("/api/v1/status", true) // authenticated API (200)
}

func TestConfigReload(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Helper to build a valid business config with a variable rule set.
	buildConfig := func(rules string) string {
		return "clients:\n  - id: surge\n    name: Surge\n    template: surge\n" +
			"rules:\n" + rules
	}
	oneRule := "  - id: apple\n    name: Apple\n    sources:\n      - content: DOMAIN,apple.example\n    outputs: [surge]\n"
	twoRules := oneRule + "  - id: banana\n    name: Banana\n    sources:\n      - content: DOMAIN,banana.example\n    outputs: [surge]\n"

	if err := os.WriteFile(cfgPath, []byte(buildConfig(oneRule)), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.EnsureArtifactDirs(dataDir, []string{"surge"}); err != nil {
		t.Fatal(err)
	}
	eng := &engine.UpdateEngine{
		DataDir: dataDir, Config: cfg, Registry: render.NewRegistry(),
		Fetcher: engine.NewFetcher(), Preprocessor: engine.NewPreprocessRunner(),
		State: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	manager, err := updates.NewManager(cfg, dataDir, st, eng, eng.Logger)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(cfg, st, eng, manager, Options{DataDir: dataDir, APIToken: "abc", ConfigFile: cfgPath})
	handler := s.Handler()

	api := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authorized(method, path, nil))
		return rec
	}
	var dirty struct {
		Changed bool `json:"changed"`
	}

	// Initially not dirty.
	rec := api("GET", "/api/v1/config/dirty")
	if rec.Code != 200 {
		t.Fatalf("dirty: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dirty); err != nil {
		t.Fatal(err)
	}
	if dirty.Changed {
		t.Error("expected not dirty initially")
	}

	// Edit config: add a second rule.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(cfgPath, []byte(buildConfig(twoRules)), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now dirty.
	rec = api("GET", "/api/v1/config/dirty")
	if err := json.Unmarshal(rec.Body.Bytes(), &dirty); err != nil {
		t.Fatal(err)
	}
	if !dirty.Changed {
		t.Error("expected dirty after edit")
	}

	// Reload.
	rec = api("POST", "/api/v1/config/reload")
	if rec.Code != 200 {
		t.Fatalf("reload: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Not dirty after reload.
	rec = api("GET", "/api/v1/config/dirty")
	if err := json.Unmarshal(rec.Body.Bytes(), &dirty); err != nil {
		t.Fatal(err)
	}
	if dirty.Changed {
		t.Error("expected not dirty after reload")
	}

	// Config updated to 2 rules.
	if n := len(s.config().Rules); n != 2 {
		t.Errorf("expected 2 rules after reload, got %d", n)
	}
	if s.DataDir != dataDir || s.Engine.DataDir != dataDir {
		t.Fatalf("runtime data directory changed: server=%q engine=%q", s.DataDir, s.Engine.DataDir)
	}

	// Removed runtime fields are rejected by strict config decoding.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(cfgPath, []byte("serve:\n  host: 127.0.0.2\n"+buildConfig(twoRules)), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = api("POST", "/api/v1/config/reload")
	if rec.Code != 422 {
		t.Fatalf("expected 422 for removed runtime field, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "config_invalid") || !strings.Contains(rec.Body.String(), "field serve not found") {
		t.Fatalf("unexpected removed-field response: %s", rec.Body.String())
	}
}
