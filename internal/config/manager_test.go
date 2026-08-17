package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManagerPreservesSourceAndAdvancesVersion(t *testing.T) {
	t.Setenv("PRIVATE_RULE_HOST", "secret.example")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := `# managed config
clients:
  # keep client comment
  - id: surge
    name: Surge
    template: surge
rules:
  - id: base
    name: Base
    sources:
      - url: https://${PRIVATE_RULE_HOST}/rules.list
    outputs: [surge]
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, version := manager.Snapshot()
	if version != 1 || cfg.Rules[0].Sources[0].URL != "https://secret.example/rules.list" {
		t.Fatalf("version=%d url=%q", version, cfg.Rules[0].Sources[0].URL)
	}
	rawView, _, err := manager.SourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	encodedView, err := json.Marshal(rawView)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encodedView), `${PRIVATE_RULE_HOST}`) {
		t.Fatalf("source view expanded environment variable: %s", encodedView)
	}

	value := patchValue(t, `{"id":"extra","name":"Extra","sources":[{"content":"DOMAIN,extra.example"}],"outputs":["surge"]}`)
	candidate, err := manager.Prepare(1, []PatchOp{{Type: "add_rule", Value: value}})
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.Changed() {
		t.Fatal("add_rule candidate reported unchanged")
	}
	committed, version, err := manager.Commit(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 || len(committed.Rules) != 2 {
		t.Fatalf("version=%d rules=%d", version, len(committed.Rules))
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	for _, want := range []string{"# managed config", "# keep client comment", `${PRIVATE_RULE_HOST}`, "id: extra"} {
		if !strings.Contains(text, want) {
			t.Fatalf("written config missing %q:\n%s", want, text)
		}
	}

	_, err = manager.Prepare(1, []PatchOp{{Type: "remove_rule", ID: "extra"}})
	var conflict *VersionConflictError
	if !errors.As(err, &conflict) || conflict.CurrentVersion != 2 {
		t.Fatalf("stale version error=%#v", err)
	}
}

func TestManagerDetectsExternalChangesAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "clients:\n  - id: surge\n    name: Surge\n    template: surge\nrules: []\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	external := source + "# external edit\n"
	if err := os.WriteFile(path, []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := manager.Dirty(); err != nil || !dirty {
		t.Fatalf("dirty=%v err=%v", dirty, err)
	}
	_, err = manager.Prepare(1, []PatchOp{{Type: "reorder_rules", Order: []string{}}})
	var dirtyErr *DirtyConfigError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("prepare error=%#v", err)
	}
	candidate, err := manager.PrepareReload()
	if err != nil {
		t.Fatal(err)
	}
	_, version, err := manager.Commit(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("version=%d", version)
	}
	if dirty, err := manager.Dirty(); err != nil || dirty {
		t.Fatalf("dirty after reload=%v err=%v", dirty, err)
	}
}

func TestManagerSnapshotsRemainIsolatedDuringConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "clients:\n  - id: surge\n    name: Surge\n    template: surge\nrules:\n  - id: base\n    name: Base\n    sources: [{content: \"DOMAIN,base.example\"}]\n    outputs: [surge]\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, _ := manager.Snapshot()
	snapshot.Rules[0].Name = "mutated snapshot"
	fresh, _ := manager.Snapshot()
	if fresh.Rules[0].Name != "Base" {
		t.Fatalf("manager changed through snapshot: %q", fresh.Rules[0].Name)
	}

	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 100; j++ {
				cfg, version := manager.Snapshot()
				if version < 1 || len(cfg.Rules) != 1 {
					t.Errorf("version=%d rules=%d", version, len(cfg.Rules))
					return
				}
			}
		}()
	}
	candidate, err := manager.Prepare(1, []PatchOp{{
		Type: "update_rule", ID: "base",
		Value: patchValue(t, `{"id":"base","name":"Updated","sources":[{"content":"DOMAIN,base.example"}],"outputs":["surge"]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, version, err := manager.Commit(candidate); err != nil || version != 2 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	readers.Wait()
}

func TestManagerNoopCommitDetectsLateExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "clients:\n  - id: surge\n    name: Surge\n    template: surge\nrules:\n  - id: base\n    name: Base\n    sources: [{content: \"DOMAIN,base.example\"}]\n    outputs: [surge]\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := manager.Prepare(1, []PatchOp{{Type: "add_output", RuleID: "base", OutputID: "surge"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source+"# late edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, version, err := manager.Commit(candidate)
	var dirtyErr *DirtyConfigError
	if !errors.As(err, &dirtyErr) || version != 1 {
		t.Fatalf("version=%d error=%#v", version, err)
	}
}

func TestManagerWriteFailureKeepsRuntimeAndSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "clients:\n  - id: surge\n    name: Surge\n    template: surge\nrules:\n  - id: base\n    name: Base\n    sources: [{content: \"DOMAIN,base.example\"}]\n    outputs: [surge]\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := manager.Prepare(1, []PatchOp{{
		Type: "update_rule", ID: "base",
		Value: patchValue(t, `{"id":"base","name":"Updated","sources":[{"content":"DOMAIN,base.example"}],"outputs":["surge"]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, _, err := manager.Commit(candidate); err == nil {
		t.Fatal("commit succeeded in a non-writable directory")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg, version := manager.Snapshot()
	if version != 1 || cfg.Rules[0].Name != "Base" {
		t.Fatalf("version=%d rule=%q", version, cfg.Rules[0].Name)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != source {
		t.Fatalf("source changed after write failure:\n%s", written)
	}
}

func patchValue(t *testing.T, value string) *yaml.Node {
	t.Helper()
	node, err := ParsePatchValue([]byte(value))
	if err != nil {
		t.Fatal(err)
	}
	return node
}
