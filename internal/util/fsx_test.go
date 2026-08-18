package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWritersCreateAndReplaceFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := AtomicWriteFile(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteStream(path, strings.NewReader("second")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "second" {
		t.Fatalf("content = %q, want second", content)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state.json.*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err = %v", matches, err)
	}
}

func TestAtomicWriteStreamCleansUpAfterReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	reader := &failingReader{err: errors.New("read failed")}
	if err := AtomicWriteStream(path, reader); err == nil {
		t.Fatal("write succeeded after reader failure")
	}
	if FileExists(path) {
		t.Fatal("destination exists after failed write")
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state.json.*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err = %v", matches, err)
	}
}

func TestAtomicWriteFileCheckedPreservesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := errors.New("source changed")
	if err := AtomicWriteFileChecked(path, []byte("candidate"), func() error { return checkErr }); !errors.Is(err, checkErr) {
		t.Fatalf("write error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "external" {
		t.Fatalf("content = %q", content)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config.yaml.*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err = %v", matches, err)
	}
}

type failingReader struct {
	err error
}

func (r *failingReader) Read([]byte) (int, error) {
	return 0, r.err
}
