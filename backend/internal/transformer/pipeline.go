// Package transformer mirrors src/lib/transformer.ts: pipeline, merge, headers,
// and JS script execution via goja.
package transformer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// Engine encapsulates a shared JS runner used by all transforms.
type Engine struct {
	JS *ScriptRunner
}

// NewEngine creates an engine with default JS runner options.
func NewEngine() *Engine {
	return &Engine{JS: NewScriptRunner(DefaultScriptOptions())}
}

// PipelineCtx bundles the per-invocation lookup tables consumed by a
// pipeline run: user-defined script transformers and the global parameter
// blobs for the built-in transformers. Both maps may be nil.
type PipelineCtx struct {
	Transformers  map[string]schema.ScriptTransformer
	BuiltinParams map[string]json.RawMessage
}

// ApplyNewTransforms runs each transform in order, returning the resulting
// contents (one entry per input source). Used by the sync pipeline where
// the per-step diagnostics are not needed; preview callers should use
// ApplyNewTransformsReported instead.
func (e *Engine) ApplyNewTransforms(contents []string, transforms []schema.Transform, ctx PipelineCtx) ([]string, error) {
	result, _, err := e.applyNewTransforms(contents, transforms, ctx, false, StageRule)
	return result, err
}

// ApplyNewTransformsReported is the preview-only counterpart to
// ApplyNewTransforms: it returns one StepReport per (transform × targeted
// source) pair so the UI can render the step-by-step pipeline. `stage`
// identifies which slot of the engine pipeline (rule/client/override) this
// invocation belongs to and is propagated verbatim into each StepReport so
// downstream callers don't have to post-stamp the field.
func (e *Engine) ApplyNewTransformsReported(contents []string, transforms []schema.Transform, ctx PipelineCtx, stage string) ([]string, []StepReport, error) {
	if stage == "" {
		stage = StageRule
	}
	return e.applyNewTransforms(contents, transforms, ctx, true, stage)
}

func (e *Engine) applyNewTransforms(contents []string, transforms []schema.Transform, ctx PipelineCtx, withReport bool, stage string) ([]string, []StepReport, error) {
	result := append([]string(nil), contents...)
	var reports []StepReport
	for idx, t := range transforms {
		next, stepReports, err := e.executeNewTransform(result, t, ctx, withReport, stage, idx)
		if err != nil {
			return nil, nil, err
		}
		result = next
		if withReport {
			reports = append(reports, stepReports...)
		}
	}
	return result, reports, nil
}

func (e *Engine) executeNewTransform(contents []string, transform schema.Transform, ctx PipelineCtx, withReport bool, stage string, stepIdx int) ([]string, []StepReport, error) {
	indices, all, err := transform.TargetIndices()
	if err != nil {
		return contents, nil, fmt.Errorf("invalid target: %w", err)
	}
	targets := make(map[int]struct{})
	if all {
		for i := range contents {
			targets[i] = struct{}{}
		}
	} else {
		for _, idx := range indices {
			targets[idx] = struct{}{}
		}
	}

	out := make([]string, len(contents))
	var reports []StepReport
	for i, content := range contents {
		if _, ok := targets[i]; !ok {
			out[i] = content
			continue
		}
		switch transform.Type {
		case "use":
			// Dispatch to the built-in registry before consulting the
			// user-script map so a "builtin:" name always means the
			// native Go implementation, never a JS shadow.
			if HasBuiltinPrefix(transform.Use) {
				res, ok := RunBuiltin(transform.Use, ctx.BuiltinParams[transform.Use], content)
				if ok {
					out[i] = res.Output
					if withReport {
						reports = append(reports, StepReport{
							Stage:         stage,
							Index:         stepIdx,
							SourceIndex:   i,
							Kind:          KindUseBuiltin,
							Label:         "use " + transform.Use,
							InputLines:    CountSignificantLines(content),
							OutputLines:   CountSignificantLines(res.Output),
							Dropped:       res.Dropped,
							Modified:      res.Modified,
							DroppedTotal:  res.DroppedTotal,
							ModifiedTotal: res.ModifiedTotal,
						})
					}
					continue
				}
				// Unknown builtin: fall through with content untouched.
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindUseBuiltin, "use "+transform.Use+" (unknown)", content))
				}
				continue
			}
			t, ok := ctx.Transformers[transform.Use]
			if !ok || strings.TrimSpace(t.Script) == "" {
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindUse, "use "+transform.Use, content))
				}
				continue
			}
			res, _ := e.JS.Execute(t.Script, content)
			out[i] = res
			if withReport {
				step := StepReport{
					Stage:       stage,
					Index:       stepIdx,
					SourceIndex: i,
					Kind:        KindUse,
					Label:       "use " + transform.Use,
					InputLines:  CountSignificantLines(content),
					OutputLines: CountSignificantLines(res),
				}
				step.Dropped, step.DroppedTotal, step.Modified, step.ModifiedTotal = SampleLineDiff(content, res, "user script removed line", "user script rewrote line")
				reports = append(reports, step)
			}
		case "replace":
			if transform.Pattern == "" {
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindReplace, "replace (empty pattern)", content))
				}
				continue
			}
			replaced, err := e.JS.RunRegexReplace(content, transform.Pattern, transform.Replacement, transform.Flags)
			if err != nil {
				// TS silently returns original content on regex error.
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindReplace, "replace /"+transform.Pattern+"/", content))
				}
				continue
			}
			out[i] = replaced
			if withReport {
				step := StepReport{
					Stage:       stage,
					Index:       stepIdx,
					SourceIndex: i,
					Kind:        KindReplace,
					Label:       "replace /" + transform.Pattern + "/",
					InputLines:  CountSignificantLines(content),
					OutputLines: CountSignificantLines(replaced),
				}
				step.Dropped, step.DroppedTotal, step.Modified, step.ModifiedTotal = SampleLineDiff(content, replaced, "regex removed line", "regex rewrote line")
				reports = append(reports, step)
			}
		case "remove_lines":
			if transform.Pattern == "" {
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindRemoveLines, "remove_lines (empty pattern)", content))
				}
				continue
			}
			filtered, err := e.JS.RunRegexRemoveLines(content, transform.Pattern)
			if err != nil {
				out[i] = content
				if withReport {
					reports = append(reports, noopStepReport(stage, stepIdx, i, KindRemoveLines, "remove_lines /"+transform.Pattern+"/", content))
				}
				continue
			}
			out[i] = filtered
			if withReport {
				step := StepReport{
					Stage:       stage,
					Index:       stepIdx,
					SourceIndex: i,
					Kind:        KindRemoveLines,
					Label:       "remove_lines /" + transform.Pattern + "/",
					InputLines:  CountSignificantLines(content),
					OutputLines: CountSignificantLines(filtered),
				}
				step.Dropped, step.DroppedTotal, _, _ = SampleLineDiff(content, filtered, "matched remove_lines pattern", "")
				reports = append(reports, step)
			}
		default:
			out[i] = content
		}
	}
	return out, reports, nil
}

func noopStepReport(stage string, stepIdx, sourceIdx int, kind, label, content string) StepReport {
	lines := CountSignificantLines(content)
	return StepReport{
		Stage:       stage,
		Index:       stepIdx,
		SourceIndex: sourceIdx,
		Kind:        kind,
		Label:       label,
		InputLines:  lines,
		OutputLines: lines,
	}
}

// MergeContents mirrors mergeContents in transformer.ts.
func MergeContents(contents []string, strategy string, dedupe bool) string {
	if len(contents) == 0 {
		return ""
	}
	switch strategy {
	case "concat":
		result := strings.Join(contents, "\n")
		if !dedupe {
			return result
		}
		lines := strings.Split(result, "\n")
		seen := make(map[string]struct{})
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				out = append(out, line)
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, line)
		}
		return strings.Join(out, "\n")
	case "union":
		seen := make(map[string]struct{})
		order := make([]string, 0)
		for _, c := range contents {
			for _, line := range strings.Split(c, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if _, ok := seen[trimmed]; ok {
					continue
				}
				seen[trimmed] = struct{}{}
				order = append(order, trimmed)
			}
		}
		return strings.Join(order, "\n")
	case "intersect":
		if len(contents) == 1 {
			return contents[0]
		}
		var firstOrder []string
		firstSet := make(map[string]struct{})
		for _, line := range strings.Split(contents[0], "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if _, ok := firstSet[trimmed]; ok {
				continue
			}
			firstSet[trimmed] = struct{}{}
			firstOrder = append(firstOrder, trimmed)
		}
		current := firstSet
		for i := 1; i < len(contents); i++ {
			next := make(map[string]struct{})
			for _, line := range strings.Split(contents[i], "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if _, ok := current[trimmed]; ok {
					next[trimmed] = struct{}{}
				}
			}
			current = next
		}
		out := make([]string, 0, len(current))
		for _, line := range firstOrder {
			if _, ok := current[line]; ok {
				out = append(out, line)
			}
		}
		return strings.Join(out, "\n")
	default:
		return strings.Join(contents, "\n")
	}
}

// StripManagedRuleHeader removes the legacy managed header so a freshly
// produced (header-less) artifact compares fairly against an on-disk file
// written by a pre-"zero header" release. Production no longer emits this
// header; the function only matters during the first resync after upgrade.
//
// Critically, when no legacy header is found the function returns content
// VERBATIM (no line-ending normalisation). Earlier revisions normalised
// CR/LF unconditionally, which caused freshly produced CRLF content to
// disagree with the LF-normalised "previous source" on every resync,
// triggering a write + change record on every sync for CRLF artifacts.
func StripManagedRuleHeader(content string) string {
	if content == "" {
		return ""
	}
	headerEnd, ok := legacyHeaderEnd(content)
	if !ok {
		return content
	}
	return content[headerEnd:]
}

// legacyHeaderEnd returns the byte offset of the first body byte after a
// legacy managed header, or ok=false when no header is present. The
// returned offset is safe to slice the original string with because the
// header was always emitted using LF terminators.
func legacyHeaderEnd(content string) (int, bool) {
	// consumeLine returns the next \n-terminated line (with any trailing
	// \r trimmed for resilience) and the byte offset of the byte after
	// the LF.
	consumeLine := func(start int) (line string, next int) {
		rest := content[start:]
		n := strings.IndexByte(rest, '\n')
		if n < 0 {
			return strings.TrimRight(rest, "\r"), len(content)
		}
		return strings.TrimRight(rest[:n], "\r"), start + n + 1
	}
	pos := 0
	l0, next := consumeLine(pos)
	if !strings.HasPrefix(l0, "# 规则数量：") {
		return 0, false
	}
	pos = next
	l1, next := consumeLine(pos)
	if !strings.HasPrefix(l1, "# 更新时间：") {
		return 0, false
	}
	pos = next
	l2, next := consumeLine(pos)
	if l2 != "# 规则类型：" {
		return 0, false
	}
	pos = next
	for pos < len(content) {
		line, np := consumeLine(pos)
		if !strings.HasPrefix(line, "# ") {
			break
		}
		pos = np
	}
	if pos < len(content) {
		line, np := consumeLine(pos)
		if strings.TrimSpace(line) == "" {
			pos = np
		}
	}
	return pos, true
}

// NormalizeEffectiveRuleContent returns the effective rule lines joined by \n.
func NormalizeEffectiveRuleContent(content string) string {
	return strings.Join(effectiveRuleLines(content), "\n")
}

func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

func effectiveRuleLines(content string) []string {
	if content == "" {
		return nil
	}
	normalized := normalizeLineEndings(content)
	var out []string
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
