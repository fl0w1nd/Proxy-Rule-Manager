package util

import (
	"path/filepath"
	"testing"
)

func TestJoinInsideEnforcesBaseDirectory(t *testing.T) {
	base := t.TempDir()
	got, err := JoinInside(base, "rules", "apple.list")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "rules", "apple.list")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}

	if _, err := JoinInside(base, "..", "outside"); err == nil {
		t.Fatal("parent traversal was accepted")
	}
}

func TestEnsureSafeSegmentRejectsPathSyntax(t *testing.T) {
	for _, segment := range []string{"", ".", "..", "nested/file", `nested\file`, filepath.Join(string(filepath.Separator), "absolute")} {
		t.Run(segment, func(t *testing.T) {
			if err := EnsureSafeSegment(segment, "segment"); err == nil {
				t.Fatalf("segment %q was accepted", segment)
			}
		})
	}
	if err := EnsureSafeSegment("apple.list", "segment"); err != nil {
		t.Fatalf("safe segment rejected: %v", err)
	}
}
