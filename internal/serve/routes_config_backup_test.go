package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

func TestConfigBackupCreatedOnSaveAndRestoredWithVersion(t *testing.T) {
	s, path := fileBackedConfigServer(t, nil)
	h := s.Handler()
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body.String())
	}

	rec = patchConfig(h, `{"version":1,"ops":[{"op":"update_rule","id":"base","value":{"id":"base","name":"Base Updated","sources":[{"url":"https://rules.example/rules.list"}],"outputs":["surge"]}}]}`)
	if rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups", nil))
	var listed configBackupListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || listed.Version != 2 || len(listed.Items) != 1 {
		t.Fatalf("list after save: %d %s", rec.Code, rec.Body.String())
	}
	backup := listed.Items[0]
	if backup.ID == "" || backup.Size == 0 || backup.CreatedAt == "" {
		t.Fatalf("backup=%+v", backup)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups/"+url.PathEscape(backup.ID), nil))
	var detail config.BackupDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || detail.YAML != string(original) {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(filepath.Join(s.DataDir, ".state", "backups", backup.ID))
	if err != nil || string(stored) != string(original) {
		t.Fatalf("stored backup: %v %s", err, stored)
	}

	rec = restoreBackup(h, backup.ID, 1)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "config_version_conflict") {
		t.Fatalf("stale restore: %d %s", rec.Code, rec.Body.String())
	}
	rec = restoreBackup(h, backup.ID, 2)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":3`) {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(original) || s.config().Rules[0].Name != "Base" {
		t.Fatal("restore did not reinstall the snapshot")
	}
	index, err := os.ReadFile(filepath.Join(s.DataDir, "static", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "Base") || strings.Contains(string(index), "Base Updated") {
		t.Fatalf("public index was not rebuilt:\n%s", index)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Version != 3 || len(listed.Items) != 2 {
		t.Fatalf("list after restore: %s", rec.Body.String())
	}

	rec = patchConfig(h, `{"version":3,"ops":[{"op":"add_output","rule_id":"base","output_id":"surge"}]}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":3`) {
		t.Fatalf("noop: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Version != 3 || len(listed.Items) != 2 {
		t.Fatalf("noop created a backup: %s", rec.Body.String())
	}
}

func TestConfigBackupRestoreGuards(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	rec := restoreBackup(h, "config-20260908-121800.yaml", 1)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "backup_not_found") {
		t.Fatalf("missing: %d %s", rec.Code, rec.Body.String())
	}
	rec = restoreBackup(h, "../config.yaml", 1)
	if rec.Code != 404 {
		t.Fatalf("escape: %d %s", rec.Code, rec.Body.String())
	}
	rec = restoreBackup(h, "config-20260908-121800.yaml", 0)
	if rec.Code != 422 {
		t.Fatalf("version: %d %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/backups", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("unauthenticated list: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/backups/config-20260908-121800.yaml/restore", strings.NewReader(`{"version":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example")
	req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("cross origin: %d %s", rec.Code, rec.Body.String())
	}
}

func TestConfigBackupRestoreDuringUpdate(t *testing.T) {
	runner := &blockingConfigRunner{started: make(chan struct{}), release: make(chan struct{})}
	s, _ := fileBackedConfigServer(t, runner)
	h := s.Handler()
	rec := patchConfig(h, `{"version":1,"ops":[{"op":"update_rule","id":"base","value":{"id":"base","name":"Base Updated","sources":[{"url":"https://rules.example/rules.list"}],"outputs":["surge"]}}]}`)
	if rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	listed := listBackups(t, h)
	job, err := s.updates.Start(updates.Request{Scope: "all"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	defer func() { close(runner.release); <-job.Done() }()
	rec = restoreBackup(h, listed.Items[0].ID, 2)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "update_in_progress") {
		t.Fatalf("restore during update: %d %s", rec.Code, rec.Body.String())
	}
}

func restoreBackup(handler http.Handler, id string, version int64) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"version": version})
	req := authorized(http.MethodPost, "/api/v1/config/backups/"+url.PathEscape(id)+"/restore", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func listBackups(t *testing.T, h http.Handler) configBackupListResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/backups", nil))
	var listed configBackupListResponse
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	return listed
}
