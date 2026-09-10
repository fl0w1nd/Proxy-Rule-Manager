package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileLockExcludesSecondAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".state", "prm.lock")
	first, err := AcquireFileLock(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = first.Release() }()

	second, err := AcquireFileLock(path)
	if second != nil {
		_ = second.Release()
		t.Fatal("second acquire unexpectedly succeeded")
	}
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("second acquire error = %v, want LockedError", err)
	}
	if runtime.GOOS != "windows" && !strings.Contains(locked.Holder, "pid") {
		t.Fatalf("holder = %q, want a recorded holder", locked.Holder)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	third, err := AcquireFileLock(path)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatalf("release third: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file removed: %v", err)
	}
}
