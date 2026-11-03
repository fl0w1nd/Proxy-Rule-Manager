package ir

import (
	"sort"
	"strings"
)

// LintLine is one parsed entry together with the input line it came from.
type LintLine struct {
	Line  int    `json:"line"`
	Text  string `json:"text"`
	Kind  Kind   `json:"kind"`
	Value string `json:"value"`
}

// LintReport describes how a rule document would be parsed by a source,
// without fetching, preprocessing, or merging anything.
type LintReport struct {
	Format      string       `json:"format,omitempty"`
	Confidence  float64      `json:"confidence"`
	Entries     int          `json:"entries"`
	Kinds       []KindCount  `json:"kinds"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	// DiagnosticCount is the total, Diagnostics may be truncated.
	DiagnosticCount int `json:"diagnostic_count"`
	// Lines holds the first parsed entries with their line numbers; it is
	// truncated to the limit passed to Lint.
	Lines          []LintLine `json:"lines"`
	LinesTruncated bool       `json:"lines_truncated,omitempty"`
}

// MaxLintDiagnostics bounds the diagnostics a LintReport carries.
const MaxLintDiagnostics = 500

// ErrUndetectedFormat is the reason reported when no parser accepts content.
const ErrUndetectedFormat = "could not detect rule format"

// Lint parses content exactly like a text source would (same detection and
// grammar as Parse) and reports per-line results. maxLines bounds the parsed
// entries echoed back in Lines; 0 disables them.
func Lint(content string, maxLines int) LintReport {
	report := LintReport{Kinds: []KindCount{}, Diagnostics: []Diagnostic{}, Lines: []LintLine{}}
	det := Detect(content)
	if det.Confidence <= 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Text: "(document)", Reason: ErrUndetectedFormat})
		report.DiagnosticCount = 1
		return report
	}
	report.Format, report.Confidence = det.Format, det.Confidence
	var entries []Entry
	parsers[det.Format].walk(content, func(line int, text string, entry Entry, err error) {
		if err != nil {
			report.DiagnosticCount++
			if len(report.Diagnostics) < MaxLintDiagnostics {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{Line: line, Text: text, Reason: err.Error()})
			}
			return
		}
		entries = append(entries, entry)
		if len(report.Lines) < maxLines {
			report.Lines = append(report.Lines, LintLine{Line: line, Text: text, Kind: entry.Kind, Value: lintValue(entry)})
		} else if maxLines > 0 {
			report.LinesTruncated = true
		}
	})
	report.Entries = len(entries)
	report.Kinds = CountKinds(entries)
	return report
}

// RuleTypeSpelling is one accepted rule-type keyword.
type RuleTypeSpelling struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// Alias marks dialect spellings of a kind whose canonical keyword is
	// listed separately.
	Alias bool `json:"alias,omitempty"`
}

// RuleTypeSpellings lists every keyword the entry grammar accepts, canonical
// spellings first (in AllKinds order), then aliases alphabetically.
func RuleTypeSpellings() []RuleTypeSpelling {
	order := map[Kind]int{}
	for i, kind := range AllKinds {
		order[kind] = i
	}
	out := make([]RuleTypeSpelling, 0, len(ruleTypeSpellings)+3)
	for name, kind := range ruleTypeSpellings {
		out = append(out, RuleTypeSpelling{Name: name, Kind: kind, Alias: name != canonicalSpelling(kind)})
	}
	for _, name := range []string{"AND", "OR", "NOT"} {
		out = append(out, RuleTypeSpelling{Name: name, Kind: Kind(strings.ToLower(name))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Alias != out[j].Alias {
			return !out[i].Alias
		}
		if !out[i].Alias && order[out[i].Kind] != order[out[j].Kind] {
			return order[out[i].Kind] < order[out[j].Kind]
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RuleLineFlags lists the per-line parameters the grammar recognises.
func RuleLineFlags() []string {
	out := make([]string, 0, len(knownLineFlags))
	for flag := range knownLineFlags {
		out = append(out, flag)
	}
	sort.Strings(out)
	return out
}

func canonicalSpelling(kind Kind) string {
	return strings.ToUpper(strings.ReplaceAll(string(kind), "_", "-"))
}

// lintValue renders the normalized value with its flags; logical entries
// show their whole tree since they have no scalar value.
func lintValue(e Entry) string {
	if e.Kind.IsLogical() {
		return e.Display()
	}
	return strings.Join(append([]string{e.Value}, e.Flags...), ",")
}
