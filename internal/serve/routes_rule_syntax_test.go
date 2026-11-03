package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRuleSyntaxListsTypesAndFlags(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/rule-syntax", nil))
	if rec.Code != 200 {
		t.Fatalf("syntax: %d %s", rec.Code, rec.Body.String())
	}
	var body ruleSyntaxResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Types) == 0 || body.Types[0].Name != "DOMAIN" || len(body.Flags) == 0 {
		t.Fatalf("body=%+v", body)
	}
}

func TestRuleSyntaxLintReportsDiagnosticsAndWarnings(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	rec := localFileJSON(h, http.MethodPost, "/api/v1/rule-syntax/lint", `{"name":"x.yaml","content":"DOMAIN,a.com\nNOPE,b\n"}`)
	if rec.Code != 200 {
		t.Fatalf("lint: %d %s", rec.Code, rec.Body.String())
	}
	var body ruleSyntaxLintResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Format != "classical" || body.Entries != 1 || body.DiagnosticCount != 1 || body.Diagnostics[0].Line != 2 {
		t.Fatalf("body=%+v", body)
	}
	if len(body.Lines) != 1 || body.Lines[0].Line != 1 || len(body.Warnings) != 1 {
		t.Fatalf("lines=%+v warnings=%v", body.Lines, body.Warnings)
	}

	rec = localFileJSON(h, http.MethodPost, "/api/v1/rule-syntax/lint", `{"content":"payload:\n  - DOMAIN,a.com\n"}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 {
		t.Fatalf("yaml: %d %s", rec.Code, rec.Body.String())
	}
	if body.Format != "mihomo-yaml" || len(body.Warnings) != 0 {
		t.Fatalf("body=%+v", body)
	}

	rec = localFileJSON(h, http.MethodPost, "/api/v1/rule-syntax/lint", `{"name":"x.list"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing content: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLocalFileItemsCarryLintSummaryAndRefs(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	h := s.Handler()
	if err := os.MkdirAll(filepath.Join(s.DataDir, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.DataDir, "local", "used.list"), []byte("DOMAIN,a.com\nNOPE,b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := patchConfig(h, `{"version":1,"ops":[{"op":"update_rule","id":"base","value":{"id":"base","name":"Base","sources":[{"group":[{"file":"used.list"}]}],"outputs":["surge"]}}]}`)
	if rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/local-files", nil))
	var listed struct{ Items []localFileItem }
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	item := listed.Items[0]
	if item.Format != "classical" || item.Entries != 1 || item.Diagnostics != 1 || len(item.Rules) != 1 || item.Rules[0].ID != "base" {
		t.Fatalf("item=%+v", item)
	}

	rec = localFileJSON(h, http.MethodPut, "/api/v1/local-files/used.list", `{"content":"DOMAIN,a.com\n"}`)
	var detail localFileDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if detail.Diagnostics != 0 || len(detail.Rules) != 1 {
		t.Fatalf("detail=%+v", detail)
	}
}

func TestRulePreviewReturnsDiagnosticItems(t *testing.T) {
	s, _ := fileBackedConfigServer(t, nil)
	rec := localFileJSON(s.Handler(), http.MethodPost, "/api/v1/rules/preview", `{"rule":{"id":"draft","name":"Draft","sources":[{"content":"DOMAIN,a.com\nNOPE,b\n"}],"outputs":[]}}`)
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var report rulePreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	source := report.Sources[0]
	if source.Diagnostics != 1 || len(source.DiagnosticItems) != 1 || source.DiagnosticItems[0].Line != 2 || source.DiagnosticItems[0].Text != "NOPE,b" {
		t.Fatalf("source=%+v", source)
	}
}
