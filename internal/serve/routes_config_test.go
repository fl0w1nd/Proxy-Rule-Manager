package serve

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	"github.com/fl0w1nd/proxy-rule-manager/templates"
)

type blockingConfigRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingConfigRunner) FullUpdate(context.Context) engine.UpdateResult {
	close(r.started)
	<-r.release
	return engine.UpdateResult{StartTime: time.Now(), EndTime: time.Now()}
}

func (r *blockingConfigRunner) PartialUpdate(ctx context.Context, _ []string) engine.UpdateResult {
	return r.FullUpdate(ctx)
}

func TestConfigPatchAPIUpdatesSourceAndRuntime(t *testing.T) {
	t.Setenv("RULE_HOST", "private.example")
	s, path := fileBackedConfigServer(t, nil)
	handler := s.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get config=%d %s", rec.Code, rec.Body.String())
	}
	var snapshot struct {
		Version int64          `json:"version"`
		Config  map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != 1 || !strings.Contains(rec.Body.String(), `${RULE_HOST}`) {
		t.Fatalf("snapshot=%s", rec.Body.String())
	}

	body := `{"version":1,"ops":[{"op":"update_rule","id":"base","value":{"id":"base","name":"Base Updated","sources":[{"url":"https://${RULE_HOST}/rules.list"}],"outputs":["surge"]}}]}`
	rec = patchConfig(handler, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch=%d %s", rec.Code, rec.Body.String())
	}
	var result configMutationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Version != 2 {
		t.Fatalf("result=%+v", result)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `${RULE_HOST}`) || !strings.Contains(string(written), "name: Base Updated") {
		t.Fatalf("written config:\n%s", written)
	}
	if got := s.config().Rules[0].Name; got != "Base Updated" {
		t.Fatalf("runtime rule name=%q", got)
	}
	index, err := os.ReadFile(filepath.Join(s.DataDir, "static", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "Base Updated") {
		t.Fatalf("public index was not refreshed:\n%s", index)
	}

	rec = patchConfig(handler, body)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "config_version_conflict") || !strings.Contains(rec.Body.String(), `"current_version":2`) {
		t.Fatalf("stale patch=%d %s", rec.Code, rec.Body.String())
	}
}

func TestConfigPatchAPIRejectsInvalidAndExternalChanges(t *testing.T) {
	s, path := fileBackedConfigServer(t, nil)
	handler := s.Handler()

	rec := patchConfig(handler, `{"version":1,"ops":[{"op":"remove_client","id":"surge"}]}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "config_invalid") {
		t.Fatalf("invalid patch=%d %s", rec.Code, rec.Body.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, []byte("# external\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = patchConfig(handler, `{"version":1,"ops":[{"op":"add_output","rule_id":"base","output_id":"surge"}]}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "config_dirty") {
		t.Fatalf("dirty patch=%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/dirty", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"changed":true`) {
		t.Fatalf("dirty status=%d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, authorized(http.MethodPost, "/api/v1/config/reload", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":2`) {
		t.Fatalf("reload=%d %s", rec.Code, rec.Body.String())
	}
}

func TestConfigPatchAPIRejectsPatchDuringUpdate(t *testing.T) {
	runner := &blockingConfigRunner{started: make(chan struct{}), release: make(chan struct{})}
	s, _ := fileBackedConfigServer(t, runner)
	job, err := s.updates.Start(updates.Request{Scope: "all"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started

	rec := patchConfig(s.Handler(), `{"version":1,"ops":[{"op":"add_output","rule_id":"base","output_id":"surge"}]}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "update_in_progress") {
		t.Fatalf("patch during update=%d %s", rec.Code, rec.Body.String())
	}
	close(runner.release)
	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("update did not finish")
	}
}

func TestConfigPatchAPIValidatesRequestEnvelope(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	handler := s.Handler()

	req := authorized(http.MethodPost, "/api/v1/config/patch", strings.NewReader(`{"version":1,"ops":[]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("media type=%d %s", rec.Code, rec.Body.String())
	}

	rec = patchConfig(handler, `{"version":1,"ops":[{"op":"remove_rule","id":"base","value":{}}]}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_patch") {
		t.Fatalf("unexpected field=%d %s", rec.Code, rec.Body.String())
	}

	rec = patchConfig(handler, `{"version":1,"ops":[{"op":"add_rule","id":"","value":{"id":"extra"}}]}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_patch") {
		t.Fatalf("empty unexpected field=%d %s", rec.Code, rec.Body.String())
	}

	rec = patchConfig(handler, `{"version":1,"ops":[]} {"version":1,"ops":[]}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("multiple objects=%d %s", rec.Code, rec.Body.String())
	}

	large := `{"version":1,"ops":[{"op":"add_rule","value":{"id":"large","name":"` + strings.Repeat("x", 1<<20) + `","sources":[],"outputs":[]}}]}`
	rec = patchConfig(handler, large)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "request_too_large") {
		t.Fatalf("large request=%d %s", rec.Code, rec.Body.String())
	}
}

func TestConfigPatchAPIRequiresAuthenticationAndSameOrigin(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	handler := s.Handler()
	body := `{"version":1,"ops":[{"op":"add_output","rule_id":"base","output_id":"surge"}]}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example")
	req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "invalid_origin") {
		t.Fatalf("cross origin=%d %s", rec.Code, rec.Body.String())
	}
}

func patchConfig(handler http.Handler, body string) *httptest.ResponseRecorder {
	req := authorized(http.MethodPost, "/api/v1/config/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func fileBackedConfigServer(t *testing.T, runner updatesRunner) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	source := `# source comment
clients:
  - id: surge
    name: Surge
    template: surge
rules:
  - id: base
    name: Base
    sources:
      - url: https://${RULE_HOST}/rules.list
    outputs: [surge]
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := config.NewManager(path, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := manager.Snapshot()
	st, err := state.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	registry := render.NewRegistry()
	if err := registry.LoadEmbedded(templates.FS); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	eng := &engine.UpdateEngine{
		DataDir: dataDir, Registry: registry, Fetcher: engine.NewFetcher(),
		Preprocessor: engine.NewPreprocessRunner(), State: st, Logger: logger,
	}
	eng.SetConfig(cfg)
	var updateRunner updatesRunner = eng
	if runner != nil {
		updateRunner = runner
	}
	updateManager, err := updates.NewManager(cfg, dataDir, st, updateRunner, logger)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(cfg, st, eng, updateManager, Options{
		DataDir: dataDir, APIToken: "abc", ConfigFile: path, ConfigManager: manager,
	})
	if err := eng.RebuildSite(); err != nil {
		t.Fatal(err)
	}
	return s, path
}

type updatesRunner interface {
	FullUpdate(context.Context) engine.UpdateResult
	PartialUpdate(context.Context, []string) engine.UpdateResult
}
