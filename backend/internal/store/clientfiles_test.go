package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestListAllClientFilesAcrossClients verifies that ListAllClientFiles returns
// every row regardless of client, ordered by (client_id, created_at).
func TestListAllClientFilesAcrossClients(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Two clients, three files in total.
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "alpha", DisplayName: "Alpha"}); err != nil {
		t.Fatalf("AddClient alpha: %v", err)
	}
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "beta", DisplayName: "Beta"}); err != nil {
		t.Fatalf("AddClient beta: %v", err)
	}

	desc := "primary config"
	mustCreate := func(client, configID, ext string, isPublic bool, description *string) schema.ClientFileMeta {
		t.Helper()
		m, err := s.CreateClientFile(ctx, client, ClientFileInput{
			ConfigID:    configID,
			DisplayName: configID + " (" + client + ")",
			Description: description,
			Ext:         ext,
			IsPublic:    isPublic,
			Content:     "payload-" + client + "-" + configID,
		})
		if err != nil {
			t.Fatalf("CreateClientFile %s/%s: %v", client, configID, err)
		}
		return m
	}

	a1 := mustCreate("alpha", "default", "yaml", true, &desc)
	a2 := mustCreate("alpha", "short", "yaml", false, nil)
	b1 := mustCreate("beta", "default", "yaml", true, nil)

	got, err := s.ListAllClientFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllClientFiles: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d (%+v)", len(got), got)
	}

	// Build an ID map so order assertions are robust against equal createdAt.
	byID := map[string]schema.ClientFileMeta{}
	for _, m := range got {
		byID[m.ID] = m
	}
	for _, want := range []schema.ClientFileMeta{a1, a2, b1} {
		actual, ok := byID[want.ID]
		if !ok {
			t.Fatalf("missing entry for %s", want.ID)
		}
		if actual.ClientID != want.ClientID ||
			actual.ConfigID != want.ConfigID ||
			actual.DisplayName != want.DisplayName ||
			actual.Ext != want.Ext ||
			actual.IsPublic != want.IsPublic ||
			actual.CreatedAt != want.CreatedAt ||
			actual.UpdatedAt != want.UpdatedAt {
			t.Fatalf("metadata mismatch for %s\n want %+v\n  got %+v", want.ID, want, actual)
		}
		if (actual.Description == nil) != (want.Description == nil) {
			t.Fatalf("description nil-ness mismatch for %s", want.ID)
		}
		if actual.Description != nil && *actual.Description != *want.Description {
			t.Fatalf("description mismatch for %s: got %q want %q", want.ID, *actual.Description, *want.Description)
		}
	}

	// Order: clients alphabetical (alpha then beta), then created_at ascending.
	if !sort.SliceIsSorted(got, func(i, j int) bool {
		if got[i].ClientID != got[j].ClientID {
			return got[i].ClientID < got[j].ClientID
		}
		return got[i].CreatedAt < got[j].CreatedAt
	}) {
		t.Fatalf("ListAllClientFiles not sorted by (client_id, created_at): %+v", got)
	}
}

// TestRestoreClientFilePreservesMetadata exercises the full backup→restore
// round-trip at the store layer: snapshot → wipe → restore → re-list, asserting
// every metadata field (incl. timestamps + isPublic + description) is bit-exact.
func TestRestoreClientFilePreservesMetadata(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "Clash Meta", DisplayName: "Clash Meta"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}

	desc := "self-hosted full config"
	original, err := s.CreateClientFile(ctx, "Clash Meta", ClientFileInput{
		ConfigID:    "self-full",
		DisplayName: "完整配置",
		Description: &desc,
		Ext:         "yaml",
		IsPublic:    true,
		Content:     "rules: []\n",
	})
	if err != nil {
		t.Fatalf("CreateClientFile: %v", err)
	}

	snap, err := s.ListAllClientFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllClientFiles: %v", err)
	}
	if len(snap) != 1 {
		t.Fatalf("want 1 row in snapshot, got %d", len(snap))
	}

	// Drop the row but keep the disk file (mirrors the restore flow, where
	// the zip walker has already unpacked client-files/<id>/<configId>.<ext>).
	if err := s.DeleteClientFile(ctx, original.ID); err != nil {
		t.Fatalf("DeleteClientFile: %v", err)
	}
	if rows, err := s.ListAllClientFiles(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("expected empty list after delete, got rows=%v err=%v", rows, err)
	}

	if err := s.RestoreClientFile(ctx, snap[0]); err != nil {
		t.Fatalf("RestoreClientFile: %v", err)
	}

	restored, err := s.GetClientFileMeta(ctx, original.ID)
	if err != nil {
		t.Fatalf("GetClientFileMeta after restore: %v", err)
	}
	if restored.ID != original.ID ||
		restored.ClientID != original.ClientID ||
		restored.ConfigID != original.ConfigID ||
		restored.DisplayName != original.DisplayName ||
		restored.Ext != original.Ext ||
		restored.IsPublic != original.IsPublic ||
		restored.CreatedAt != original.CreatedAt ||
		restored.UpdatedAt != original.UpdatedAt {
		t.Fatalf("metadata mismatch after restore\n want %+v\n  got %+v", original, restored)
	}
	if restored.Description == nil || *restored.Description != desc {
		t.Fatalf("description not preserved: got %v want %q", restored.Description, desc)
	}
}

// TestCreateClientFileAtomicConflictLeavesNoOrphan verifies the
// temp→DB→rename ordering: a duplicate (configId, ext) insert must fail
// without leaving stray files in the client directory.
func TestCreateClientFileAtomicConflictLeavesNoOrphan(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "c1", DisplayName: "C1"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	first, err := s.CreateClientFile(ctx, "c1", ClientFileInput{
		ConfigID: "main", DisplayName: "main", Ext: "yaml", Content: "v1",
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	finalPath := clientFilePath(s.ClientFileDir, "c1", "main", "yaml")
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("first create did not produce on-disk file: %v", err)
	}

	// Duplicate (configId, ext) must fail …
	if _, err := s.CreateClientFile(ctx, "c1", ClientFileInput{
		ConfigID: "main", DisplayName: "main2", Ext: "yaml", Content: "v2-clobber",
	}); err == nil {
		t.Fatalf("expected duplicate insert to fail")
	}

	// … and the existing file content for `first` must still be v1, with
	// no leftover temp file polluting the directory.
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read after conflict: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("expected v1, got %q", string(got))
	}
	dir := filepath.Dir(finalPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file after failed insert: %s", e.Name())
		}
	}
	_ = first
}

// TestUpdateClientFileChangesIdentifierAtomically ensures changing configId
// renames the on-disk file to the new path while removing the old one, and
// keeps content intact.
func TestUpdateClientFileChangesIdentifierAtomically(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "cc", DisplayName: "cc"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	created, err := s.CreateClientFile(ctx, "cc", ClientFileInput{
		ConfigID: "a", DisplayName: "a", Ext: "yaml", Content: "hello",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	oldPath := clientFilePath(s.ClientFileDir, "cc", "a", "yaml")
	newPath := clientFilePath(s.ClientFileDir, "cc", "b", "yaml")

	if _, err := s.UpdateClientFile(ctx, created.ID, ClientFileInput{
		ConfigID: "b", Ext: "yaml",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old path should be gone after identifier change, stat err=%v", err)
	}
	content, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new path: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content drift after rename: got %q", string(content))
	}
}

// TestUpdateClientFileConflictPreservesOldFile asserts that when the target
// (configId, ext) is already taken, the old file is not destroyed.
func TestUpdateClientFileConflictPreservesOldFile(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "cx", DisplayName: "cx"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if _, err := s.CreateClientFile(ctx, "cx", ClientFileInput{
		ConfigID: "keep", DisplayName: "keep", Ext: "yaml", Content: "keep-payload",
	}); err != nil {
		t.Fatalf("create keep: %v", err)
	}
	moving, err := s.CreateClientFile(ctx, "cx", ClientFileInput{
		ConfigID: "moving", DisplayName: "moving", Ext: "yaml", Content: "moving-payload",
	})
	if err != nil {
		t.Fatalf("create moving: %v", err)
	}

	// Attempt to rename moving → keep (already taken).
	if _, err := s.UpdateClientFile(ctx, moving.ID, ClientFileInput{
		ConfigID: "keep", Ext: "yaml",
	}); err == nil {
		t.Fatalf("expected conflict error")
	}

	movingPath := clientFilePath(s.ClientFileDir, "cx", "moving", "yaml")
	keepPath := clientFilePath(s.ClientFileDir, "cx", "keep", "yaml")
	if data, err := os.ReadFile(movingPath); err != nil || string(data) != "moving-payload" {
		t.Fatalf("moving file changed after failed update: data=%q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(keepPath); err != nil || string(data) != "keep-payload" {
		t.Fatalf("keep file changed after failed update: data=%q err=%v", string(data), err)
	}
}

// TestRestoreClientFileRejectsIncomplete makes sure the lightweight restore
// path refuses obviously broken metadata so a corrupt backup doesn't silently
// insert junk rows.
func TestRestoreClientFileRejectsIncomplete(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	cases := []schema.ClientFileMeta{
		{ID: "", ClientID: "c", ConfigID: "x", Ext: "yaml"},
		{ID: "id", ClientID: "", ConfigID: "x", Ext: "yaml"},
		{ID: "id", ClientID: "c", ConfigID: "", Ext: "yaml"},
	}
	for i, m := range cases {
		if err := s.RestoreClientFile(ctx, m); err == nil {
			t.Fatalf("case %d: expected error for incomplete meta %+v", i, m)
		}
	}
}
