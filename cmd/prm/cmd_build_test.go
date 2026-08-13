package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommandCreatesStandaloneStaticSite(t *testing.T) {
	root := t.TempDir()
	configPath := writeBuildTestConfig(t, root, "content: DOMAIN,example.com")
	output := filepath.Join(root, "dist")
	withBuildCommandPaths(t, configPath, output)

	if err := buildCmd.RunE(buildCmd, nil); err != nil {
		t.Fatalf("build command: %v", err)
	}
	for _, name := range []string{"index.html", ".nojekyll", "rules/surge/rule.list", "static/icons/prm.svg"} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(name))); err != nil {
			t.Errorf("published file %s: %v", name, err)
		}
	}
	page, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "/admin") || strings.Contains(string(page), "/api/v1") {
		t.Fatal("standalone static page contains a service-only route")
	}
}

func TestBuildCommandPreservesOutputAfterUpdateError(t *testing.T) {
	root := t.TempDir()
	configPath := writeBuildTestConfig(t, root, "file: missing.list")
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(output, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("previous-output"), 0o644); err != nil {
		t.Fatal(err)
	}
	withBuildCommandPaths(t, configPath, output)

	if err := buildCmd.RunE(buildCmd, nil); err == nil {
		t.Fatal("build command succeeded after an update error")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "previous-output" {
		t.Fatalf("previous output changed: data=%q err=%v", data, err)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel.txt" {
		t.Fatalf("previous output contents changed: %v", entries)
	}
}

func TestRuntimeDataDirAppliesToValidatePreviewAndUpdate(t *testing.T) {
	root := t.TempDir()
	configPath := writeBuildTestConfig(t, root, "content: DOMAIN,example.com")
	dataDir := filepath.Join(root, "runtime-data")
	resetRuntimeFlags(t)
	previousConfig, previousDataDir, previousTarget := cfgFile, dataDirFlag, previewTarget
	cfgFile, dataDirFlag, previewTarget = configPath, dataDir, "surge"
	t.Cleanup(func() {
		cfgFile, dataDirFlag, previewTarget = previousConfig, previousDataDir, previousTarget
	})

	if err := validateCmd.RunE(validateCmd, nil); err != nil {
		t.Fatalf("validate command: %v", err)
	}
	if err := previewCmd.RunE(previewCmd, []string{"rule"}); err != nil {
		t.Fatalf("preview command: %v", err)
	}
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update command: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "rules", "surge", "rule.list")); err != nil {
		t.Fatalf("runtime artifact: %v", err)
	}
}

func writeBuildTestConfig(t *testing.T, root, source string) string {
	t.Helper()
	content := "clients:\n" +
		"  - id: surge\n" +
		"    name: Surge\n" +
		"    template: surge\n" +
		"rules:\n" +
		"  - id: rule\n" +
		"    name: Rule\n" +
		"    sources:\n" +
		"      - " + source + "\n" +
		"    outputs: [surge]\n" +
		"update:\n" +
		"  schedule:\n" +
		"    mode: manual\n" +
		"  fetch:\n" +
		"    retries: 0\n"
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func withBuildCommandPaths(t *testing.T, configPath, output string) {
	t.Helper()
	previousConfig, previousOutput, previousDataDir := cfgFile, buildOutput, dataDirFlag
	cfgFile, buildOutput, dataDirFlag = configPath, output, filepath.Join(filepath.Dir(configPath), "data")
	t.Cleanup(func() {
		cfgFile, buildOutput, dataDirFlag = previousConfig, previousOutput, previousDataDir
	})
}
