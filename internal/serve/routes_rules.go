package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"gopkg.in/yaml.v3"
)

const (
	rulePreviewTimeout   = 30 * time.Second
	rulePreviewDiffLimit = 20
	rulePreviewMaxBytes  = 256 * 1024
)

type rulePreviewRequest struct {
	Rule json.RawMessage `json:"rule"`
}

type rulePreviewSource struct {
	Details     []string `json:"details,omitempty"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Entries     int      `json:"entries"`
	Diagnostics int      `json:"diagnostics"`
	Error       string   `json:"error,omitempty"`
	DurationMs  int64    `json:"duration_ms"`
}

type rulePreviewArtifact struct {
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Output     string `json:"output,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Error      string `json:"error,omitempty"`
}

type rulePreviewKindDelta struct {
	Kind    string   `json:"kind"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type rulePreviewDiff struct {
	Added   int                    `json:"added"`
	Removed int                    `json:"removed"`
	Groups  []rulePreviewKindDelta `json:"groups,omitempty"`
}

type rulePreviewResponse struct {
	RuleID      string                `json:"rule_id"`
	RuleName    string                `json:"rule_name"`
	ElapsedMs   int64                 `json:"elapsed_ms"`
	Sources     []rulePreviewSource   `json:"sources"`
	PreOps      int                   `json:"pre_ops"`
	PostOps     int                   `json:"post_ops"`
	Merged      int                   `json:"merged"`
	MergedKinds []ir.KindCount        `json:"merged_kinds,omitempty"`
	OpsError    string                `json:"ops_error,omitempty"`
	OpsDiff     rulePreviewDiff       `json:"ops_diff"`
	Outputs     []rulePreviewArtifact `json:"outputs"`
}

func (s *Server) handleRulePreview(w http.ResponseWriter, r *http.Request) {
	var request rulePreviewRequest
	if !decodeConfigRequest(w, r, &request) {
		return
	}
	if len(request.Rule) == 0 || string(request.Rule) == "null" {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{
			"errors": []configIssue{{Path: "rule", Message: "required"}},
		})
		return
	}
	var rule config.RuleConfig
	if err := yaml.Unmarshal(request.Rule, &rule); err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{
			"errors": []configIssue{{Path: "rule", Message: err.Error()}},
		})
		return
	}
	if rule.ID == "" {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{
			"errors": []configIssue{{Path: "rule.id", Message: "required"}},
		})
		return
	}

	cfg := s.config().DeepCopy()
	overlayRule(cfg, rule)
	if errs := cfg.Validate(s.DataDir); len(errs) > 0 {
		writeConfigMutationError(w, s.ConfigManager, config.ConfigErrors(errs))
		return
	}

	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), rulePreviewTimeout)
	defer cancel()
	report, err := engine.Preview(
		ctx,
		cfg,
		s.DataDir,
		rule.ID,
		"",
		s.Engine.Registry,
		s.Engine.Fetcher,
		s.Engine.Preprocessor,
		s.Engine.Geosite,
		s.Engine.GeoIP,
		s.Engine.Logger,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeAPIError(w, http.StatusGatewayTimeout, "preview_timeout", "预览超时，请减少远程来源或稍后重试", map[string]any{})
			return
		}
		var missing *engine.RuleNotFoundError
		if errors.As(err, &missing) {
			writeAPIError(w, http.StatusNotFound, "rule_not_found", missing.Error(), map[string]any{})
			return
		}
		writeAPIError(w, http.StatusUnprocessableEntity, "preview_failed", "规则预览失败", map[string]any{"reason": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buildRulePreviewResponse(cfg, report, time.Since(started)))
}

func overlayRule(cfg *config.Config, rule config.RuleConfig) {
	for i := range cfg.Rules {
		if cfg.Rules[i].ID == rule.ID {
			cfg.Rules[i] = rule
			return
		}
	}
	cfg.Rules = append(cfg.Rules, rule)
}

func buildRulePreviewResponse(cfg *config.Config, report *engine.PreviewReport, elapsed time.Duration) rulePreviewResponse {
	sources := make([]rulePreviewSource, 0, len(report.Sources))
	var definitions []config.SourceConfig
	for _, rule := range cfg.Rules {
		if rule.ID == report.RuleID {
			definitions = rule.Sources
			break
		}
	}
	for i, source := range report.Sources {
		label := fmt.Sprintf("来源 %d", i+1)
		var details []string
		if i < len(definitions) {
			definition := definitions[i]
			label = previewSourceName(definition, i)
			if definition.Group != nil {
				for j, child := range definition.Group {
					details = append(details, previewSourceName(child, j))
				}
			} else if definition.Label != "" {
				definition.Label = ""
				details = append(details, previewSourceName(definition, i))
			}
		}
		sources = append(sources, rulePreviewSource{
			Label:       label,
			Details:     details,
			Type:        source.Type,
			Entries:     len(source.Entries),
			Diagnostics: len(source.Diagnostics),
			Error:       source.Error,
			DurationMs:  source.DurationMs,
		})
	}
	targets := config.ExpandSelectedTargets(cfg.Clients, previewOutputIDs(cfg, report.RuleID))
	outputs := make([]rulePreviewArtifact, 0, len(targets))
	for _, target := range targets {
		item := rulePreviewArtifact{ID: target.ID, Name: target.OptionName, ClientID: target.ClientID, ClientName: target.ClientName}
		if errText := report.ArtifactErrors[target.ID]; errText != "" {
			item.Error = errText
			outputs = append(outputs, item)
			continue
		}
		raw := report.Artifacts[target.ID]
		if len(raw) > rulePreviewMaxBytes {
			item.Output = string(raw[:rulePreviewMaxBytes])
			item.Truncated = true
		} else {
			item.Output = string(raw)
		}
		outputs = append(outputs, item)
	}
	return rulePreviewResponse{
		RuleID:      report.RuleID,
		RuleName:    report.RuleName,
		ElapsedMs:   elapsed.Milliseconds(),
		Sources:     sources,
		PreOps:      len(report.PreOps),
		PostOps:     len(report.PostOps),
		Merged:      len(report.Merged),
		MergedKinds: ir.CountKinds(report.Merged),
		OpsError:    report.OpsError,
		OpsDiff:     previewDiff(report.OpsDiff, rulePreviewDiffLimit),
		Outputs:     outputs,
	}
}

func previewDiff(diff ir.EntryDiff, limit int) rulePreviewDiff {
	capped := capEntryDiff(diff, limit)
	groups := make([]rulePreviewKindDelta, 0, len(capped.Groups))
	for _, group := range capped.Groups {
		groups = append(groups, rulePreviewKindDelta{Kind: string(group.Kind), Added: group.Added, Removed: group.Removed})
	}
	return rulePreviewDiff{Added: capped.AddedCount, Removed: capped.RemovedCount, Groups: groups}
}

func previewOutputIDs(cfg *config.Config, ruleID string) []string {
	for _, rule := range cfg.Rules {
		if rule.ID == ruleID {
			return append([]string(nil), rule.Outputs...)
		}
	}
	return nil
}

func capEntryDiff(diff ir.EntryDiff, limit int) ir.EntryDiff {
	if limit <= 0 {
		return diff
	}
	groups := make([]ir.KindDelta, 0, len(diff.Groups))
	for _, group := range diff.Groups {
		item := group
		if len(item.Added) > limit {
			item.Added = append([]string(nil), item.Added[:limit]...)
		}
		if len(item.Removed) > limit {
			item.Removed = append([]string(nil), item.Removed[:limit]...)
		}
		groups = append(groups, item)
	}
	diff.Groups = groups
	return diff
}

func previewSourceName(source config.SourceConfig, index int) string {
	if source.Label != "" {
		return source.Label
	}
	switch source.SourceType() {
	case "group":
		return fmt.Sprintf("来源组 %d", index+1)
	case "url":
		return source.URL
	case "local":
		if source.File != "" {
			return source.File
		}
		return fmt.Sprintf("内联文本 %d", index+1)
	case "ref":
		return source.Ref
	case "geosite":
		if ref, err := source.ResolveGeositeRef(); err == nil {
			return ref.FormatRef()
		}
	case "geoip":
		if ref, err := source.ResolveGeoIPRef(); err == nil {
			return ref.Provider + "/" + ref.List
		}
	}
	return fmt.Sprintf("来源 %d", index+1)
}
