package util

import "testing"

func TestDiffLinesCountsChangedLines(t *testing.T) {
	diff := DiffLines("name: Base\noutputs: [surge]\n", "name: Updated\noutputs: [surge]\n")
	if diff.Added != 1 || diff.Removed != 1 || len(diff.Lines) != 2 {
		t.Fatalf("diff=%+v", diff)
	}
	if diff.Lines[0] != (LineChange{Kind: "del", Text: "name: Base"}) || diff.Lines[1] != (LineChange{Kind: "add", Text: "name: Updated"}) {
		t.Fatalf("lines=%+v", diff.Lines)
	}
	same := DiffLines("a\n", "a\n")
	if same.Added != 0 || same.Removed != 0 || len(same.Lines) != 0 {
		t.Fatalf("identical=%+v", same)
	}
}
