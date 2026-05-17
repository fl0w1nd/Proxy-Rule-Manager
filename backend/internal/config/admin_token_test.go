package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withCleanEnv isolates a test from inherited environment variables that
// would otherwise dictate which resolution branch fires.
func withCleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("ALLOW_EMPTY_ADMIN_TOKEN", "")
}

func TestResolveAdminToken_FromEnv(t *testing.T) {
	cfg := &Config{DataDir: t.TempDir()}
	t.Setenv("ADMIN_TOKEN", "  env-supplied-token  ")
	t.Setenv("ALLOW_EMPTY_ADMIN_TOKEN", "1") // explicit override must lose to env

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Source != AdminTokenFromEnv {
		t.Fatalf("source: got %v want FromEnv", res.Source)
	}
	if res.Token != "env-supplied-token" {
		t.Fatalf("token: got %q", res.Token)
	}
	// No file should have been written when env wins.
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "admin-token")); !os.IsNotExist(err) {
		t.Fatalf("env path must not persist a token file (err=%v)", err)
	}
}

func TestResolveAdminToken_FromFile(t *testing.T) {
	withCleanEnv(t)
	cfg := &Config{DataDir: t.TempDir()}
	path := filepath.Join(cfg.DataDir, "admin-token")
	if err := os.WriteFile(path, []byte("disk-token\n"), 0o600); err != nil {
		t.Fatalf("seed token file: %v", err)
	}

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Source != AdminTokenFromFile {
		t.Fatalf("source: got %v want FromFile", res.Source)
	}
	if res.Token != "disk-token" {
		t.Fatalf("token: got %q", res.Token)
	}
	if res.FilePath != path {
		t.Fatalf("file path: got %q", res.FilePath)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings for 0600 file, got %v", res.Warnings)
	}
}

func TestResolveAdminToken_FromFile_WarnsOnLoosePerms(t *testing.T) {
	withCleanEnv(t)
	cfg := &Config{DataDir: t.TempDir()}
	path := filepath.Join(cfg.DataDir, "admin-token")
	if err := os.WriteFile(path, []byte("disk-token"), 0o644); err != nil {
		t.Fatalf("seed token file: %v", err)
	}

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Source != AdminTokenFromFile {
		t.Fatalf("source: got %v want FromFile", res.Source)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected a warning for permissive mode")
	}
}

func TestResolveAdminToken_AllowEmpty(t *testing.T) {
	withCleanEnv(t)
	cfg := &Config{DataDir: t.TempDir()}
	t.Setenv("ALLOW_EMPTY_ADMIN_TOKEN", "1")

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Source != AdminTokenAllowedEmpty {
		t.Fatalf("source: got %v want AllowedEmpty", res.Source)
	}
	if res.Token != "" {
		t.Fatalf("token must be empty in fail-open mode, got %q", res.Token)
	}
	// Must not generate a token file in fail-open mode.
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "admin-token")); !os.IsNotExist(err) {
		t.Fatalf("fail-open mode must not persist a token (err=%v)", err)
	}
}

func TestResolveAdminToken_Generated(t *testing.T) {
	withCleanEnv(t)
	cfg := &Config{DataDir: t.TempDir()}

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Source != AdminTokenGenerated {
		t.Fatalf("source: got %v want Generated", res.Source)
	}
	if len(res.Token) < 32 {
		t.Fatalf("generated token too short: %d chars", len(res.Token))
	}
	if strings.ContainsAny(res.Token, " \t\n+/=") {
		t.Fatalf("generated token contains unsafe chars: %q", res.Token)
	}
	if res.FilePath != filepath.Join(cfg.DataDir, "admin-token") {
		t.Fatalf("file path: got %q", res.FilePath)
	}
	info, err := os.Stat(res.FilePath)
	if err != nil {
		t.Fatalf("token file not created: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("generated token file perm: got %o want 0600", mode)
	}
	persisted, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.TrimSpace(string(persisted)) != res.Token {
		t.Fatalf("persisted token differs from returned token")
	}

	// A second call must reuse the freshly persisted file (FromFile path)
	// rather than generating a different token.
	res2, err := ResolveAdminToken(cfg)
	if err != nil {
		t.Fatalf("resolve (2nd): %v", err)
	}
	if res2.Source != AdminTokenFromFile {
		t.Fatalf("second call source: got %v want FromFile", res2.Source)
	}
	if res2.Token != res.Token {
		t.Fatalf("token changed across calls: %q vs %q", res.Token, res2.Token)
	}
}

func TestResolveAdminToken_EmptyFileFallsThrough(t *testing.T) {
	withCleanEnv(t)
	cfg := &Config{DataDir: t.TempDir()}
	path := filepath.Join(cfg.DataDir, "admin-token")
	// A whitespace-only file must be treated as missing, otherwise an
	// operator who accidentally truncated it would get the fail-open
	// behaviour without realising.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := ResolveAdminToken(cfg)
	if err != nil {
		// The generate step will hit O_EXCL on the existing file.
		// That's a legitimate error; surface it instead of silently
		// overwriting the operator's (possibly mis-edited) file.
		if !strings.Contains(err.Error(), "exist") {
			t.Fatalf("resolve: %v", err)
		}
		return
	}
	if res.Source == AdminTokenFromFile {
		t.Fatalf("empty file must not be treated as a valid token")
	}
}
