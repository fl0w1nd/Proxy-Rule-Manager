package transformer

import "strings"

// SampleLineDiff produces dropped + modified samples for a step whose
// before/after content is known. It is used by transforms that don't
// natively emit per-line diagnostics (the regex replace / remove_lines
// pair, and user JS scripts) so the preview panel can still show
// "what changed".
//
// Algorithm: walk both sides line-by-line, classifying each input line.
//
//   - If the line is present (verbatim) in the output's bag of lines, it
//     passed through unchanged.
//   - If the line vanished entirely, it's a Dropped sample.
//   - Otherwise we try to pair it with the *next* unmatched output line;
//     if such a pairing exists we record it as a Modified sample.
//
// This is a heuristic (not a Myers diff) — it's tuned for the common case
// of "regex rewrites a substring on the same line", not for cross-line
// reordering. The cap (MaxReportSamples + MaxSampleBytes per sample) keeps
// the cost bounded even on multi-MB rule lists.
//
// dropReason / modifyReason are surfaced verbatim on each sample. Pass an
// empty modifyReason to skip the modified track entirely (use cases like
// `remove_lines` that can only drop).
func SampleLineDiff(before, after, dropReason, modifyReason string) (dropped []DroppedLine, droppedTotal int, modified []ModifiedLine, modifiedTotal int) {
	if before == after {
		return nil, 0, nil, 0
	}
	beforeLines := splitForDiff(before)
	afterLines := splitForDiff(after)

	// Output multiset for "did this line survive?" lookups. We use a
	// counter map because identical rule lines can repeat (e.g. multiple
	// "DOMAIN-SUFFIX,…" entries with the same suffix from different
	// sources). Removing one occurrence per match keeps subsequent lookups
	// honest.
	afterCounts := make(map[string]int, len(afterLines))
	for _, l := range afterLines {
		afterCounts[l]++
	}

	// `unmatchedAfter` walks the output in order. Whenever we find an
	// input line that's gone, we try to pair it with the first as-yet
	// unmatched output line — that pairing becomes a Modified sample.
	unmatchedAfter := make([]int, 0, len(afterLines))
	consumed := make([]bool, len(afterLines))
	for ai := range afterLines {
		unmatchedAfter = append(unmatchedAfter, ai)
	}

	for _, line := range beforeLines {
		if afterCounts[line] > 0 {
			afterCounts[line]--
			// Advance unmatchedAfter past matching outputs so later
			// modify-pairs only see truly unmatched candidates.
			for len(unmatchedAfter) > 0 {
				idx := unmatchedAfter[0]
				if consumed[idx] {
					unmatchedAfter = unmatchedAfter[1:]
					continue
				}
				if afterLines[idx] == line {
					consumed[idx] = true
					unmatchedAfter = unmatchedAfter[1:]
					break
				}
				// Output head is not this matched line: leave it for a
				// future modify-pair attempt.
				break
			}
			continue
		}
		// Line disappeared. Try to pair it with the next unmatched
		// output line for a modify sample; otherwise it's a drop.
		paired := false
		if modifyReason != "" {
			for len(unmatchedAfter) > 0 {
				idx := unmatchedAfter[0]
				if consumed[idx] {
					unmatchedAfter = unmatchedAfter[1:]
					continue
				}
				modified = AppendModified(modified, &modifiedTotal, ModifiedLine{
					From:   line,
					To:     afterLines[idx],
					Reason: modifyReason,
				})
				consumed[idx] = true
				unmatchedAfter = unmatchedAfter[1:]
				paired = true
				break
			}
		}
		if !paired {
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				Text:   line,
				Reason: dropReason,
			})
		}
	}
	return dropped, droppedTotal, modified, modifiedTotal
}

// splitForDiff returns the line list with terminal-LF tolerance: "a\nb\n"
// and "a\nb" both yield ["a", "b"].
func splitForDiff(content string) []string {
	if content == "" {
		return nil
	}
	normalized := normalizeLineEndings(content)
	parts := strings.Split(normalized, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
