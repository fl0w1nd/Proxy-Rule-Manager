package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerAppliesStructuredOperationsAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := `clients:
  - id: a
    name: A
    template: surge
  - id: b
    name: B
    template: surge
rules:
  - id: r1
    name: R1
    sources: [{content: "DOMAIN,r1.example"}]
    outputs: [a]
  - id: r2
    name: R2
    sources: [{ref: r1}]
    outputs: [a]
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	ops := []PatchOp{
		{Type: "add_client", Value: patchValue(t, `{"id":"c","name":"C","template":"surge"}`)},
		{Type: "update_client", ID: "b", Value: patchValue(t, `{"id":"b","name":"Bee","template":"surge"}`)},
		{Type: "add_rule", Value: patchValue(t, `{"id":"r3","name":"R3","sources":[{"content":"DOMAIN,r3.example"}],"outputs":["c"]}`)},
		{Type: "update_rule", ID: "r2", Value: patchValue(t, `{"id":"r2","name":"R2 updated","sources":[{"content":"DOMAIN,r2.example"}],"outputs":["a"]}`)},
		{Type: "batch_add_output", RuleIDs: []string{"r1", "r2"}, OutputIDs: []string{"b"}},
		{Type: "add_output", RuleID: "r3", OutputID: "b"},
		{Type: "remove_output", RuleID: "r1", OutputID: "a"},
		{Type: "batch_remove_output", RuleIDs: []string{"r2"}, OutputIDs: []string{"a"}},
		{Type: "remove_client", ID: "a"},
		{Type: "remove_rule", ID: "r1"},
		{Type: "reorder_rules", Order: []string{"r3", "r2"}},
		{Type: "update_schedule", Value: patchValue(t, `{"mode":"interval","interval":"30m","timezone":"UTC"}`)},
		{Type: "update_fetch", Value: patchValue(t, `{"timeout":"20s","max_download":"5MB","concurrency":3,"per_host_concurrency":2,"retries":1,"retry_delay":"1s","user_agent":"PRM-Test"}`)},
		{Type: "update_preprocess", Value: patchValue(t, `{"timeout":"3s","max_output":"2MB"}`)},
		{Type: "update_history", Value: patchValue(t, `{"history_retention":"48h","history_limit":50}`)},
		{Type: "update_geosite", Value: patchValue(t, `{"providers":[{"name":"v2fly","clients":["c"]}]}`)},
	}
	candidate, err := manager.Prepare(1, ops)
	if err != nil {
		t.Fatal(err)
	}
	cfg, version, err := manager.Commit(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 || len(cfg.Clients) != 2 || cfg.Clients[0].ID != "b" || cfg.Clients[0].Name != "Bee" {
		t.Fatalf("version=%d clients=%+v", version, cfg.Clients)
	}
	if len(cfg.Rules) != 2 || cfg.Rules[0].ID != "r3" || cfg.Rules[1].ID != "r2" {
		t.Fatalf("rules=%+v", cfg.Rules)
	}
	if len(cfg.Rules[1].Outputs) != 1 || cfg.Rules[1].Outputs[0] != "b" {
		t.Fatalf("r2 outputs=%v", cfg.Rules[1].Outputs)
	}
	if cfg.Update.Schedule.Mode != "interval" || cfg.Update.Fetch.UserAgent != "PRM-Test" || cfg.Update.HistoryLimit != 50 {
		t.Fatalf("update=%+v", cfg.Update)
	}
	if cfg.Geosite == nil || cfg.Geosite.Providers[0].Clients[0] != "c" {
		t.Fatalf("geosite=%+v", cfg.Geosite)
	}

	noop, err := manager.Prepare(2, []PatchOp{{Type: "add_output", RuleID: "r2", OutputID: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if noop.Changed() {
		t.Fatal("idempotent add_output reported a change")
	}
	_, sameVersion, err := manager.Commit(noop)
	if err != nil || sameVersion != 2 {
		t.Fatalf("version=%d err=%v", sameVersion, err)
	}
}

func TestManagerRejectsInvalidTransactionWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "clients:\n  - id: a\n    name: A\n    template: surge\nrules:\n  - id: r\n    name: R\n    sources: [{content: \"DOMAIN,r.example\"}]\n    outputs: [a]\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Prepare(1, []PatchOp{{Type: "remove_client", ID: "a"}})
	var configErrs ConfigErrors
	if !errors.As(err, &configErrs) {
		t.Fatalf("error=%#v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != source {
		t.Fatalf("invalid transaction changed source:\n%s", written)
	}
	_, version := manager.Snapshot()
	if version != 1 {
		t.Fatalf("version=%d", version)
	}
}

func TestPatchPreservesUntouchedYAMLPresentation(t *testing.T) {
	t.Setenv("RULE_HOST", "effective.example")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := `# document comment
clients:
  - id: surge
    name: 'Surge Styled' # scalar comment
    template: surge
rules:
  - id: first
    name: First
    sources:
      - url: "https://${RULE_HOST}/first.list"
    outputs: [surge]
  - id: second
    name: Second
    sources: [{content: "DOMAIN,second.example"}]
    outputs: [surge]
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := manager.Prepare(1, []PatchOp{{
		Type: "update_rule", ID: "second",
		Value: patchValue(t, `{"id":"second","name":"Second Updated","sources":[{"content":"DOMAIN,second.example"}],"outputs":["surge"]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Commit(candidate); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	for _, want := range []string{
		"# document comment", "name: 'Surge Styled' # scalar comment",
		`url: "https://${RULE_HOST}/first.list"`, "outputs: [surge]",
		"id: first", "id: second", "name: Second Updated",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("written config missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "id: first") > strings.Index(text, "id: second") {
		t.Fatalf("rule order changed:\n%s", text)
	}
}

func TestPatchRejectsInvalidReorderAndClearsGeosite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := `clients:
  - id: surge
    name: Surge
    template: surge
rules:
  - id: first
    name: First
    sources: [{content: "DOMAIN,first.example"}]
    outputs: [surge]
  - id: second
    name: Second
    sources: [{content: "DOMAIN,second.example"}]
    outputs: [surge]
geosite:
  providers:
    - name: v2fly
      clients: [surge]
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Prepare(1, []PatchOp{{Type: "reorder_rules", Order: []string{"first", "first"}}})
	var patchErr *PatchError
	if !errors.As(err, &patchErr) || patchErr.Path != "order" {
		t.Fatalf("reorder error=%#v", err)
	}

	candidate, err := manager.Prepare(1, []PatchOp{{Type: "update_geosite", Value: patchValue(t, `null`)}})
	if err != nil {
		t.Fatal(err)
	}
	cfg, version, err := manager.Commit(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 || cfg.Geosite != nil {
		t.Fatalf("version=%d geosite=%+v", version, cfg.Geosite)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "geosite:") {
		t.Fatalf("geosite remains in source:\n%s", written)
	}
}
