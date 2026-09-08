package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

func rawRequest(t *testing.T, h http.Handler, endpoint string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	req := authorized(http.MethodPost, "/api/v1/config/"+endpoint, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestConfigRawRoundTrip(t *testing.T) {
	s, path := fileBackedConfigServer(t, nil)
	h := s.Handler()
	original, _ := os.ReadFile(path)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/raw", nil))
	var snapshot configRawResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || snapshot.YAML != string(original) || snapshot.Path != path || snapshot.Version != 1 {
		t.Fatalf("snapshot: %s", rec.Body.String())
	}
	raw := strings.ReplaceAll(strings.ReplaceAll(snapshot.YAML, "name: Base", "name: Edited"), "\n", "\r\n") + "# trailing comment\n"
	rec = rawRequest(t, h, "raw", map[string]any{"yaml": raw, "version": 1})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":2`) {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	written, _ := os.ReadFile(path)
	if string(written) != raw || s.config().Rules[0].Name != "Edited" {
		t.Fatal("source or runtime mismatch")
	}
	rec = rawRequest(t, h, "raw", map[string]any{"yaml": raw, "version": 2})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":2`) {
		t.Fatalf("no-op: %s", rec.Body.String())
	}
	rec = rawRequest(t, h, "raw", map[string]any{"yaml": raw, "version": 1})
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "config_version_conflict") {
		t.Fatalf("stale: %s", rec.Body.String())
	}
	external := raw + "# external\n"
	if err := os.WriteFile(path, []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/config/raw", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.YAML != external || snapshot.Version != 2 {
		t.Fatal("raw read did not reflect disk")
	}
	rec = rawRequest(t, h, "raw", map[string]any{"yaml": raw, "version": 2})
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "config_dirty") {
		t.Fatalf("dirty: %s", rec.Body.String())
	}
}

func TestConfigRawValidation(t *testing.T) {
	s, path := fileBackedConfigServer(t, nil)
	h := s.Handler()
	original, _ := os.ReadFile(path)
	cases := []struct {
		name, raw, path string
		line            int
	}{
		{"syntax", "clients:\n  - id: surge\n    template: @\n", "config", 3},
		{"unknown field", string(original) + "unknown: true\n", "unknown", 12},
		{"duration", string(original) + "update:\n  fetch:\n    timeout: nonsense\n", "update.fetch.timeout", 14},
		{"reference", strings.ReplaceAll(string(original), "outputs: [surge]", "outputs: [missing]"), "rules[0].outputs[0]", 11},
		{"template", strings.ReplaceAll(string(original), "template: surge", "template: missing"), "clients[0]", 3},
		{"extra document", string(original) + "---\nclients: []\n", "config", 12},
		{"empty", "", "config", 1},
	}
	for _, tc := range cases {
		for _, ending := range []string{"\n", "\r\n"} {
			t.Run(tc.name+ending, func(t *testing.T) {
				raw := strings.ReplaceAll(tc.raw, "\n", ending)
				for _, endpoint := range []string{"validate", "raw"} {
					rec := rawRequest(t, h, endpoint, map[string]any{"yaml": raw, "version": 1})
					var response struct {
						Error struct {
							Details struct {
								Errors []configIssue `json:"errors"`
							} `json:"details"`
						} `json:"error"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if rec.Code != 422 || len(response.Error.Details.Errors) == 0 {
						t.Fatalf("%s: %d %s", endpoint, rec.Code, rec.Body.String())
					}
					issue := response.Error.Details.Errors[0]
					if issue.Line != tc.line || issue.Path != tc.path {
						t.Fatalf("issue=%+v, want line=%d path=%s", issue, tc.line, tc.path)
					}
				}
			})
		}
	}
	valid := strings.ReplaceAll(string(original), "id: surge", "id: new-client")
	valid = strings.ReplaceAll(valid, "outputs: [surge]", "outputs: [new-client]")
	rec := rawRequest(t, h, "validate", map[string]any{"yaml": valid})
	if rec.Code != 200 {
		t.Fatalf("valid: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(s.DataDir, "rules", "new-client")); !os.IsNotExist(err) {
		t.Fatalf("dry run created artifact directory: %v", err)
	}
	written, _ := os.ReadFile(path)
	_, version := s.ConfigManager.Snapshot()
	if string(written) != string(original) || version != 1 || s.config().Clients[0].ID != "surge" {
		t.Fatal("validation changed config")
	}
}

func TestConfigRawConcurrentSaves(t *testing.T) {
	s, path := fileBackedConfigServer(t, nil)
	original, _ := os.ReadFile(path)
	h := s.Handler()
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, name := range []string{"First", "Second"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			rec := rawRequest(t, h, "raw", map[string]any{"yaml": strings.ReplaceAll(string(original), "name: Base", "name: "+name), "version": 1})
			codes <- rec.Code
		}(name)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("statuses: %v", counts)
	}
}

func TestConfigRawRequestGuards(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	for _, endpoint := range []string{"raw", "validate"} {
		for _, tc := range []struct {
			body   string
			status int
		}{
			{`{}`, 422}, {`null`, 422}, {`{"yaml":null,"version":1}`, 422},
			{`{"yaml":42,"version":1}`, 422}, {`{"yaml":"","version":1,"unknown":true}`, 422},
			{`{"yaml":"","version":1} {}`, 422},
			{`{"yaml":"` + strings.Repeat("x", 1<<20) + `","version":1}`, 413},
		} {
			req := authorized(http.MethodPost, "/api/v1/config/"+endpoint, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("%s: got %d want %d", endpoint, rec.Code, tc.status)
			}
		}
		for _, mode := range []string{"anonymous", "cross-origin", "media-type"} {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/config/"+endpoint, strings.NewReader(`{"yaml":"","version":1}`))
			req.Header.Set("Content-Type", "application/json")
			status := 401
			if mode == "cross-origin" {
				req.AddCookie(&http.Cookie{Name: tokenCookieName, Value: "abc"})
				req.Header.Set("Origin", "https://attacker.example")
				status = 403
			}
			if mode == "media-type" {
				req.Header.Set("Authorization", "Bearer abc")
				req.Header.Del("Content-Type")
				status = 415
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != status {
				t.Fatalf("%s %s: got %d want %d", endpoint, mode, rec.Code, status)
			}
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/config/raw", nil))
	if rec.Code != 401 {
		t.Fatalf("unauthenticated read: %d", rec.Code)
	}
}

func TestConfigRawSaveDuringUpdate(t *testing.T) {
	runner := &blockingConfigRunner{started: make(chan struct{}), release: make(chan struct{})}
	s, path := fileBackedConfigServer(t, runner)
	job, err := s.updates.Start(updates.Request{Scope: "all"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	defer func() { close(runner.release); <-job.Done() }()
	original, _ := os.ReadFile(path)
	rec := rawRequest(t, s.Handler(), "raw", map[string]any{"yaml": string(original) + "# edit\n", "version": 1})
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "update_in_progress") {
		t.Fatalf("save during update: %d %s", rec.Code, rec.Body.String())
	}
	written, _ := os.ReadFile(path)
	if string(written) != string(original) {
		t.Fatal("rejected save changed disk")
	}
}
