package util

import "strings"

// LineChange is one line in a text diff from an older document to a newer one.
type LineChange struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// LineDiff is a line-level delta. Kind "del" is present only in the old
// document; "add" is present only in the new document.
type LineDiff struct {
	Added   int          `json:"added"`
	Removed int          `json:"removed"`
	Lines   []LineChange `json:"lines"`
}

// DiffLines reports how to turn old into new using matching lines as the
// unchanged set. Duplicate lines are counted with multiplicity.
func DiffLines(old, new string) LineDiff {
	from := splitLines(old)
	to := splitLines(new)
	remainNew := lineCounts(to)
	remainOld := lineCounts(from)
	diff := LineDiff{Lines: []LineChange{}}
	for _, line := range from {
		if remainNew[line] > 0 {
			remainNew[line]--
			continue
		}
		diff.Lines = append(diff.Lines, LineChange{Kind: "del", Text: line})
		diff.Removed++
	}
	for _, line := range to {
		if remainOld[line] > 0 {
			remainOld[line]--
			continue
		}
		diff.Lines = append(diff.Lines, LineChange{Kind: "add", Text: line})
		diff.Added++
	}
	return diff
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.Split(s, "\n")
}

func lineCounts(lines []string) map[string]int {
	counts := make(map[string]int, len(lines))
	for _, line := range lines {
		counts[line]++
	}
	return counts
}
