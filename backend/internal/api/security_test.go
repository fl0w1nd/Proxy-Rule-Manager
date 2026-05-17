package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

// TestStatus_UnauthenticatedRedactsFailureError ensures that the public
// /api/status view never surfaces the free-form last failure message, which
// can contain source URLs with embedded tokens.
func TestStatus_UnauthenticatedRedactsFailureError(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()

	// Persist a rule so /api/status has something to summarise.
	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{{
		Name:   "secret-rule",
		Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
		Tags:   []string{},
	}}
	if _, err := srv.Store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Seed a failed artifact attempt whose error message looks exactly like
	// the leak the redaction is meant to plug.
	const sensitiveErr = "Source https://example.com/private?token=SHHH-DO-NOT-LEAK: 403 Forbidden"
	if err := srv.Store.RecordArtifactAttempts(ctx, []store.ArtifactAttempt{{
		RuleName:    "secret-rule",
		Client:      "clash_meta",
		AttemptedAt: "2024-01-01T00:00:00Z",
		Status:      "failed",
		Error:       sensitiveErr,
	}}); err != nil {
		t.Fatalf("record attempt: %v", err)
	}

	publicCode, publicBody := getJSON(t, ts.URL, "/api/status", "")
	if publicCode != http.StatusOK {
		t.Fatalf("public status: %d", publicCode)
	}
	rules, _ := publicBody["rules"].([]any)
	if len(rules) == 0 {
		t.Fatalf("public status returned no rules: %v", publicBody)
	}
	first, _ := rules[0].(map[string]any)
	if got, _ := first["lastFailureError"].(string); got != "" {
		t.Fatalf("public status must omit lastFailureError, got %q", got)
	}
	if got, _ := first["hasError"].(bool); !got {
		t.Errorf("hasError must remain true so UI can surface the failure state")
	}
	if raw, err := json.Marshal(publicBody); err == nil {
		if bytes.Contains(raw, []byte("SHHH-DO-NOT-LEAK")) {
			t.Fatalf("sensitive token leaked into public status body: %s", raw)
		}
	}

	adminCode, adminBody := getJSON(t, ts.URL, "/api/status", "secret")
	if adminCode != http.StatusOK {
		t.Fatalf("admin status: %d", adminCode)
	}
	adminRules, _ := adminBody["rules"].([]any)
	if len(adminRules) == 0 {
		t.Fatalf("admin status returned no rules")
	}
	first, _ = adminRules[0].(map[string]any)
	if got, _ := first["lastFailureError"].(string); got != sensitiveErr {
		t.Errorf("admin status must keep full failure error, got %q", got)
	}
}

// TestServeIconSet_SVGHasSandboxCSP verifies that SVG icons are served with
// the headers that prevent inline scripts from running in the page origin.
func TestServeIconSet_SVGHasSandboxCSP(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	if err := os.MkdirAll(srv.Config.IconSetDir, 0o755); err != nil {
		t.Fatalf("mkdir iconset: %v", err)
	}
	// A benign SVG body — the response headers are what we care about.
	svgPath := filepath.Join(srv.Config.IconSetDir, "harmless.svg")
	if err := os.WriteFile(svgPath, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatalf("write svg: %v", err)
	}

	resp, err := http.Get(ts.URL + "/IconSet/harmless.svg")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type: got %q want image/svg+xml", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q want nosniff", got)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" || !strings.Contains(csp, "sandbox") {
		t.Errorf("Content-Security-Policy must include sandbox, got %q", csp)
	}
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("Content-Security-Policy must include default-src 'none', got %q", csp)
	}
	if disp := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(disp, "inline;") {
		t.Errorf("Content-Disposition: got %q want inline; ...", disp)
	}
}

// TestServeIconSet_PNGNoSandboxCSP ensures non-SVG icons get the global
// nosniff header but not the SVG-specific sandbox CSP — that would break
// caching/rendering elsewhere for no benefit.
func TestServeIconSet_PNGNoSandboxCSP(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	if err := os.MkdirAll(srv.Config.IconSetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pngPath := filepath.Join(srv.Config.IconSetDir, "logo.png")
	if err := os.WriteFile(pngPath, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}

	resp, err := http.Get(ts.URL + "/IconSet/logo.png")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("nosniff header missing on PNG: got %q", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != "" {
		t.Errorf("PNG should not carry CSP, got %q", got)
	}
}

// TestGlobalNosniffHeader verifies the global middleware applies the header
// to a routine JSON endpoint as well.
func TestGlobalNosniffHeader(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	resp, err := http.Get(ts.URL + "/api/auth/required")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options on /api/auth/required: got %q want nosniff", got)
	}
}

// TestDecodeJSON_RejectsOversizeBody confirms that any single JSON request
// is capped at MaxJSONBodyBytes — defeating a slow/large body DoS.
func TestDecodeJSON_RejectsOversizeBody(t *testing.T) {
	s := &Server{}
	// Build a body that just barely exceeds the cap.
	huge := bytes.Repeat([]byte("a"), MaxJSONBodyBytes+10)
	body := append([]byte(`{"token":"`), huge...)
	body = append(body, []byte(`"}`)...)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Token string `json:"token"`
	}
	if err := s.DecodeJSON(req, &dst); err == nil {
		t.Fatalf("DecodeJSON accepted oversize body (%d bytes), want error", len(body))
	}
}

// TestDecodeJSON_AllowsRegularBody is a non-regression for the existing
// happy path now that the body is wrapped in MaxBytesReader.
func TestDecodeJSON_AllowsRegularBody(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"token":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	var dst struct {
		Token string `json:"token"`
	}
	if err := s.DecodeJSON(req, &dst); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if dst.Token != "ok" {
		t.Fatalf("token: got %q", dst.Token)
	}
}

// TestIconUpload_RejectsSVGWithScript exercises the upload-time content
// scan. The CSP sandbox is the authoritative defense; this is the cheap
// outer filter that keeps obvious payloads from ever landing on disk.
func TestIconUpload_RejectsSVGWithScript(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	if err := os.MkdirAll(srv.Config.IconSetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	body, contentType := buildUploadBody(t, "evil.svg",
		`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/iconset/upload", body)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	uploaded, _ := out["uploaded"].([]any)
	errs, _ := out["errors"].([]any)
	if len(uploaded) != 0 {
		t.Fatalf("malicious SVG must not appear in uploaded list, got %v", uploaded)
	}
	if len(errs) != 1 {
		t.Fatalf("expected exactly one upload error, got %v", errs)
	}
	if _, err := os.Stat(filepath.Join(srv.Config.IconSetDir, "evil.svg")); !os.IsNotExist(err) {
		t.Fatalf("malicious SVG must not be written to disk, stat err=%v", err)
	}
}

func TestIconUpload_AcceptsHarmlessSVG(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	if err := os.MkdirAll(srv.Config.IconSetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	body, contentType := buildUploadBody(t, "ok.svg",
		`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/iconset/upload", body)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	uploaded, _ := out["uploaded"].([]any)
	if len(uploaded) != 1 {
		t.Fatalf("expected one uploaded SVG, got %v (full=%v)", uploaded, out)
	}
	if _, err := os.Stat(filepath.Join(srv.Config.IconSetDir, "ok.svg")); err != nil {
		t.Fatalf("clean SVG missing from disk: %v", err)
	}
}

func TestIconUpload_RejectsSVGWithOnLoad(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	body, contentType := buildUploadBody(t, "trick.svg",
		`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/iconset/upload", body)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	uploaded, _ := out["uploaded"].([]any)
	errs, _ := out["errors"].([]any)
	if len(uploaded) != 0 {
		t.Fatalf("SVG with onload must be rejected, got %v", uploaded)
	}
	if len(errs) != 1 {
		t.Fatalf("expected one upload error, got %v", errs)
	}
}

// buildUploadBody assembles a multipart body with a single file named
// "files" (matches what handleUploadIcons expects).
func buildUploadBody(t *testing.T, filename, content string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="files"; filename="`+filename+`"`)
	hdr.Set("Content-Type", "image/svg+xml")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, mw.FormDataContentType()
}
