package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

func newTestServer(t *testing.T, adminToken string) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		DataDir:       dir,
		DBPath:        filepath.Join(dir, "db.sqlite"),
		RulesDir:      filepath.Join(dir, "Rules"),
		SourcesDir:    filepath.Join(dir, "sources"),
		GeositeDir:    filepath.Join(dir, "geosite"),
		IconSetDir:    filepath.Join(dir, "iconset"),
		ClientFileDir: filepath.Join(dir, "client"),
		WAFDir:        filepath.Join(dir, "waf"),
		Port:          0,
		AdminToken:    adminToken,
		OutDir:        filepath.Join(dir, "out"),
	}
	paths := store.Paths{
		DataDir:       cfg.DataDir,
		RulesDir:      cfg.RulesDir,
		SourcesDir:    cfg.SourcesDir,
		GeositeDir:    cfg.GeositeDir,
		IconSetDir:    cfg.IconSetDir,
		ClientFileDir: cfg.ClientFileDir,
		WAFDir:        cfg.WAFDir,
	}
	st, err := store.Open(cfg.DBPath, paths)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := New(cfg, st)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestServeGeositeRuleFileWithEncodedSpecialChars(t *testing.T) {
	srv, ts := newTestServer(t, "token")
	cases := []string{
		"alibaba@!cn.list",
		"category-ai+ads.list",
		"foo#bar.list",
		"foo&bar=baz.list",
		"foo(bar),baz.list",
		"foo;bar.list",
	}

	for _, name := range cases {
		path := filepath.Join(srv.Config.RulesDir, "Clash Meta", "geosite", "v2fly", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		resp, err := http.Get(ts.URL + "/Rules/Clash%20Meta/geosite/v2fly/" + url.PathEscape(name))
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status: got %d body=%q", name, resp.StatusCode, string(body))
		}
		if string(body) != name {
			t.Fatalf("%s body: got %q", name, string(body))
		}
	}
}

func TestServeRuleFileRejectsEscapedPathSeparator(t *testing.T) {
	_, ts := newTestServer(t, "token")

	resp, err := http.Get(ts.URL + "/Rules/Clash%20Meta/geosite/v2fly/foo%2Fbar.list")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func getJSON(t *testing.T, base, path, token string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func postJSON(t *testing.T, base, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequest(http.MethodPost, base+path, &buf)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func putJSON(t *testing.T, base, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequest(http.MethodPut, base+path, &buf)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestServer_AuthRequiredEcho(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/auth/required", "")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if v, _ := body["required"].(bool); !v {
		t.Fatalf("expected required=true when ADMIN_TOKEN is set, got %v", body)
	}
}

func TestServer_AuthRequiredOpenWhenNoToken(t *testing.T) {
	_, ts := newTestServer(t, "")
	code, body := getJSON(t, ts.URL, "/api/auth/required", "")
	if code != http.StatusOK || body["required"].(bool) {
		t.Fatalf("expected required=false, got %v / %d", body, code)
	}
}

func TestServer_AdminGuardRejectsMissingToken(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/config", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%v)", code, body)
	}
}

func TestServer_AdminGuardAcceptsValidToken(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/config", "secret")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", code, body)
	}
	if _, ok := body["config"]; !ok {
		t.Fatalf("expected config payload, got keys: %v", keys(body))
	}
}

func TestServer_PublicStatusShape(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/status", "")
	if code != http.StatusOK {
		t.Fatalf("status code: %d (%v)", code, body)
	}
	for _, key := range []string{"rulesCount", "geositeRulesCount", "rules", "geositeRules", "clients"} {
		if _, ok := body[key]; !ok {
			t.Errorf("public status missing %q (have %v)", key, keys(body))
		}
	}
	if _, ok := body["todayStats"]; ok {
		t.Errorf("public status must not expose todayStats")
	}
}

func TestServer_AdminStatusIncludesTodayStats(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/status", "secret")
	if code != http.StatusOK {
		t.Fatalf("status code: %d", code)
	}
	if _, ok := body["todayStats"]; !ok {
		t.Errorf("admin status must include todayStats, have keys %v", keys(body))
	}
}

func TestServer_LoginAndVerifyHappyPath(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := postJSON(t, ts.URL, "/api/auth/login", "", map[string]any{"token": "secret"})
	if code != http.StatusOK {
		t.Fatalf("login: %d %v", code, body)
	}
	if v, _ := body["success"].(bool); !v {
		t.Fatalf("expected success=true, got %v", body)
	}
	code, _ = postJSON(t, ts.URL, "/api/auth/verify", "secret", nil)
	if code != http.StatusOK {
		t.Fatalf("verify: %d", code)
	}
}

func TestServer_RenameRulePreservesArtifactContent(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()
	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{{
		Name: "old-rule",
		Output: schema.OutputConfig{
			Clients: []string{"clash_meta"},
		},
		Tags: []string{},
	}}
	if _, err := srv.Store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	oldPath := filepath.Join(srv.Config.RulesDir, "clash_meta", "old-rule.list")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const content = "DOMAIN-SUFFIX,example.com,Proxy\n"
	if err := os.WriteFile(oldPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	code, body := putJSON(t, ts.URL, "/api/rules/old-rule", "secret", map[string]any{"newName": "new-rule"})
	if code != http.StatusOK {
		t.Fatalf("rename status: %d body=%v", code, body)
	}
	newPath := filepath.Join(srv.Config.RulesDir, "clash_meta", "new-rule.list")
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read renamed artifact: %v", err)
	}
	if string(data) != content {
		t.Fatalf("renamed content changed: %q", data)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old artifact removed, stat err=%v", err)
	}
}

func TestServer_LoginBadTokenIncreasesFailures(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	code, _ := postJSON(t, ts.URL, "/api/auth/login", "", map[string]any{"token": "wrong"})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
	if got := len(srv.RateLimiter.Snapshot()); got == 0 {
		t.Fatalf("expected at least one tracked failure, snapshot empty")
	}
}

func TestServer_SPAFallback(t *testing.T) {
	_, ts := newTestServer(t, "")
	// out/index.html doesn't exist in temp dir, so SPA fallback should 404.
	code, _ := getJSON(t, ts.URL, "/some/spa/route", "")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404 fallback when index.html missing, got %d", code)
	}
}

func TestServer_RuleFilePathSafety(t *testing.T) {
	_, ts := newTestServer(t, "")
	for _, p := range []string{"/Rules/../etc/passwd", "/Rules/clash_meta/..%2fpasswd"} {
		code, _ := getJSON(t, ts.URL, p, "")
		if code != http.StatusBadRequest && code != http.StatusNotFound {
			t.Errorf("expected 400/404 for unsafe path %s, got %d", p, code)
		}
	}
}

func TestServer_RateLimiterClearsOnSuccess(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	_, _ = postJSON(t, ts.URL, "/api/auth/login", "", map[string]any{"token": "wrong"})
	_, _ = postJSON(t, ts.URL, "/api/auth/login", "", map[string]any{"token": "secret"})
	for ip := range srv.RateLimiter.Snapshot() {
		if strings.HasPrefix(ip, "127.0.0.1") {
			t.Fatalf("expected failure record cleared after successful login: %v", ip)
		}
	}
}

// TestServer_ErrorEnvelopeShape verifies that Error() puts the human-readable
// message in "error" (not http.StatusText) and adds a typed "code" field.
// The frontend reads error.error as the display message; see api-client.ts:42-48.
func TestServer_ErrorEnvelopeShape(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, body := getJSON(t, ts.URL, "/api/config", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
	if msg, _ := body["error"].(string); msg != "Unauthorized" {
		t.Errorf("error field: want %q, got %q", "Unauthorized", msg)
	}
	if c, _ := body["code"].(string); c != "UNAUTHORIZED" {
		t.Errorf("code field: want %q, got %q", "UNAUTHORIZED", c)
	}
	// Must not duplicate the message in a separate "message" key.
	if _, ok := body["message"]; ok {
		t.Errorf("unexpected 'message' field in 401 error envelope: %v", body)
	}
}

// TestServer_DecodeJSONAllowsUnknownFields checks that unknown JSON fields are
// silently dropped, matching TS Zod .strip() behaviour (not rejected).
func TestServer_DecodeJSONAllowsUnknownFields(t *testing.T) {
	s := &Server{}
	body := strings.NewReader(`{"token":"tok","extra":"field","nested":{"x":1}}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	var dst struct {
		Token string `json:"token"`
	}
	if err := s.DecodeJSON(req, &dst); err != nil {
		t.Fatalf("DecodeJSON with unknown fields must not error: %v", err)
	}
	if dst.Token != "tok" {
		t.Errorf("expected token=tok, got %q", dst.Token)
	}
}

// TestServer_JSONListNilSlice verifies that a typed nil slice is marshalled
// as [] rather than null.
func TestServer_JSONListNilSlice(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	var sl []string // typed nil — json.Marshal would encode as null
	s.JSONList(w, "items", sl)
	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := out["items"]
	if !ok {
		t.Fatal("missing 'items' key in response")
	}
	arr, ok := items.([]any)
	if !ok {
		t.Fatalf("expected JSON array, got %T (%v)", items, items)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %v", arr)
	}
}

// TestServer_JSONListNonNilSlice verifies that a non-nil slice is passed through.
func TestServer_JSONListNonNilSlice(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	s.JSONList(w, "items", []string{"a", "b"})
	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	arr, _ := out["items"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %v", arr)
	}
}

// TestServer_PermanentBanRetryAfterFallback ensures that a permanently banned
// IP (retryAfter=0) still gets Retry-After: 60 matching TS (retryAfter || 60).
// ClientIP uses X-Forwarded-For when present, so we inject a known IP.
func TestServer_PermanentBanRetryAfterFallback(t *testing.T) {
	const testIP = "10.0.0.99"
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()
	if err := srv.Store.UpsertBan(ctx, schema.BanRecord{
		IP:        testIP,
		Reason:    "test_permanent",
		BannedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		ExpiresAt: nil, // permanent — IsBlocked returns retryAfter=0
		FailCount: 10,
	}); err != nil {
		t.Fatalf("upsert ban: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-Forwarded-For", testIP) // ClientIP reads this first
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	ra := resp.Header.Get("Retry-After")
	if ra == "" || ra == "0" {
		t.Errorf("permanent ban must yield non-zero Retry-After, got %q", ra)
	}
}

// TestServer_PanicRecovery verifies that a panicking handler returns 500 with
// the structured error envelope instead of crashing.
func TestServer_PanicRecovery(t *testing.T) {
	// Register a panic-inducing route directly on a plain mux.
	mux := http.NewServeMux()
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	ps := httptest.NewServer(panicRecoverer(mux))
	defer ps.Close()

	resp, err := http.Get(ps.URL + "/panic")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if c, _ := body["code"].(string); c != "INTERNAL_ERROR" {
		t.Errorf("code field: want INTERNAL_ERROR, got %q", c)
	}
	if e, _ := body["error"].(string); e == "" {
		t.Errorf("error field must be non-empty")
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
