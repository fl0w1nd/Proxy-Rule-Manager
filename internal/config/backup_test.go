package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveBackupWritesRollingSnapshots(t *testing.T) {
	manager, dir := backupTestManager(t)
	first := time.Date(2026, 9, 8, 12, 18, 0, 0, time.UTC)
	if err := manager.SaveBackup(first); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".state", "backups", "config-20260908-121800.yaml")
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != backupTestSource {
		t.Fatalf("backup content:\n%s", written)
	}

	candidate, err := manager.Prepare(1, []PatchOp{{
		Type: "update_rule", ID: "base",
		Value: patchValue(t, `{"id":"base","name":"Updated","sources":[{"url":"https://rules.example/rules.list"}],"outputs":["surge"]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Commit(candidate); err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveBackup(first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, ".state", "backups", "config-20260908-121800-2.yaml")
	written, err = os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "name: Updated") {
		t.Fatalf("second backup should capture the committed source:\n%s", written)
	}

	items, err := manager.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "config-20260908-121800-2.yaml" || items[1].ID != "config-20260908-121800.yaml" {
		t.Fatalf("items=%+v", items)
	}
	if items[0].CreatedAt != "2026-09-08T12:18:00.000Z" {
		t.Fatalf("created_at=%s", items[0].CreatedAt)
	}
	if items[1].Added+items[1].Removed == 0 {
		t.Fatal("original snapshot should differ from the current source")
	}
	detail, err := manager.BackupDetail(items[1].ID)
	if err != nil || detail.YAML != backupTestSource || len(detail.Lines) == 0 {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}

	raw, err := manager.ReadBackup(items[1].ID)
	if err != nil || string(raw) != backupTestSource {
		t.Fatalf("read original backup: %v %s", err, raw)
	}
}

func TestSaveBackupPrunesOldestBeyondLimit(t *testing.T) {
	manager, dir := backupTestManager(t)
	start := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < maxConfigBackups+3; i++ {
		if err := manager.SaveBackup(start.Add(time.Duration(i) * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	items, err := manager.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != maxConfigBackups {
		t.Fatalf("kept %d backups", len(items))
	}
	if items[len(items)-1].ID != "config-20260908-120003.yaml" || items[0].ID != "config-20260908-120022.yaml" {
		t.Fatalf("items=%+v", items)
	}
	if _, err := os.Stat(filepath.Join(dir, ".state", "backups", "config-20260908-120000.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("oldest backup was retained")
	}
}

func TestReadBackupRejectsUnknownAndMissingIDs(t *testing.T) {
	manager, _ := backupTestManager(t)
	for _, id := range []string{"../config.yaml", "config.yaml", "config-20260908-121800.yaml", "foo/bar.yaml"} {
		if _, err := manager.ReadBackup(id); !errors.Is(err, ErrBackupNotFound) {
			t.Fatalf("id=%q err=%v", id, err)
		}
	}
}

func TestMemoryManagerSkipsBackups(t *testing.T) {
	manager := NewMemoryManager(&Config{})
	if err := manager.SaveBackup(time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := manager.ListBackups()
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

const backupTestSource = `clients:
  - id: surge
    name: Surge
    template: surge
rules:
  - id: base
    name: Base
    sources:
      - url: https://rules.example/rules.list
    outputs: [surge]
`

func backupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(backupTestSource), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	return manager, dir
}
