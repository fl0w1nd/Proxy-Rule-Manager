package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AdminTokenSource describes where the active admin token came from.
type AdminTokenSource int

const (
	// AdminTokenFromEnv means ADMIN_TOKEN was set explicitly.
	AdminTokenFromEnv AdminTokenSource = iota
	// AdminTokenFromFile means we reused a previously persisted token file.
	AdminTokenFromFile
	// AdminTokenGenerated means we created a new token and persisted it.
	AdminTokenGenerated
	// AdminTokenAllowedEmpty means the operator explicitly opted into the
	// fully-open mode via ALLOW_EMPTY_ADMIN_TOKEN=1.
	AdminTokenAllowedEmpty
)

// AdminTokenResult captures the outcome of ResolveAdminToken so the boot
// banner in main can show the right message without rederiving state.
type AdminTokenResult struct {
	// Token is the final admin token to install on the server. Empty only
	// when Source == AdminTokenAllowedEmpty.
	Token string
	// Source records how the token was obtained.
	Source AdminTokenSource
	// FilePath is the on-disk path of the token file when relevant
	// (Source is FromFile or Generated).
	FilePath string
	// Warnings collects non-fatal issues worth surfacing in the logs
	// (e.g. file permissions wider than 0600).
	Warnings []string
}

// adminTokenFilename is the basename of the persisted token file inside
// the data dir.
const adminTokenFilename = "admin-token"

// ResolveAdminToken decides which admin token to use, mirroring the
// precedence documented in the security plan:
//
//  1. ADMIN_TOKEN env var (highest priority — never persisted)
//  2. <DATA_DIR>/admin-token file (if non-empty)
//  3. ALLOW_EMPTY_ADMIN_TOKEN=1 → run with no token (fully open mode)
//  4. otherwise generate a random 32-byte URL-safe token and persist it
//
// The function never panics; failures to read/write the token file in the
// "generate" path are returned as errors so main can decide what to do
// (typically fall back to the in-memory token rather than refuse to boot).
func ResolveAdminToken(cfg *Config) (AdminTokenResult, error) {
	if cfg == nil {
		return AdminTokenResult{}, errors.New("config: nil cfg")
	}
	if env := strings.TrimSpace(os.Getenv("ADMIN_TOKEN")); env != "" {
		return AdminTokenResult{Token: env, Source: AdminTokenFromEnv}, nil
	}

	path := filepath.Join(cfg.DataDir, adminTokenFilename)

	// File on disk takes precedence over generation so restarts keep the
	// same token without operator intervention.
	if tok, warnings, ok, err := readAdminTokenFile(path); err != nil {
		return AdminTokenResult{}, err
	} else if ok {
		return AdminTokenResult{
			Token:    tok,
			Source:   AdminTokenFromFile,
			FilePath: path,
			Warnings: warnings,
		}, nil
	}

	// Explicit opt-in to fail-open mode. Distinct from a missing env var so
	// we never silently fall back to "no auth".
	if isTruthy(os.Getenv("ALLOW_EMPTY_ADMIN_TOKEN")) {
		return AdminTokenResult{Source: AdminTokenAllowedEmpty}, nil
	}

	// Generate and persist a fresh token. We attempt to mkdir DataDir
	// first because main may run before any other component has touched
	// the filesystem.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return AdminTokenResult{}, fmt.Errorf("config: ensure data dir: %w", err)
	}
	tok, err := generateAdminToken()
	if err != nil {
		return AdminTokenResult{}, fmt.Errorf("config: generate admin token: %w", err)
	}
	if err := writeAdminTokenFile(path, tok); err != nil {
		return AdminTokenResult{}, fmt.Errorf("config: persist admin token: %w", err)
	}
	return AdminTokenResult{
		Token:    tok,
		Source:   AdminTokenGenerated,
		FilePath: path,
	}, nil
}

// readAdminTokenFile reads and trims the persisted token. Returns ok=false
// when the file is missing or empty so callers can fall through to the
// next resolution step.
func readAdminTokenFile(path string) (string, []string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, false, nil
		}
		return "", nil, false, fmt.Errorf("config: stat admin token file: %w", err)
	}
	if info.IsDir() {
		return "", nil, false, fmt.Errorf("config: admin token path is a directory: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, false, fmt.Errorf("config: read admin token file: %w", err)
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", nil, false, nil
	}
	var warnings []string
	// On POSIX, the mode contains world/group bits we can sanity check.
	// We only warn — refusing to start would surprise operators who
	// intentionally relaxed perms (e.g. shared compose volumes).
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		warnings = append(warnings, fmt.Sprintf(
			"admin token file %s has permissive mode %o; recommend chmod 600", path, mode))
	}
	return tok, warnings, true, nil
}

// writeAdminTokenFile writes the token with 0600 perms via O_CREATE|O_EXCL
// so we never clobber an existing file by accident.
func writeAdminTokenFile(path, token string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(token + "\n"); err != nil {
		return err
	}
	return f.Sync()
}

// generateAdminToken returns 32 random bytes encoded as URL-safe base64
// without padding (43 chars).
func generateAdminToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// isTruthy maps the usual "yes/on/true/1" shortcuts to bool.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
