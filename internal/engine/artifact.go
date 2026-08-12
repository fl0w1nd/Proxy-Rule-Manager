package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

func rulesDir(dataDir string) string {
	return filepath.Join(dataDir, "rules")
}

func clientArtifactDir(dataDir, clientID string) (string, error) {
	if err := util.EnsureSafeSegment(clientID, "client id"); err != nil {
		return "", err
	}
	return util.JoinInside(rulesDir(dataDir), clientID)
}

// ArtifactPath resolves a managed artifact path under data/rules/{client}.
func ArtifactPath(dataDir, clientID, relativeName string) (string, error) {
	clientDir, err := clientArtifactDir(dataDir, clientID)
	if err != nil {
		return "", err
	}
	parts := strings.Split(filepath.ToSlash(relativeName), "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("empty artifact name")
	}
	for _, part := range parts {
		if err := util.EnsureSafeSegment(part, "artifact path segment"); err != nil {
			return "", err
		}
	}
	return util.JoinInside(clientDir, parts...)
}

// EnsureArtifactDirs creates the output directory structure under dataDir.
func EnsureArtifactDirs(dataDir string, clientIDs []string) error {
	if err := util.EnsureDir(rulesDir(dataDir)); err != nil {
		return err
	}
	for _, id := range clientIDs {
		dir, err := clientArtifactDir(dataDir, id)
		if err != nil {
			return err
		}
		if err := util.EnsureDir(dir); err != nil {
			return err
		}
	}
	return util.EnsureDir(filepath.Join(dataDir, ".state", "snapshots"))
}

// ListArtifacts returns managed artifact paths relative to a client directory.
func ListArtifacts(dataDir, clientID string) ([]string, error) {
	dir, err := clientArtifactDir(dataDir, clientID)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(files)
	return files, err
}

// CountArtifacts returns the current number of published files below data/rules.
func CountArtifacts(dataDir string) (int, error) {
	root := rulesDir(dataDir)
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return count, err
}

// ReconcileArtifacts removes files under data/rules that were not produced by
// the latest successful full update.
func ReconcileArtifacts(dataDir string, expected map[string]struct{}) error {
	root := rulesDir(dataDir)
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				dirs = append(dirs, path)
			}
			return nil
		}
		if _, ok := expected[filepath.Clean(path)]; ok {
			return nil
		}
		return os.Remove(path)
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dirs[i]); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// ReconcileRuleArtifacts removes unexpected top-level artifacts for selected
// rules while preserving geosite catalogs and artifacts for every other rule.
func ReconcileRuleArtifacts(dataDir string, ruleIDs map[string]struct{}, expected map[string]struct{}) error {
	root := rulesDir(dataDir)
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				dirs = append(dirs, path)
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 2 {
			return nil
		}
		ruleID := strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))
		if _, selected := ruleIDs[ruleID]; !selected {
			return nil
		}
		if _, keep := expected[filepath.Clean(path)]; keep {
			return nil
		}
		return os.Remove(path)
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dirs[i]); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}
