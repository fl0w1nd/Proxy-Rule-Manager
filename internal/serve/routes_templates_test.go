package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

const customTemplateYAML = "# custom output\nid: custom\nname: Custom\ncodec: linelist\nextension: .list\nkind_map:\n  domain: DOMAIN\n  ip_cidr: IP-CIDR\n"

func TestTemplatesCreatePreviewAndVersionedEdit(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	body, _ := json.Marshal(map[string]any{"yaml": customTemplateYAML})
	preview := localFileJSON(h, http.MethodPost, "/api/v1/templates/validate", string(body))
	if preview.Code != 200 || !strings.Contains(preview.Body.String(), "DOMAIN,example.com") || !strings.Contains(preview.Body.String(), `"sample"`) {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	if _, ok := s.Engine.Registry.Get("custom"); ok {
		t.Fatal("preview registered template")
	}
	created := localFileJSON(h, http.MethodPost, "/api/v1/templates", string(body))
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var detail templateDetail
	if err := json.Unmarshal(created.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Builtin || detail.Version == "" {
		t.Fatalf("detail: %+v", detail)
	}
	if _, ok := s.Engine.Registry.Get("custom"); !ok {
		t.Fatal("template not available immediately")
	}
	// The new template can be selected by a client in the same process.
	result := patchConfig(h, `{"version":1,"ops":[{"op":"add_client","value":{"id":"custom","name":"Custom","template":"custom","variants":[{"id":"custom-non-ip","ops":[{"type":"exclude_kinds","kinds":["ip_cidr"]}]}]}}]}`)
	if result.Code != 200 {
		t.Fatalf("client: %d %s", result.Code, result.Body.String())
	}
	get := httptest.NewRecorder()
	h.ServeHTTP(get, authorized(http.MethodGet, "/api/v1/templates/custom", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), "# custom output") {
		t.Fatalf("read: %d %s", get.Code, get.Body.String())
	}
	duplicate := localFileJSON(h, http.MethodPost, "/api/v1/templates", string(body))
	if duplicate.Code != 409 {
		t.Fatalf("duplicate: %d", duplicate.Code)
	}
	changed := strings.Replace(customTemplateYAML, "DOMAIN", "HOST", 1)
	body, _ = json.Marshal(map[string]any{"yaml": changed, "version": detail.Version})
	updated := localFileJSON(h, http.MethodPut, "/api/v1/templates/custom", string(body))
	if updated.Code != 200 {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	stale := localFileJSON(h, http.MethodPut, "/api/v1/templates/custom", string(body))
	if stale.Code != 409 {
		t.Fatalf("stale: %d", stale.Code)
	}
	raw, err := os.ReadFile(filepath.Join(s.DataDir, "templates", "custom.yaml"))
	if err != nil || string(raw) != changed {
		t.Fatalf("disk: %s %v", raw, err)
	}
	registry := render.NewRegistry()
	if err := registry.LoadDir(filepath.Join(s.DataDir, "templates")); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get("custom"); !ok {
		t.Fatal("custom template missing after reload")
	}
}

func TestTemplateValidationAndBuiltinProtection(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	for _, content := range []string{
		"id: [broken", customTemplateYAML + "unknown: true\n", customTemplateYAML + "---\nid: extra\n",
		strings.Replace(customTemplateYAML, "custom\n", "../escape\n", 1),
		strings.Replace(customTemplateYAML, "DOMAIN", "[]", 1),
	} {
		body, _ := json.Marshal(map[string]any{"yaml": content})
		rec := localFileJSON(h, http.MethodPost, "/api/v1/templates", string(body))
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"errors"`) {
			t.Fatalf("invalid %q: %d %s", content, rec.Code, rec.Body.String())
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		url := "/api/v1/templates"
		if method == http.MethodPut {
			url += "/surge"
		}
		body, _ := json.Marshal(map[string]any{"yaml": strings.Replace(customTemplateYAML, "id: custom", "id: surge", 1)})
		rec := localFileJSON(h, method, url, string(body))
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), "builtin_template") {
			t.Fatalf("builtin: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/templates", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"builtin":true`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTemplatesAuthenticationAndOrigin(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	for _, path := range []string{"/api/v1/templates", "/api/v1/templates/surge"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 401 {
			t.Fatalf("auth %s: %d", path, rec.Code)
		}
	}
	for _, path := range []string{"/api/v1/templates", "/api/v1/templates/validate"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"yaml":"id: custom"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://other.example")
		req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
		h.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatalf("origin: %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestClientDeletionRejectsGeositeReference(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	rec := patchConfig(h, `{"version":1,"ops":[{"op":"add_client","value":{"id":"geo","name":"Geo","template":"surge"}},{"op":"update_geosite","value":{"providers":[{"name":"v2fly","clients":["geo"]}]}}]}`)
	if rec.Code != 200 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	rec = patchConfig(h, `{"version":2,"ops":[{"op":"remove_client","id":"geo"}]}`)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "geosite.providers[0].clients[0]") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	cfg, version := s.ConfigManager.Snapshot()
	if version != 2 || len(cfg.Clients) != 2 {
		t.Fatal("rejected delete changed configuration")
	}
}

func TestTemplateSaveDuringUpdate(t *testing.T) {
	runner := &blockingConfigRunner{started: make(chan struct{}), release: make(chan struct{})}
	s, _ := fileBackedConfigServer(t, runner)
	job, err := s.updates.Start(updates.Request{Scope: "all"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	defer func() { close(runner.release); <-job.Done() }()
	body, _ := json.Marshal(map[string]any{"yaml": customTemplateYAML})
	rec := localFileJSON(s.Handler(), http.MethodPost, "/api/v1/templates", string(body))
	if rec.Code != 409 {
		t.Fatalf("save during update: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := s.Engine.Registry.Get("custom"); ok {
		t.Fatal("rejected save changed registry")
	}
	if _, err := os.Stat(filepath.Join(s.DataDir, "templates", "custom.yaml")); !os.IsNotExist(err) {
		t.Fatal("rejected save wrote template")
	}
}

func TestConcurrentTemplateEdits(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	body, _ := json.Marshal(map[string]any{"yaml": customTemplateYAML})
	rec := localFileJSON(h, http.MethodPost, "/api/v1/templates", string(body))
	var created templateDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	results := make(chan int, 2)
	for _, prefix := range []string{"# first\n", "# second\n"} {
		go func(prefix string) {
			payload, _ := json.Marshal(map[string]any{"yaml": prefix + customTemplateYAML, "version": created.Version})
			results <- localFileJSON(h, http.MethodPut, "/api/v1/templates/custom", string(payload)).Code
		}(prefix)
	}
	first, second := <-results, <-results
	if (first != 200 || second != 409) && (first != 409 || second != 200) {
		t.Fatalf("concurrent statuses: %d %d", first, second)
	}
}
