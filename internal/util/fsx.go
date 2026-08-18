package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes content to filePath via a temporary file + rename.
// It creates parent directories as needed.
func AtomicWriteFile(filePath string, content []byte) error {
	return AtomicWriteFileChecked(filePath, content, nil)
}

// AtomicWriteFileChecked writes content atomically after running check with
// the completed temporary file ready for rename.
func AtomicWriteFileChecked(filePath string, content []byte, check func() error) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	base := filepath.Base(filePath)
	temp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", filePath, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp %s: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", tempPath, err)
	}
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("rename %s: %w", filePath, err)
	}
	cleanup = false
	return nil
}

// AtomicWriteStream writes src to filePath atomically without buffering
// the entire payload in memory.
func AtomicWriteStream(filePath string, src io.Reader) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	base := filepath.Base(filePath)
	temp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", filePath, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := io.Copy(temp, src); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp %s: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", tempPath, err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("rename %s: %w", filePath, err)
	}
	cleanup = false
	return nil
}

// EnsureDir ensures the directory exists.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// FileExists reports whether the path exists (file or directory).
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
