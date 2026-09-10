package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// errLockHeld is returned by openLockFile when another process holds the lock.
var errLockHeld = errors.New("lock is held by another process")

// LockedError reports that a lock is already held by another process.
type LockedError struct {
	Holder string // best-effort holder description; empty when unreadable
}

func (e *LockedError) Error() string {
	if e.Holder == "" {
		return "already locked by another process"
	}
	return "already locked by " + e.Holder
}

// FileLock is an exclusive advisory lock on one lock file. It is held until
// Release or process exit and prevents two processes from writing the same
// data directory.
type FileLock struct {
	file *os.File
}

// AcquireFileLock takes an exclusive lock on path, creating the lock file and
// its parent directory when needed. It fails immediately when another process
// holds the lock.
func AcquireFileLock(path string) (*FileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := openLockFile(path)
	if err != nil {
		if errors.Is(err, errLockHeld) {
			return nil, &LockedError{Holder: readLockHolder(path)}
		}
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := writeLockHolder(file); err != nil {
		_ = closeLockFile(file)
		return nil, err
	}
	return &FileLock{file: file}, nil
}

// Release drops the lock. The lock file stays on disk for the next holder;
// releasing twice is a no-op.
func (l *FileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	return closeLockFile(file)
}

// writeLockHolder records the holder so a blocked process can name it.
func writeLockHolder(file *os.File) error {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown host"
	}
	holder := fmt.Sprintf("pid %d on %s since %s", os.Getpid(), host, time.Now().UTC().Format(time.RFC3339))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek lock file: %w", err)
	}
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate lock file: %w", err)
	}
	if _, err := file.WriteString(holder + "\n"); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}
	return nil
}

// readLockHolder returns the recorded holder, or "" when it cannot be read
// (including platforms that keep the lock file unreadable while locked).
func readLockHolder(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
