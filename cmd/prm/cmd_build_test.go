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

func writeBuildTestConfig(t *testing.T, root, source string) string {
	t.Helper()
	dataDir := strings.ReplaceAll(filepath.ToSlash(filepath.Join(root, "data")), "'", "''")
	content := "data_dir: '" + dataDir + "'\n" +
		"clients:\n" +
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
		"    retries: 0\n" +
		"serve:\n" +
		"  port: 3001\n"
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func withBuildCommandPaths(t *testing.T, configPath, output string) {
	t.Helper()
	previousConfig, previousOutput := cfgFile, buildOutput
	cfgFile, buildOutput = configPath, output
	t.Cleanup(func() {
		cfgFile, buildOutput = previousConfig, previousOutput
	})
}
