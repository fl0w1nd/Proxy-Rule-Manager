package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestUpdateClientRenameMovesDirectoriesAndCommitsDB exercises the happy path
// of the two-phase rename: directories move first, then the DB cascade
// commits.
func TestUpdateClientRenameMovesDirectoriesAndCommitsDB(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "old_id", DisplayName: "Old"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "old_id"), 0o755); err != nil {
		t.Fatalf("mkdir rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.RulesDir, "old_id", "Sample.list"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.ClientFileDir, "old_id"), 0o755); err != nil {
		t.Fatalf("mkdir clientfile: %v", err)
	}

	if err := s.UpdateClient(ctx, "old_id", schema.ClientConfig{ID: "new_id", DisplayName: "New"}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}

	if _, err := os.Stat(filepath.Join(s.RulesDir, "old_id")); !os.IsNotExist(err) {
		t.Fatalf("old rules dir should be gone, stat err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(s.RulesDir, "new_id", "Sample.list")); err != nil || string(data) != "payload" {
		t.Fatalf("rule not moved to new path: data=%q err=%v", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(s.ClientFileDir, "new_id")); err != nil {
		t.Fatalf("client file dir not moved: %v", err)
	}
}

// TestUpdateClientRenameRefusesOnTargetExists ensures the pre-flight check
// surfaces a clear error and leaves both filesystem and DB untouched.
func TestUpdateClientRenameRefusesOnTargetExists(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "src", DisplayName: "src"}); err != nil {
		t.Fatalf("AddClient src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	// Pre-create the target directory to trigger the conflict.
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "dst"), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	if err := s.UpdateClient(ctx, "src", schema.ClientConfig{ID: "dst", DisplayName: "dst"}); err == nil {
		t.Fatalf("expected error when target rules dir already exists")
	}

	// DB still has src, not dst.
	clients, _ := s.GetClients(ctx)
	hasSrc := false
	for _, c := range clients {
		if c.ID == "dst" {
			t.Fatalf("rename should not have committed: %+v", clients)
		}
		if c.ID == "src" {
			hasSrc = true
		}
	}
	if !hasSrc {
		t.Fatalf("src client missing after failed rename: %+v", clients)
	}
}
