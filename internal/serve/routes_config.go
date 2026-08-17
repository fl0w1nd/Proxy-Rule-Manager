package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

type configSnapshotResponse struct {
	Version int64 `json:"version"`
	Config  any   `json:"config"`
}

type configMutationResponse struct {
	Status   string   `json:"status,omitempty"`
	Version  int64    `json:"version"`
	Warnings []string `json:"warnings"`
}

type configPatchRequest struct {
	Version int64                  `json:"version"`
	Ops     []configPatchOperation `json:"ops"`
}

type configPatchOperation struct {
	Op        string          `json:"op"`
	ID        string          `json:"id,omitempty"`
	RuleID    string          `json:"rule_id,omitempty"`
	OutputID  string          `json:"output_id,omitempty"`
	RuleIDs   []string        `json:"rule_ids,omitempty"`
	OutputIDs []string        `json:"output_ids,omitempty"`
	Order     []string        `json:"order,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	present   map[string]bool
}

func (op *configPatchOperation) UnmarshalJSON(data []byte) error {
	type operationFields configPatchOperation
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	known := map[string]bool{
		"op": true, "id": true, "rule_id": true, "output_id": true,
		"rule_ids": true, "output_ids": true, "order": true, "value": true,
	}
	for field := range fields {
		if !known[field] {
			return fmt.Errorf("json: unknown field %q", field)
		}
	}
	var decoded operationFields
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*op = configPatchOperation(decoded)
	op.present = make(map[string]bool, len(fields))
	for field := range fields {
		op.present[field] = true
	}
	return nil
}

type configIssue struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	source, version, err := s.ConfigManager.SourceSnapshot()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "config_read_failed", "读取配置失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, configSnapshotResponse{Version: version, Config: source})
}

// handleConfigDirty reports whether the config file differs from the managed
// source document. The frontend polls this before offering a reload.
func (s *Server) handleConfigDirty(w http.ResponseWriter, _ *http.Request) {
	if s.configFile == "" {
		writeJSON(w, http.StatusOK, map[string]any{"changed": false})
		return
	}
	changed, err := s.ConfigManager.Dirty()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "config_read_failed", "读取配置文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": changed})
}

func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeConfigPatchRequest(w, r)
	if !ok {
		return
	}
	ops, err := buildPatchOperations(request.Ops)
	if err != nil {
		writeConfigMutationError(w, s.ConfigManager, err)
		return
	}
	candidate, err := s.ConfigManager.Prepare(request.Version, ops)
	if err != nil {
		writeConfigMutationError(w, s.ConfigManager, err)
		return
	}
	if candidate.Changed() {
		if err := s.preflightConfig(candidate.Config()); err != nil {
			writeConfigMutationError(w, s.ConfigManager, err)
			return
		}
	}
	version, warnings, err := s.commitConfig(candidate)
	if err != nil {
		writeConfigMutationError(w, s.ConfigManager, err)
		return
	}
	writeJSON(w, http.StatusOK, configMutationResponse{Version: version, Warnings: warnings})
}

// handleConfigReload imports the current source file through the same runtime
// transaction used by the patch endpoint.
func (s *Server) handleConfigReload(w http.ResponseWriter, _ *http.Request) {
	if s.configFile == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "reload_unavailable", "配置热重载未启用", map[string]any{})
		return
	}
	candidate, err := s.ConfigManager.PrepareReload()
	if err != nil {
		writeConfigMutationError(w, s.ConfigManager, err)
		return
	}
	if candidate.Changed() {
		if err := s.preflightConfig(candidate.Config()); err != nil {
			writeConfigMutationError(w, s.ConfigManager, err)
			return
		}
	}
	version, warnings, err := s.commitConfig(candidate)
	if err != nil {
		writeConfigMutationError(w, s.ConfigManager, err)
		return
	}
	writeJSON(w, http.StatusOK, configMutationResponse{Status: "reloaded", Version: version, Warnings: warnings})
}

func decodeConfigPatchRequest(w http.ResponseWriter, r *http.Request) (configPatchRequest, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求需要 application/json", map[string]any{})
		return configPatchRequest{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request configPatchRequest
	if err := decoder.Decode(&request); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "请求内容超过 1 MiB", map[string]any{})
		} else {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{"errors": []configIssue{{Path: "request", Message: err.Error()}}})
		}
		return configPatchRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求只能包含一个 JSON 对象", map[string]any{"errors": []configIssue{{Path: "request", Message: "must contain exactly one JSON object"}}})
		return configPatchRequest{}, false
	}
	if request.Version < 1 {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_patch", "配置版本必须大于零", map[string]any{"errors": []configIssue{{Path: "version", Message: "must be greater than zero"}}})
		return configPatchRequest{}, false
	}
	if len(request.Ops) == 0 {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_patch", "配置变更不能为空", map[string]any{"errors": []configIssue{{Path: "ops", Message: "at least one operation is required"}}})
		return configPatchRequest{}, false
	}
	return request, true
}

func buildPatchOperations(items []configPatchOperation) ([]config.PatchOp, error) {
	ops := make([]config.PatchOp, len(items))
	for i, item := range items {
		if field := unexpectedPatchField(item); field != "" {
			return nil, &config.PatchError{OpIndex: i, Path: field, Message: fmt.Sprintf("field is not valid for %q", item.Op)}
		}
		op := config.PatchOp{
			Type: item.Op, ID: item.ID, RuleID: item.RuleID, OutputID: item.OutputID,
			RuleIDs: append([]string(nil), item.RuleIDs...), OutputIDs: append([]string(nil), item.OutputIDs...), Order: append([]string(nil), item.Order...),
		}
		if len(item.Value) > 0 {
			value, err := config.ParsePatchValue(item.Value)
			if err != nil {
				return nil, &config.PatchError{OpIndex: i, Path: "value", Message: err.Error()}
			}
			op.Value = value
		}
		ops[i] = op
	}
	return ops, nil
}

func unexpectedPatchField(item configPatchOperation) string {
	allowed := map[string]bool{"op": true}
	switch item.Op {
	case "add_client", "add_rule", "update_schedule", "update_fetch", "update_preprocess", "update_history", "update_geosite":
		allowed["value"] = true
	case "update_client", "update_rule":
		allowed["id"], allowed["value"] = true, true
	case "remove_client", "remove_rule":
		allowed["id"] = true
	case "add_output", "remove_output":
		allowed["rule_id"], allowed["output_id"] = true, true
	case "batch_add_output", "batch_remove_output":
		allowed["rule_ids"], allowed["output_ids"] = true, true
	case "reorder_rules":
		allowed["order"] = true
	}
	for field := range item.present {
		if !allowed[field] {
			return field
		}
	}
	return ""
}

func (s *Server) preflightConfig(cfg *config.Config) error {
	for i, client := range cfg.Clients {
		for _, target := range config.ExpandClientTargets(client) {
			if _, ok := s.Engine.Registry.Get(target.Template); !ok {
				return config.ConfigErrors{{Path: fmt.Sprintf("clients[%d]", i), Message: fmt.Sprintf("output target %q references unknown template %q", target.ID, target.Template)}}
			}
		}
	}
	targets := config.ExpandOutputTargets(cfg.Clients)
	ids := make([]string, len(targets))
	for i, target := range targets {
		ids[i] = target.ID
	}
	if err := engine.EnsureArtifactDirs(s.DataDir, ids); err != nil {
		return fmt.Errorf("prepare artifact directories: %w", err)
	}
	return nil
}

func (s *Server) commitConfig(candidate *config.Candidate) (int64, []string, error) {
	var version int64
	changed := candidate.Changed()
	err := s.updates.ReconfigureChanged(changed, func() (*config.Config, error) {
		cfg, committedVersion, err := s.ConfigManager.Commit(candidate)
		if err != nil {
			return nil, err
		}
		version = committedVersion
		if changed {
			s.activateConfig(cfg)
		}
		return cfg, nil
	})
	if err != nil {
		return 0, nil, err
	}
	if !changed {
		return version, []string{}, nil
	}
	return version, s.reconcileConfig(candidate.Config()), nil
}

func (s *Server) activateConfig(cfg *config.Config) {
	s.Engine.SetConfig(cfg)
	s.Engine.Fetcher.Configure(
		time.Duration(cfg.Update.Fetch.Timeout), int64(cfg.Update.Fetch.MaxDownload),
		cfg.Update.Fetch.Concurrency, cfg.Update.Fetch.PerHostConcurrency,
		cfg.Update.Fetch.Retries, time.Duration(cfg.Update.Fetch.RetryDelay), cfg.Update.Fetch.UserAgent,
	)
	s.Engine.Preprocessor.Configure(
		time.Duration(cfg.Update.Preprocess.Timeout), int(int64(cfg.Update.Preprocess.MaxOutput)),
	)
}

func (s *Server) reconcileConfig(cfg *config.Config) []string {
	warnings := make([]string, 0)
	ruleIDs := make([]string, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if changed, err := s.State.BackfillEntryCounts(ruleIDs); err != nil {
		warnings = append(warnings, fmt.Sprintf("回填规则状态失败：%v", err))
	} else if changed {
		if err := s.State.Save(); err != nil {
			warnings = append(warnings, fmt.Sprintf("保存规则状态失败：%v", err))
		}
	}
	if err := s.Engine.EnsureSite(); err != nil {
		warnings = append(warnings, fmt.Sprintf("刷新规则站失败：%v", err))
	}
	return warnings
}

func writeConfigMutationError(w http.ResponseWriter, manager *config.Manager, err error) {
	var versionConflict *config.VersionConflictError
	if errors.As(err, &versionConflict) {
		source, currentVersion, sourceErr := manager.SourceSnapshot()
		if sourceErr != nil {
			writeAPIError(w, http.StatusInternalServerError, "config_read_failed", "读取当前配置失败", map[string]any{})
			return
		}
		writeAPIError(w, http.StatusConflict, "config_version_conflict", "配置版本已经变化", map[string]any{"current_version": currentVersion, "config": source})
		return
	}
	var dirty *config.DirtyConfigError
	if errors.As(err, &dirty) {
		writeAPIError(w, http.StatusConflict, "config_dirty", "配置文件已被外部修改，请先重新加载", map[string]any{})
		return
	}
	if errors.Is(err, updates.ErrUpdateInProgress) {
		writeAPIError(w, http.StatusConflict, "update_in_progress", "更新进行中，请稍后修改配置", map[string]any{})
		return
	}
	if errors.Is(err, config.ErrPersistenceUnavailable) {
		writeAPIError(w, http.StatusServiceUnavailable, "config_write_unavailable", "配置写入未启用", map[string]any{})
		return
	}
	var patchErr *config.PatchError
	if errors.As(err, &patchErr) {
		path := fmt.Sprintf("ops[%d]", patchErr.OpIndex)
		if patchErr.Path != "" {
			path += "." + patchErr.Path
		}
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_patch", "配置变更无效", map[string]any{"errors": []configIssue{{Path: path, Message: patchErr.Message}}})
		return
	}
	var configErrs config.ConfigErrors
	if errors.As(err, &configErrs) {
		issues := make([]configIssue, len(configErrs))
		for i, issue := range configErrs {
			issues[i] = configIssue{Path: issue.Path, Line: issue.Line, Message: issue.Message}
		}
		writeAPIError(w, http.StatusUnprocessableEntity, "config_invalid", "配置校验失败", map[string]any{"errors": issues})
		return
	}
	var documentErr *config.InvalidDocumentError
	if errors.As(err, &documentErr) {
		writeAPIError(w, http.StatusUnprocessableEntity, "config_invalid", "配置文件无效", map[string]any{"errors": []configIssue{{Path: "config", Message: documentErr.Error()}}})
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "config_update_failed", "配置更新失败", map[string]any{"reason": err.Error()})
}
