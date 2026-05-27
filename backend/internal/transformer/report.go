package transformer

// This file declares the diagnostic types surfaced exclusively by the
// preview pipeline (/api/preview). The sync pipeline never emits these
// records — see ApplyNewTransformsReported in pipeline.go for the only
// production call site, and PreviewRule in syncengine/engine.go for the
// aggregator that wraps each step into a TransformReport.

// StageRule / StageMerge / StageClient / StageOverride name the four
// logical phases of the per-client transform pipeline. They keep the JSON
// representation stable so the frontend timeline can colour-code stages
// without depending on the order of slice items.
const (
	StageRule     = "rule"
	StageMerge    = "merge"
	StageClient   = "client"
	StageOverride = "override"
)

// KindUse / KindUseBuiltin / KindReplace / KindRemoveLines / KindMerge name
// the operation kind for a step. KindUseBuiltin is distinct from KindUse so
// the UI can render the built-in lock icon and the dropped/modified
// breakdown that only built-ins emit.
const (
	KindUse         = "use"
	KindUseBuiltin  = "use_builtin"
	KindReplace     = "replace"
	KindRemoveLines = "remove_lines"
	KindMerge       = "merge"
)

// MaxReportSamples caps how many dropped or modified line samples a step
// keeps. The cap is enforced inside the reported pipeline and the built-in
// transformers so a runaway transform cannot OOM the preview response.
const MaxReportSamples = 50

// MaxSampleBytes caps the byte length of a single sampled rule line
// (DroppedLine.Text, ModifiedLine.From/.To). A pathological upstream that
// ships multi-MB single-line entries would otherwise blow up the preview
// response and stall the browser when expanding the panel. The truncated
// suffix is exposed via DroppedLine.Truncated / ModifiedLine.Truncated so
// the UI can flag it explicitly.
const MaxSampleBytes = 2048

// DroppedLine describes a single line that a step removed. LineNo is
// 1-indexed relative to the step's input. Text is capped at
// MaxSampleBytes; Truncated reports whether the cap kicked in.
type DroppedLine struct {
	LineNo    int    `json:"lineNo"`
	Text      string `json:"text"`
	Reason    string `json:"reason"`
	Truncated bool   `json:"truncated,omitempty"`
}

// ModifiedLine describes a line that the step rewrote without dropping.
// Used today only by the mihomo→shadowrocket built-in for the MATCH→FINAL
// rename. LineNo is 1-indexed relative to the step's input. From/To are
// capped at MaxSampleBytes; Truncated reports whether either side hit the
// cap so the UI can flag the comparison as partial.
type ModifiedLine struct {
	LineNo    int    `json:"lineNo"`
	From      string `json:"from"`
	To        string `json:"to"`
	Reason    string `json:"reason,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// StepReport summarises the effect of one transform stage on one source.
// SourceIndex is only meaningful for the StageRule phase (where multiple
// sources flow side-by-side); merge collapses them into a single track and
// later stages always carry SourceIndex=0.
type StepReport struct {
	Stage       string         `json:"stage"`
	Index       int            `json:"index"`
	SourceIndex int            `json:"sourceIndex,omitempty"`
	Kind        string         `json:"kind"`
	Label       string         `json:"label"`
	InputLines  int            `json:"inputLines"`
	OutputLines int            `json:"outputLines"`
	Dropped     []DroppedLine  `json:"dropped,omitempty"`
	Modified    []ModifiedLine `json:"modified,omitempty"`
	// DroppedTotal / ModifiedTotal record the full pre-cap counts so the UI
	// can show "showing 50 of 287 dropped" when the samples slice is
	// truncated by MaxReportSamples.
	DroppedTotal  int `json:"droppedTotal,omitempty"`
	ModifiedTotal int `json:"modifiedTotal,omitempty"`
}

// FinalStats summarises the post-pipeline content for one client. ByType
// groups rule lines by their leading token (text formats) or by the
// leading token of each payload entry (yaml). PayloadCount is set only
// when the content parses as a yaml document with a top-level payload
// sequence.
type FinalStats struct {
	TotalLines   int            `json:"totalLines"`
	ByType       map[string]int `json:"byType"`
	PayloadCount *int           `json:"payloadCount,omitempty"`
}

// TransformReport bundles all StepReports for one client plus the final
// content statistics. PreviewResult.Reports maps client id → TransformReport.
type TransformReport struct {
	Steps      []StepReport `json:"steps"`
	FinalStats FinalStats   `json:"finalStats"`
}

// AppendDropped is a small helper that respects MaxReportSamples while
// still bumping the running total. Keeping the cap centralised here so
// every emitter (built-ins, replace/remove_lines reporters) behaves
// identically. The sample's Text is byte-capped at MaxSampleBytes before
// it lands in the slice.
func AppendDropped(samples []DroppedLine, total *int, item DroppedLine) []DroppedLine {
	*total++
	if len(samples) >= MaxReportSamples {
		return samples
	}
	item.Text, item.Truncated = capSample(item.Text)
	return append(samples, item)
}

// AppendModified mirrors AppendDropped for the modified-lines bucket. Both
// From and To are byte-capped independently; if either hits the cap, the
// whole entry is flagged as Truncated.
func AppendModified(samples []ModifiedLine, total *int, item ModifiedLine) []ModifiedLine {
	*total++
	if len(samples) >= MaxReportSamples {
		return samples
	}
	from, fTrunc := capSample(item.From)
	to, tTrunc := capSample(item.To)
	item.From = from
	item.To = to
	item.Truncated = fTrunc || tTrunc
	return append(samples, item)
}

// capSample returns text trimmed to MaxSampleBytes plus an "…(truncated N
// bytes)" sentinel when the cap kicked in, and a bool indicating whether
// the cap kicked in. We slice at byte boundaries; for the rule-content
// domain this is safe because rule lines are ASCII-only in practice
// (DOMAIN tokens, IP literals, etc.) and even if a multi-byte sequence
// were split, it would only affect the truncated tail which is
// human-readable text downstream.
func capSample(text string) (string, bool) {
	if len(text) <= MaxSampleBytes {
		return text, false
	}
	cut := MaxSampleBytes
	// Step back so we don't slice inside a multi-byte UTF-8 sequence:
	// the trailing-byte test is `(b & 0xC0) == 0x80`. At most 3 step-backs.
	for cut > 0 && (text[cut]&0xC0) == 0x80 {
		cut--
	}
	return text[:cut], true
}
