package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFilesCRUDAndGuards(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files", nil))
	if rec.Code != 200 || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body.String())
	}

	rec = localFileJSON(h, http.MethodPost, "/api/v1/local-files", `{"name":"my-direct.list","content":"example.com\nexample.org\n"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created localFileDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "my-direct.list" || created.Lines != 2 || created.Content != "example.com\nexample.org\n" || created.Size == 0 {
		t.Fatalf("created=%+v", created)
	}
	written, err := os.ReadFile(filepath.Join(s.DataDir, "local", "my-direct.list"))
	if err != nil || string(written) != "example.com\nexample.org\n" {
		t.Fatalf("disk: %v %q", err, written)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files", nil))
	var listed struct{ Items []localFileItem }
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(listed.Items) != 1 || listed.Items[0].Name != "my-direct.list" || listed.Items[0].Lines != 2 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files/my-direct.list", nil))
	var detail localFileDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || detail.Content != "example.com\nexample.org\n" {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}

	rec = localFileJSON(h, http.MethodPut, "/api/v1/local-files/my-direct.list", `{"content":"DOMAIN,example.com\n"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"lines":1`) {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}

	rec = localFileJSON(h, http.MethodPost, "/api/v1/local-files", `{"name":"my-direct.list","content":""}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "file_exists") {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := authorized(http.MethodDelete, "/api/v1/local-files/my-direct.list", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(s.DataDir, "local", "my-direct.list")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still present: %v", err)
	}
}

func TestLocalFilesRejectsUnsafeNames(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	for _, name := range []string{"../secret.list", "nested/file.list", `nested\file.list`, "noext", "notes.md", ".hidden.list", "..", ""} {
		body := `{"name":` + jsonString(name) + `,"content":"x\n"}`
		rec := localFileJSON(h, http.MethodPost, "/api/v1/local-files", body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_filename") {
			t.Fatalf("create %q: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	for _, name := range []string{"noext", "notes.md", ".hidden.list"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files/"+name, nil))
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "invalid_filename") {
			t.Fatalf("get %q: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	outside := filepath.Join(s.DataDir, "..", "escape.list")
	if err := os.WriteFile(outside, []byte("stolen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files/..%2Fescape.list", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("escaped get: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}

func TestLocalFilesRefuseDeleteWhenReferenced(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	if err := os.MkdirAll(filepath.Join(s.DataDir, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.DataDir, "local", "used.list"), []byte("DOMAIN,used.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := patchConfig(h, `{"version":1,"ops":[{"op":"update_rule","id":"base","value":{"id":"base","name":"Base","sources":[{"file":"used.list"}],"outputs":["surge"]}}]}`)
	if rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodDelete, "/api/v1/local-files/used.list", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "file_in_use") {
		t.Fatalf("in use: %d %s", rec.Code, rec.Body.String())
	}
	var payload apiErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	rules, _ := payload.Error.Details["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("refs=%v", payload.Error.Details["rules"])
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodDelete, "/api/v1/local-files/missing.list", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLocalFilesAuthAndOriginGuards(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/local-files", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list: %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/local-files", strings.NewReader(`{"name":"x.list","content":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example")
	req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross origin: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLocalFilesSkipsUnmanagedEntries(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	dir := filepath.Join(s.DataDir, "local")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"keep.list":        "a.example\n",
		"notes.md":         "# skip\n",
		"nested/deep.list": "hidden.example\n",
		".draft.list":      "draft\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files", nil))
	var listed struct{ Items []localFileItem }
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(listed.Items) != 1 || listed.Items[0].Name != "keep.list" || listed.Items[0].Lines != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
}

func localFileJSON(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := authorized(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
