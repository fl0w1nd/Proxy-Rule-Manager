package serve

import (
	"net/http"
	"runtime"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/version"
)

type statusResponse struct {
	Version            string `json:"version"`
	GoVersion          string `json:"go_version"`
	LastCheck          string `json:"last_check,omitempty"`
	PublishedArtifacts int    `json:"published_artifacts"`
}

type rulesResponse struct {
	Items []ruleInfo `json:"items"`
	Total int        `json:"total"`
}

type ruleInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Entries   int            `json:"entries"`
	VersionAt string         `json:"version_at,omitempty"`
	LastCheck *ruleCheckInfo `json:"last_check,omitempty"`
}

type ruleCheckInfo struct {
	Result    string `json:"result"`
	CheckedAt string `json:"checked_at"`
}

type geositeProvidersResponse struct {
	Items []geositeProviderInfo `json:"items"`
	Total int                   `json:"total"`
}

type geositeProviderInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Result    string `json:"result"`
	CheckedAt string `json:"checked_at,omitempty"`
	Lists     int    `json:"lists"`
	Variants  int    `json:"variants"`
	Entries   int    `json:"entries"`
	Files     int    `json:"files"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	artifacts, err := engine.CountArtifacts(s.DataDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "artifact_count_failed", "统计发布文件失败", map[string]any{})
		return
	}
	resp := statusResponse{Version: version.Current(), GoVersion: runtime.Version(), PublishedArtifacts: artifacts}
	if checkedAt, ok := s.State.LastCheck(); ok {
		resp.LastCheck = apiTime(checkedAt)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRules(w http.ResponseWriter, _ *http.Request) {
	cfg := s.config()
	resp := rulesResponse{Items: make([]ruleInfo, 0, len(cfg.Rules)), Total: len(cfg.Rules)}
	for _, rule := range cfg.Rules {
		entries, _ := s.State.RuleEntryCount(rule.ID)
		item := ruleInfo{ID: rule.ID, Name: rule.Name, Entries: entries}
		if result, checkedAt, versionAt, ok := s.State.RuleUpdate(rule.ID); ok {
			item.LastCheck = &ruleCheckInfo{Result: result, CheckedAt: apiTime(checkedAt)}
			if !versionAt.IsZero() {
				item.VersionAt = apiTime(versionAt)
			}
		}
		resp.Items = append(resp.Items, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGeositeProviders(w http.ResponseWriter, _ *http.Request) {
	summaries := s.Engine.GeositeProviderSummaries()
	resp := geositeProvidersResponse{Items: make([]geositeProviderInfo, 0, len(summaries)), Total: len(summaries)}
	for _, summary := range summaries {
		item := geositeProviderInfo{
			Name: summary.Name, Version: summary.Version, Result: summary.Result,
			Lists: summary.Lists, Variants: summary.Variants, Entries: summary.Entries, Files: summary.Files,
		}
		if !summary.CheckedAt.IsZero() {
			item.CheckedAt = apiTime(summary.CheckedAt)
		}
		resp.Items = append(resp.Items, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func apiTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}
