package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
)

func TestExportStaticWritesPublicWhitelistAndReplacesOldOutput(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.FullUpdate(context.Background())
	if len(result.Errors) != 0 {
		t.Fatalf("full update: %v", result.Errors)
	}

	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := eng.ExportStatic(output); err != nil {
		t.Fatalf("ExportStatic: %v", err)
	}
	topLevel, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	var topLevelNames []string
	for _, entry := range topLevel {
		topLevelNames = append(topLevelNames, entry.Name())
	}
	if got, want := strings.Join(topLevelNames, ","), ".nojekyll,index.html,rules,static"; got != want {
		t.Fatalf("published top-level paths = %q, want %q", got, want)
	}

	for _, name := range []string{"index.html", ".nojekyll", "rules/surge/rule.list", "static/icons/prm.svg"} {
		if info, err := os.Stat(filepath.Join(output, filepath.FromSlash(name))); err != nil || info.IsDir() {
			t.Errorf("published file %s: info=%v err=%v", name, info, err)
		}
	}
	for _, name := range []string{"stale.txt", ".state", "admin.html", "static/.builtin-assets.json"} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Errorf("unexpected published path %s: %v", name, err)
		}
	}
	page, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"规则索引 · 更新于", "Rule", "rules/surge/rule.list", "static/icons/prm.svg"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("static index missing %q", want)
		}
	}
	for _, excluded := range []string{"/admin", "/api/v1", "构建日志", "发布成功"} {
		if strings.Contains(string(page), excluded) {
			t.Errorf("static index contains %q", excluded)
		}
	}
}

func TestExportStaticRejectsSymlinksAndPreservesOldOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional Windows privileges")
	}
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("full update: %v", result.Errors)
	}
	outside := filepath.Join(root, "outside.list")
	if err := os.WriteFile(outside, []byte("DOMAIN,outside.example"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dataDir, "rules", "surge", "linked.list")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(output, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := eng.ExportStatic(output); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("ExportStatic error = %v", err)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "previous" {
		t.Fatalf("previous output changed: data=%q err=%v", data, err)
	}
}

func TestValidateStaticOutputPathProtectsRootsAndData(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filesystemRoot := filepath.VolumeName(root) + string(filepath.Separator)
	for _, output := range []string{filesystemRoot, dataDir, filepath.Join(dataDir, "dist"), root} {
		if _, err := validateStaticOutputPath(output, dataDir); err == nil {
			t.Errorf("output %q was accepted", output)
		}
	}
	if got, err := validateStaticOutputPath(filepath.Join(root, "dist"), dataDir); err != nil || got == "" {
		t.Fatalf("safe output rejected: got=%q err=%v", got, err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "dist-link")
		if err := os.Symlink(filepath.Join(root, "dist"), link); err != nil {
			t.Fatal(err)
		}
		if _, err := validateStaticOutputPath(link, dataDir); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("symbolic-link output error = %v", err)
		}
	}
}

func TestReplaceStaticDirectoryRestoresPreviousOutputAfterPublishFailure(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(output, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingStaging := filepath.Join(root, ".dist.staging-missing")
	if err := replaceStaticDirectory(missingStaging, output); err == nil {
		t.Fatal("replaceStaticDirectory accepted a missing staging directory")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "previous" {
		t.Fatalf("previous output was not restored: data=%q err=%v", data, err)
	}
}
