package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes content to filePath via a temporary file + rename.
// It creates parent directories as needed and falls back to copy/unlink when
// rename across mount points fails.
func AtomicWriteFile(filePath string, content []byte) error {
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
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(filePath)
		if rerr := os.Rename(tempPath, filePath); rerr != nil {
			return fmt.Errorf("rename %s: %w", filePath, rerr)
		}
	}
	cleanup = false
	return nil
}

// WriteTempFile writes content to a temp file alongside finalPath and returns
// the temp path. Caller is responsible for either renaming it onto finalPath
// (preferably via os.Rename) or removing it. This split lets callers commit
// other side effects (e.g. a DB row) in between writing and renaming, while
// still benefiting from same-filesystem atomic rename semantics.
func WriteTempFile(finalPath string, content []byte) (string, error) {
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	base := filepath.Base(finalPath)
	temp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp for %s: %w", finalPath, err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write temp %s: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close temp %s: %w", tempPath, err)
	}
	return tempPath, nil
}

// CommitTempFile atomically replaces finalPath with tempPath, with a single
// fallback when rename across an existing target fails on certain filesystems.
// On error the tempPath is left in place so the caller can inspect/cleanup.
func CommitTempFile(tempPath, finalPath string) error {
	if err := os.Rename(tempPath, finalPath); err != nil {
		// Some filesystems require the target to be removed first.
		_ = os.Remove(finalPath)
		if rerr := os.Rename(tempPath, finalPath); rerr != nil {
			return fmt.Errorf("rename %s -> %s: %w", tempPath, finalPath, rerr)
		}
	}
	return nil
}

// AtomicWriteStream writes src to filePath atomically without buffering the
// entire payload in memory. Mirrors AtomicWriteFile but takes an io.Reader,
// so callers handling user uploads can pipe straight from multipart files
// instead of going through []byte.
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
		_ = os.Remove(filePath)
		if rerr := os.Rename(tempPath, filePath); rerr != nil {
			return fmt.Errorf("rename %s: %w", filePath, rerr)
		}
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
