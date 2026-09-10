package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

func TestBuildAppRefusesLockedDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	lock, err := util.AcquireFileLock(filepath.Join(dataDir, lockFileRelPath))
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	if _, err := buildApp(dataDir); err == nil || !strings.Contains(err.Error(), "is in use") {
		t.Fatalf("buildApp error = %v, want a data directory in-use error", err)
	}
}

func TestBuildAppReleasesLockOnFailure(t *testing.T) {
	dir := t.TempDir()
	original := cfgFile
	cfgFile = filepath.Join(dir, "missing.yaml")
	t.Cleanup(func() { cfgFile = original })
	dataDir := filepath.Join(dir, "data")

	if _, err := buildApp(dataDir); err == nil {
		t.Fatal("buildApp with a missing config file succeeded")
	}
	lock, err := util.AcquireFileLock(filepath.Join(dataDir, lockFileRelPath))
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	_ = lock.Release()
}
