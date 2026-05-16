package api

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

// newRestoreTargetStore opens a fresh, isolated store with the directory
// layout importGoBackupZip expects. Used as the destination for the
// backup→restore round-trip below.
func newRestoreTargetStore(t *testing.T) (*store.Store, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		DataDir:       dir,
		DBPath:        filepath.Join(dir, "db.sqlite"),
		RulesDir:      filepath.Join(dir, "Rules"),
		SourcesDir:    filepath.Join(dir, "sources"),
		GeositeDir:    filepath.Join(dir, "geosite"),
		IconSetDir:    filepath.Join(dir, "iconset"),
		ClientFileDir: filepath.Join(dir, "client"),
		WAFDir:        filepath.Join(dir, "waf"),
		OutDir:        filepath.Join(dir, "out"),
	}
	st, err := store.Open(cfg.DBPath, store.Paths{
		DataDir:       cfg.DataDir,
		RulesDir:      cfg.RulesDir,
		SourcesDir:    cfg.SourcesDir,
		GeositeDir:    cfg.GeositeDir,
		IconSetDir:    cfg.IconSetDir,
		ClientFileDir: cfg.ClientFileDir,
		WAFDir:        cfg.WAFDir,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, cfg
}

// TestDatabaseBackupRestore_PreservesClientFilesMetadata exercises the full
// HTTP backup endpoint → in-process importGoBackupZip → second store flow.
// Regression test for the bug where buildDatabaseSnapshot omitted client_files
// metadata, causing displayName / configId / isPublic to be lost on restore.
func TestDatabaseBackupRestore_PreservesClientFilesMetadata(t *testing.T) {
	srv, ts := newTestServer(t, "")
	ctx := context.Background()

	// Seed source state: a custom client + two client files (one public, one
	// private, one with a description) on the source server.
	if err := srv.Store.AddClient(ctx, schema.ClientConfig{ID: "Clash Meta", DisplayName: "Clash Meta"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	desc := "self-hosted full"
	cfA, err := srv.Store.CreateClientFile(ctx, "Clash Meta", store.ClientFileInput{
		ConfigID:    "self-full",
		DisplayName: "完整配置",
		Description: &desc,
		Ext:         "yaml",
		IsPublic:    true,
		Content:     "rules: []\n",
	})
	if err != nil {
		t.Fatalf("CreateClientFile A: %v", err)
	}
	cfB, err := srv.Store.CreateClientFile(ctx, "Clash Meta", store.ClientFileInput{
		ConfigID:    "short",
		DisplayName: "短规则",
		Ext:         "yaml",
		IsPublic:    false,
		Content:     "rules: []\n# short\n",
	})
	if err != nil {
		t.Fatalf("CreateClientFile B: %v", err)
	}

	// Pull the zip via the real HTTP endpoint so we exercise buildDatabaseSnapshot
	// + addDirToZip end-to-end.
	resp, err := http.Get(ts.URL + "/api/database/backup")
	if err != nil {
		t.Fatalf("GET /api/database/backup: %v", err)
	}
	zipBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("backup status: %d body=%s", resp.StatusCode, string(zipBytes))
	}
	if got := resp.Header.Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type: want application/zip, got %q", got)
	}

	// Sanity: the zip must contain client-files/<clientId>/<configId>.<ext>
	// for both rows. This guards against accidental zip-walker regressions.
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	wantPaths := map[string]bool{
		"client-files/Clash Meta/self-full.yaml": false,
		"client-files/Clash Meta/short.yaml":     false,
	}
	for _, f := range zr.File {
		if _, ok := wantPaths[f.Name]; ok {
			wantPaths[f.Name] = true
		}
	}
	for p, seen := range wantPaths {
		if !seen {
			t.Fatalf("backup zip missing %s", p)
		}
	}

	// Restore into a brand-new, empty store.
	dstStore, _ := newRestoreTargetStore(t)
	if err := importGoBackupZip(zr, dstStore); err != nil {
		t.Fatalf("importGoBackupZip: %v", err)
	}

	// Metadata must be byte-for-byte preserved.
	got, err := dstStore.ListAllClientFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllClientFiles after restore: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 client_files rows, got %d (%+v)", len(got), got)
	}
	byID := map[string]schema.ClientFileMeta{}
	for _, m := range got {
		byID[m.ID] = m
	}
	for _, want := range []schema.ClientFileMeta{cfA, cfB} {
		actual, ok := byID[want.ID]
		if !ok {
			t.Fatalf("restored row missing id=%s", want.ID)
		}
		if actual.ClientID != want.ClientID ||
			actual.ConfigID != want.ConfigID ||
			actual.DisplayName != want.DisplayName ||
			actual.Ext != want.Ext ||
			actual.IsPublic != want.IsPublic ||
			actual.CreatedAt != want.CreatedAt ||
			actual.UpdatedAt != want.UpdatedAt {
			t.Fatalf("metadata drift after restore for %s\n want %+v\n  got %+v", want.ID, want, actual)
		}
		if (actual.Description == nil) != (want.Description == nil) {
			t.Fatalf("description nil-ness drift for %s", want.ID)
		}
		if actual.Description != nil && *actual.Description != *want.Description {
			t.Fatalf("description drift for %s: got %q want %q", want.ID, *actual.Description, *want.Description)
		}
	}

	// Disk content survived too (zip walker landed it in the right place).
	for _, want := range []struct {
		client, configID, ext, content string
	}{
		{"Clash Meta", "self-full", "yaml", "rules: []\n"},
		{"Clash Meta", "short", "yaml", "rules: []\n# short\n"},
	} {
		path := filepath.Join(dstStore.ClientFileDir, want.client, want.configID+"."+want.ext)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored content %s: %v", path, err)
		}
		if string(data) != want.content {
			t.Fatalf("content drift for %s: got %q want %q", path, string(data), want.content)
		}
	}

	// And the public listing API on the restored store still recognizes the
	// public file — i.e., is_public=1 round-tripped, not just landed as 0.
	pubs, err := dstStore.ListPublicClientFiles(ctx)
	if err != nil {
		t.Fatalf("ListPublicClientFiles: %v", err)
	}
	if len(pubs) != 1 || pubs[0].ID != cfA.ID {
		ids := make([]string, len(pubs))
		for i, p := range pubs {
			ids[i] = p.ID
		}
		sort.Strings(ids)
		t.Fatalf("public listing drift: want [%s], got %v", cfA.ID, ids)
	}
}

// buildMinimalBackupZip creates a valid backup zip with the given db.json
// payload and optional extra file entries (prefix → relPath → content).
func buildMinimalBackupZip(t *testing.T, dbJSON string, files map[string]map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("db.json")
	if err != nil {
		t.Fatalf("create db.json in zip: %v", err)
	}
	if _, err := w.Write([]byte(dbJSON)); err != nil {
		t.Fatalf("write db.json: %v", err)
	}
	for prefix, entries := range files {
		for rel, content := range entries {
			w, err := zw.Create(prefix + rel)
			if err != nil {
				t.Fatalf("create %s%s: %v", prefix, rel, err)
			}
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("write %s%s: %v", prefix, rel, err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestDatabaseRestore_RemovesOldFilesNotInBackup verifies that files present on
// disk before restore but absent from the backup zip are removed after a
// successful restore.  Regression test: importGoBackupZip used to extract
// entries over the live directories, leaving stale files behind.
func TestDatabaseRestore_RemovesOldFilesNotInBackup(t *testing.T) {
	dstStore, _ := newRestoreTargetStore(t)

	// Seed pre-existing files that should disappear after restore.
	oldRuleDir := filepath.Join(dstStore.RulesDir, "stale-client")
	if err := os.MkdirAll(oldRuleDir, 0o755); err != nil {
		t.Fatalf("mkdir old rule dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldRuleDir, "stale-rule.txt"), []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("write stale rule: %v", err)
	}
	oldCFDir := filepath.Join(dstStore.ClientFileDir, "stale-client")
	if err := os.MkdirAll(oldCFDir, 0o755); err != nil {
		t.Fatalf("mkdir old client-file dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldCFDir, "stale.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatalf("write stale client file: %v", err)
	}

	// Build a backup zip that only has a new rule file (no stale entries).
	dbJSON := `{"version":1,"config":{"version":1,"rules":[]},"clients":null}`
	zipBytes := buildMinimalBackupZip(t, dbJSON, map[string]map[string]string{
		"Rules/": {"fresh-client/fresh-rule.txt": "fresh\n"},
	})

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if err := importGoBackupZip(zr, dstStore); err != nil {
		t.Fatalf("importGoBackupZip: %v", err)
	}

	// Stale files must be gone.
	if _, err := os.Stat(filepath.Join(dstStore.RulesDir, "stale-client")); !os.IsNotExist(err) {
		t.Errorf("stale Rules/ directory should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dstStore.ClientFileDir, "stale-client")); !os.IsNotExist(err) {
		t.Errorf("stale client-files/ directory should be removed, err=%v", err)
	}

	// New file from the backup must be present.
	data, err := os.ReadFile(filepath.Join(dstStore.RulesDir, "fresh-client", "fresh-rule.txt"))
	if err != nil {
		t.Fatalf("read fresh rule: %v", err)
	}
	if string(data) != "fresh\n" {
		t.Errorf("fresh rule content: got %q, want %q", string(data), "fresh\n")
	}

	// No leftover staging/old directories.
	for _, suffix := range []string{".restore-staging", ".restore-old"} {
		if _, err := os.Stat(dstStore.RulesDir + suffix); !os.IsNotExist(err) {
			t.Errorf("leftover %s directory should be cleaned up", suffix)
		}
	}
}

// TestDatabaseRestore_PreservesDataOnInvalidZip ensures that a failed restore
// does not touch the existing on-disk files.  The original directories must
// remain completely intact.
func TestDatabaseRestore_PreservesDataOnInvalidZip(t *testing.T) {
	dstStore, _ := newRestoreTargetStore(t)

	// Pre-seed files that must survive a failed restore.
	ruleDir := filepath.Join(dstStore.RulesDir, "my-client")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "my-rule.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Build a zip with an invalid db.json (missing "config" field).
	zipBytes := buildMinimalBackupZip(t, `{"version":1}`, nil)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if err := importGoBackupZip(zr, dstStore); err == nil {
		t.Fatal("expected error for invalid db.json")
	}

	// Original files must still be present and intact.
	data, err := os.ReadFile(filepath.Join(ruleDir, "my-rule.txt"))
	if err != nil {
		t.Fatalf("original file should still exist: %v", err)
	}
	if string(data) != "mine\n" {
		t.Errorf("original file content changed: got %q", string(data))
	}

	// No staging/old artifacts left behind.
	for _, suffix := range []string{".restore-staging", ".restore-old"} {
		if _, err := os.Stat(dstStore.RulesDir + suffix); !os.IsNotExist(err) {
			t.Errorf("leftover %s directory on failed restore", suffix)
		}
	}
}
