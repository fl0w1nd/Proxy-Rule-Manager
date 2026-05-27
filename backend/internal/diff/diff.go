// Package diff produces unified diffs compatible with the existing
// src/lib/diff.ts implementation (jsdiff createTwoFilesPatch / context=3).
package diff

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	largeLineThreshold = 500
	largeCharThreshold = 50000
)

// ChangeType matches the TS ChangeType union.
type ChangeType string

const (
	Created ChangeType = "created"
	Updated ChangeType = "updated"
	Deleted ChangeType = "deleted"
)

// indexSep is the separator line emitted by jsdiff after "Index: <name>".
const indexSep = "==================================================================="

// CreateLineDiff produces a unified diff with the given context size.
// Output mirrors jsdiff's createTwoFilesPatch including the "Index:" header
// so the frontend diff-viewer receives the same format from both backends.
//
// Both `before` and `after` are normalised to end with a trailing "\n" before
// diffing to work around a long-standing pmezard/go-difflib quirk: when the
// final line has no LF terminator, the library emits the next diff line
// directly after it without a separator, producing visually broken output
// such as "-foo+bar\n+\n". A semantically equal "missing-vs-present trailing
// newline" pair therefore renders as a no-op, which matches how every other
// rule consumer (cat, gh diff, jsdiff) treats trailing-LF normalisation.
func CreateLineDiff(before, after string, contextLines int) string {
	if contextLines <= 0 {
		contextLines = 3
	}
	before = ensureTrailingNewline(before)
	after = ensureTrailingNewline(after)
	udiff := difflib.UnifiedDiff{
		A:        strings.SplitAfter(before, "\n"),
		B:        strings.SplitAfter(after, "\n"),
		FromFile: "before",
		ToFile:   "after",
		Context:  contextLines,
	}
	out, err := difflib.GetUnifiedDiffString(udiff)
	if err != nil {
		return ""
	}
	if out == "" {
		return out
	}
	return "Index: before\n" + indexSep + "\n" + out
}

// ensureTrailingNewline appends "\n" when content is non-empty and lacks a
// terminal LF. Empty strings are returned untouched so the "created" /
// "deleted" diff shapes (one side empty) keep their natural form.
func ensureTrailingNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// CreateActivityDiff prefers compact representations for huge created/deleted payloads.
func CreateActivityDiff(changeType ChangeType, before, after string, contextLines int) string {
	if changeType == Updated {
		return CreateLineDiff(before, after, contextLines)
	}
	target := after
	if changeType == Deleted {
		target = before
	}
	if shouldSummarize(target) {
		return largeSummary(changeType, target)
	}
	return CreateLineDiff(before, after, contextLines)
}

func shouldSummarize(content string) bool {
	// Use rune count (not byte count) to match the TS content.length semantics
	// which counts UTF-16 code units (≈ runes for BMP characters).
	return countLines(content) > largeLineThreshold || utf8.RuneCountInString(content) > largeCharThreshold
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func largeSummary(kind ChangeType, content string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s summary\n", kind))
	sb.WriteString("# full diff omitted for large payload\n")
	sb.WriteString(fmt.Sprintf("# lines: %d\n", countLines(content)))
	sb.WriteString(fmt.Sprintf("# bytes: %d", len(content)))
	return sb.String()
}
