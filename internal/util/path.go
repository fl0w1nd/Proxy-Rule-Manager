package util

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EnsureSafeSegment validates that segment cannot escape its parent directory.
func EnsureSafeSegment(segment, label string) error {
	if segment == "" || segment == "." || segment == ".." {
		return fmt.Errorf("invalid %s: %q", label, segment)
	}
	if filepath.IsAbs(segment) || strings.Contains(segment, "/") || strings.Contains(segment, "\\") {
		return fmt.Errorf("invalid %s: %q", label, segment)
	}
	return nil
}

// JoinInside joins parts and ensures the result stays under baseDir.
func JoinInside(baseDir string, parts ...string) (string, error) {
	full := filepath.Join(append([]string{baseDir}, parts...)...)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absFull+string(filepath.Separator), absBase+string(filepath.Separator)) &&
		absFull != absBase {
		return "", fmt.Errorf("path escapes base: %s", full)
	}
	return full, nil
}
