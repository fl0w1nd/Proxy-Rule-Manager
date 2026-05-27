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

// DroppedLine describes a single line that a step removed. LineNo is
// 1-indexed relative to the step's input.
type DroppedLine struct {
	LineNo int    `json:"lineNo"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// ModifiedLine describes a line that the step rewrote without dropping.
// Used today only by the mihomo→shadowrocket built-in for the MATCH→FINAL
// rename. LineNo is 1-indexed relative to the step's input.
type ModifiedLine struct {
	LineNo int    `json:"lineNo"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
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
// identically.
func AppendDropped(samples []DroppedLine, total *int, item DroppedLine) []DroppedLine {
	*total++
	if len(samples) >= MaxReportSamples {
		return samples
	}
	return append(samples, item)
}

// AppendModified mirrors AppendDropped for the modified-lines bucket.
func AppendModified(samples []ModifiedLine, total *int, item ModifiedLine) []ModifiedLine {
	*total++
	if len(samples) >= MaxReportSamples {
		return samples
	}
	return append(samples, item)
}
