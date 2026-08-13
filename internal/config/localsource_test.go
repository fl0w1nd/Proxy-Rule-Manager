package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func resolverCfg(t *testing.T, dataDir string) *Config {
	t.Helper()
	return &Config{DataDir: dataDir}
}

func TestLocalFileResolverAllowsFilesInsideRoot(t *testing.T) {
	dataDir := t.TempDir()
	localDir := filepath.Join(dataDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(localDir, "rules.list")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := resolverCfg(t, dataDir)
	resolve := cfg.LocalFileResolver()

	got, err := resolve(target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, readErr := os.ReadFile(got); readErr != nil {
		t.Fatalf("read resolved path %q: %v", got, readErr)
	}
}

func TestLocalFileResolverAnchorsRelativeAtLocal(t *testing.T) {
	dataDir := t.TempDir()
	localDir := filepath.Join(dataDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(localDir, "custom.list")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := resolverCfg(t, dataDir)
	resolve := cfg.LocalFileResolver()

	// A bare relative name resolves against data_dir/local, not the CWD.
	got, err := resolve("custom.list")
	if err != nil {
		t.Fatalf("relative resolve: %v", err)
	}
	if _, readErr := os.ReadFile(got); readErr != nil {
		t.Fatalf("read resolved relative path %q: %v", got, readErr)
	}
	// A nested relative path is anchored at the same root.
	if _, err := resolve("nested/deeper.list"); err != nil {
		t.Fatalf("nested relative resolve: %v", err)
	}
}

func TestLocalFileResolverRejectsTraversalAndAbsoluteEscape(t *testing.T) {
	dataDir := t.TempDir()
	cfg := resolverCfg(t, dataDir)
	resolve := cfg.LocalFileResolver()

	for _, tc := range []struct {
		name string
		file string
	}{
		{"parent traversal", filepath.Join(dataDir, "..", "secret.list")},
		{"absolute outside", "/etc/passwd"},
		{"deep traversal", filepath.Join(dataDir, "..", "..", "etc", "hosts")},
		{"sibling under data_dir", filepath.Join(dataDir, "rules", "escape.list")},
		{"relative parent traversal", "../secret.list"},
		{"relative deep traversal", "../../etc/hosts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolve(tc.file); err == nil {
				t.Fatalf("expected rejection for %q", tc.file)
			}
		})
	}
}

func TestLocalFileResolverRejectsSymlinkEscape(t *testing.T) {
	dataDir := t.TempDir()
	localDir := filepath.Join(dataDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Place a symlink inside data_dir/local that points outside it.
	link := filepath.Join(localDir, "escape.list")
	if err := os.Symlink(outsideFile, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks not supported on this Windows environment")
		}
		t.Fatalf("symlink: %v", err)
	}
	cfg := resolverCfg(t, dataDir)
	if _, err := cfg.LocalFileResolver()(link); err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
}

func TestLocalFileResolverNonExistentInsideRootIsAllowed(t *testing.T) {
	// A missing file inside the root must pass containment (so the caller's
	// ReadFile surfaces the not-found error rather than a containment error).
	dataDir := t.TempDir()
	cfg := resolverCfg(t, dataDir)
	got, err := cfg.LocalFileResolver()(filepath.Join(dataDir, "local", "missing.list"))
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
	if _, readErr := os.ReadFile(got); readErr == nil {
		t.Fatalf("expected not-found on read")
	}
}

func TestValidateRejectsEscapingFileSource(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &Config{
		DataDir: dataDir,
		Clients: []ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []RuleConfig{{
			ID: "r", Name: "r",
			Sources: []SourceConfig{{File: "/etc/passwd"}},
			Outputs: []string{"surge"},
		}},
	}
	cfg.Defaults()
	errs := cfg.Validate()
	if !containsErrorPath(errs, "rules[0].sources[0].file") {
		t.Fatalf("expected escape error, got: %v", ConfigErrors(errs))
	}
}

func TestValidateAcceptsFileSourceUnderDataDirLocal(t *testing.T) {
	dataDir := t.TempDir()
	for _, file := range []string{
		filepath.Join(dataDir, "local", "rules.list"), // absolute
		"rules.list",       // bare relative, anchored at data_dir/local
		"nested/deep.list", // nested relative
	} {
		t.Run(file, func(t *testing.T) {
			cfg := &Config{
				DataDir: dataDir,
				Clients: []ClientConfig{{ID: "surge", Template: "surge"}},
				Rules: []RuleConfig{{
					ID: "r", Name: "r",
					Sources: []SourceConfig{{File: file}},
					Outputs: []string{"surge"},
				}},
			}
			cfg.Defaults()
			errs := cfg.Validate()
			for _, e := range errs {
				if strings.Contains(e.Path, "sources[0].file") {
					t.Fatalf("expected no file-source error for %q, got: %v", file, e)
				}
			}
		})
	}
}
