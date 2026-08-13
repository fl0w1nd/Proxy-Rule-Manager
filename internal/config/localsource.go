package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// LocalFileResolver validates that a local file source path is contained
// within the runtime data directory's local subdirectory (dataDir/local), resolving
// symlinks to prevent escape. It returns an absolute, canonical path that is
// safe to read, or an error describing why the path was rejected.
type LocalFileResolver func(file string) (string, error)

// NewLocalFileResolver builds a resolver confined to dataDir/local. The root is
// derived from the runtime data directory and canonicalized with filepath.EvalSymlinks so
// symlinked directories (e.g. /var -> /private/var on macOS) and symlinked
// files that point outside the root are handled correctly. The confinement is
// fixed and not configurable: local file sources must live under dataDir/local,
// so no config field can widen the set of host paths a rule may read.
//
// Relative file paths are anchored at dataDir/local, so a source declared as
// file: "custom.list" reads dataDir/local/custom.list. Absolute paths are
// accepted only when they resolve under the same root.
func NewLocalFileResolver(dataDir string) LocalFileResolver {
	root := canonicalizePath(absPath(filepath.Join(dataDir, "local")))
	return func(file string) (string, error) {
		if file == "" {
			return "", fmt.Errorf("local file source path is empty")
		}
		var base string
		if filepath.IsAbs(file) {
			base = filepath.Clean(file)
		} else {
			base = filepath.Join(root, file)
		}
		canon := canonicalizePath(base)
		if !isUnderRoot(canon, root) {
			return "", fmt.Errorf("local file source %q is outside %s", file, root)
		}
		return canon, nil
	}
}

// absPath returns an absolute, cleaned form of p. Relative paths are resolved
// against the process working directory, matching how the runtime data directory is interpreted
// elsewhere (artifact paths, etc.).
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}

// canonicalizePath returns p with all existing components resolved through
// filepath.EvalSymlinks. When p itself exists, this is just EvalSymlinks. When
// p does not exist yet (e.g. a file source that will be created later, or a
// missing file), the longest existing ancestor is resolved and the
// non-existent tail is re-joined, so symlinked ancestor directories (such as
// /var on macOS) are still normalized.
func canonicalizePath(p string) string {
	p = filepath.Clean(p)
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(real)
	}
	dir := p
	tail := ""
	for {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			if tail == "" {
				return filepath.Clean(real)
			}
			return filepath.Clean(filepath.Join(real, tail))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached a non-existent root; nothing more to resolve.
			return p
		}
		if tail == "" {
			tail = filepath.Base(dir)
		} else {
			tail = filepath.Join(filepath.Base(dir), tail)
		}
		dir = parent
	}
}

// isUnderRoot reports whether path is the root itself or lives beneath it.
func isUnderRoot(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
