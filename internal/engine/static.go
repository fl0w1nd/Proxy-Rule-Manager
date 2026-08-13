package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// ExportStatic assembles a standalone static site from the current published
// rules and icons, then swaps it into outputDir as one complete directory.
func (e *UpdateEngine) ExportStatic(outputDir string) error {
	output, err := validateStaticOutputPath(outputDir, e.Config.DataDir)
	if err != nil {
		return err
	}
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create static output parent: %w", err)
	}

	base := filepath.Base(output)
	staging, err := os.MkdirTemp(parent, "."+base+".staging-*")
	if err != nil {
		return fmt.Errorf("create static staging directory: %w", err)
	}
	stagingExists := true
	defer func() {
		if stagingExists {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := copyStaticTree(filepath.Join(e.Config.DataDir, "rules"), filepath.Join(staging, "rules")); err != nil {
		return fmt.Errorf("copy rules: %w", err)
	}
	stagingStatic := filepath.Join(staging, site.StaticDir)
	if err := copyStaticTree(
		filepath.Join(e.Config.DataDir, site.StaticDir, "icons"),
		filepath.Join(stagingStatic, "icons"),
	); err != nil {
		return fmt.Errorf("copy icons: %w", err)
	}
	if err := util.AtomicWriteFile(filepath.Join(staging, ".nojekyll"), nil); err != nil {
		return fmt.Errorf("write .nojekyll: %w", err)
	}

	updatedAt := time.Now()
	if checkedAt, ok := e.State.LastCheck(); ok {
		updatedAt = checkedAt
	}
	index := e.publicIndexData(updatedAt, stagingStatic, "", nil, nil)
	if err := site.WriteIndex(staging, stagingStatic, index); err != nil {
		return fmt.Errorf("write static index: %w", err)
	}

	if err := replaceStaticDirectory(staging, output); err != nil {
		return err
	}
	stagingExists = false
	return nil
}

func validateStaticOutputPath(outputDir, dataDir string) (string, error) {
	if outputDir == "" {
		return "", fmt.Errorf("static output directory is empty")
	}
	rawOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve static output: %w", err)
	}
	if info, statErr := os.Lstat(filepath.Clean(rawOutput)); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("static output is a symbolic link: %s", rawOutput)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect static output: %w", statErr)
	}
	output, err := canonicalPath(outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve static output: %w", err)
	}
	if filepath.Dir(output) == output {
		return "", fmt.Errorf("static output cannot be a filesystem root: %s", output)
	}
	workingDir, err := canonicalPath(".")
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	if output == workingDir {
		return "", fmt.Errorf("static output cannot replace the working directory: %s", output)
	}
	data, err := canonicalPath(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	if pathsOverlap(output, data) {
		return "", fmt.Errorf("static output and data directory overlap: %s and %s", output, data)
	}
	if info, err := os.Lstat(output); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("static output is not a directory: %s", output)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect static output: %w", err)
	}
	return output, nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	current := abs
	var tail []string
	for {
		resolved, evalErr := filepath.EvalSymlinks(current)
		if evalErr == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

func pathsOverlap(a, b string) bool {
	return pathWithin(a, b) || pathWithin(b, a)
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !filepath.IsAbs(rel) &&
		(rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func copyStaticTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is not publishable: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported publish file type: %s", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		writeErr := util.AtomicWriteStream(target, file)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	})
}

func replaceStaticDirectory(staging, output string) error {
	backup := staging + ".previous"
	hadOutput := false
	if _, err := os.Lstat(output); err == nil {
		hadOutput = true
		if err := os.Rename(output, backup); err != nil {
			return fmt.Errorf("move previous static output: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect previous static output: %w", err)
	}
	if err := os.Rename(staging, output); err != nil {
		if hadOutput {
			if rollbackErr := os.Rename(backup, output); rollbackErr != nil {
				return fmt.Errorf("publish static output: %v; restore previous output: %w", err, rollbackErr)
			}
		}
		return fmt.Errorf("publish static output: %w", err)
	}
	if hadOutput {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous static output: %w", err)
		}
	}
	return nil
}
