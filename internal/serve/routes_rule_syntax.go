package serve

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

// lintPreviewLines bounds the parsed lines echoed by the lint endpoint.
const lintPreviewLines = 2000

type ruleSyntaxResponse struct {
	Types []ir.RuleTypeSpelling `json:"types"`
	Flags []string              `json:"flags"`
}

type ruleSyntaxLintResponse struct {
	ir.LintReport
	Warnings []string `json:"warnings"`
}

// handleRuleSyntax lists the rule-type keywords and line flags the parser
// accepts, so editors can offer completions without duplicating the grammar.
func (s *Server) handleRuleSyntax(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, ruleSyntaxResponse{Types: ir.RuleTypeSpellings(), Flags: ir.RuleLineFlags()})
}

// handleRuleSyntaxLint parses rule text with the same detection and grammar
// a text source uses and reports per-line results. The optional name is a
// filename used only for extension/format consistency warnings.
func (s *Server) handleRuleSyntaxLint(w http.ResponseWriter, r *http.Request) {
	var request localFileWriteRequest
	if !decodeLocalFileRequest(w, r, &request) {
		return
	}
	content, ok := localFileContent(w, request.Content)
	if !ok {
		return
	}
	report := ir.Lint(content, lintPreviewLines)
	writeJSON(w, http.StatusOK, ruleSyntaxLintResponse{LintReport: report, Warnings: lintWarnings(request.Name, report)})
}

func lintWarnings(name string, report ir.LintReport) []string {
	warnings := []string{}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	switch {
	case report.Format == "":
	case ext == ".yaml" && report.Format != ir.FormatMihomoYAML:
		warnings = append(warnings, "扩展名为 .yaml，但内容没有 payload: 列表，将按规则行列表解析")
	case (ext == ".list" || ext == ".txt") && report.Format == ir.FormatMihomoYAML:
		warnings = append(warnings, "内容是 YAML payload 格式，建议使用 .yaml 扩展名")
	}
	return warnings
}
