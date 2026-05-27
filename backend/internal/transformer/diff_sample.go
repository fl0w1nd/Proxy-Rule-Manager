package transformer

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// SampleLineDiff produces dropped + modified + added samples for a step
// whose before/after content is known. It is used by transforms that
// don't natively emit per-line diagnostics (the regex replace /
// remove_lines pair and user JS scripts) so the preview panel can still
// show "what changed".
//
// Algorithm: compute the LCS-based opcodes of beforeLines vs afterLines
// via difflib.SequenceMatcher and classify each opcode:
//
//   - 'e' (equal):    no-op, the lines passed through unchanged.
//   - 'd' (delete):   every line in the slice is a Dropped sample.
//   - 'i' (insert):   every line in the slice is an Added sample (only
//     when addReason != ""; an empty reason disables the
//     added track, e.g. for `remove_lines` which can only
//     ever drop).
//   - 'r' (replace):  when both sides have the same length and
//     modifyReason != "", lines are paired 1:1 as
//     Modified samples. Otherwise the slice is split
//     into Dropped + Added so we never invent a fake
//     pairing between two semantically unrelated lines.
//
// Earlier revisions paired any vanished line with the *next* unmatched
// output line as a Modified sample. That heuristic produced visually
// nonsensical pairs ("DOMAIN,aws-na.intlgame.com → DOMAIN,www.jupiter-
// launcher.com" — the right side is just whatever happened to be next)
// for transformers that only ever drop lines, which is the common case.
//
// dropReason / modifyReason / addReason are surfaced verbatim on each
// sample. Pass an empty modifyReason / addReason to disable the
// corresponding track entirely. dropReason is always honoured because
// every transform we care about can produce drops.
func SampleLineDiff(before, after, dropReason, modifyReason, addReason string) (
	dropped []DroppedLine,
	droppedTotal int,
	modified []ModifiedLine,
	modifiedTotal int,
	added []AddedLine,
	addedTotal int,
) {
	if before == after {
		return nil, 0, nil, 0, nil, 0
	}
	beforeLines := splitForDiff(before)
	afterLines := splitForDiff(after)

	sm := difflib.NewMatcher(beforeLines, afterLines)
	for _, op := range sm.GetOpCodes() {
		switch op.Tag {
		case 'e':
			// Equal blocks are skipped — they contributed no diagnostics.
		case 'd':
			for k := op.I1; k < op.I2; k++ {
				dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
					LineNo: k + 1,
					Text:   beforeLines[k],
					Reason: dropReason,
				})
			}
		case 'i':
			if addReason == "" {
				continue
			}
			for k := op.J1; k < op.J2; k++ {
				added = AppendAdded(added, &addedTotal, AddedLine{
					LineNo: k + 1,
					Text:   afterLines[k],
					Reason: addReason,
				})
			}
		case 'r':
			bLen := op.I2 - op.I1
			aLen := op.J2 - op.J1
			// Pair as Modified only when the block is a clean 1:1 swap
			// AND the caller opted into modify tracking. This keeps the
			// pairing positionally honest: a regex rewrite that turns
			// one line into one line at the same index is exactly what
			// "modified" should mean. Anything else (different lengths,
			// or modifyReason="") falls back to split drop + add so the
			// UI never invents a misleading rewrite arrow.
			if modifyReason != "" && bLen == aLen {
				for k := 0; k < bLen; k++ {
					modified = AppendModified(modified, &modifiedTotal, ModifiedLine{
						LineNo: op.I1 + k + 1,
						From:   beforeLines[op.I1+k],
						To:     afterLines[op.J1+k],
						Reason: modifyReason,
					})
				}
				continue
			}
			for k := op.I1; k < op.I2; k++ {
				dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
					LineNo: k + 1,
					Text:   beforeLines[k],
					Reason: dropReason,
				})
			}
			if addReason != "" {
				for k := op.J1; k < op.J2; k++ {
					added = AppendAdded(added, &addedTotal, AddedLine{
						LineNo: k + 1,
						Text:   afterLines[k],
						Reason: addReason,
					})
				}
			}
		}
	}
	return dropped, droppedTotal, modified, modifiedTotal, added, addedTotal
}

// splitForDiff returns the line list with terminal-LF tolerance: "a\nb\n"
// and "a\nb" both yield ["a", "b"]. We deliberately do NOT trim
// leading/trailing whitespace on each line so the sample displayed in
// the UI matches what the operator sees in the source content.
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
