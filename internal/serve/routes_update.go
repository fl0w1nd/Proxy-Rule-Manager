package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

type createUpdateRequest struct {
	Scope   string   `json:"scope"`
	RuleIDs []string `json:"rule_ids"`
}

type updateSummary struct {
	ID                 string   `json:"id"`
	Origin             string   `json:"origin"`
	Scope              string   `json:"scope"`
	RequestedRuleIDs   []string `json:"requested_rule_ids"`
	Status             string   `json:"status"`
	StartedAt          string   `json:"started_at"`
	FinishedAt         string   `json:"finished_at,omitempty"`
	RulesTotal         int      `json:"rules_total"`
	RulesSucceeded     int      `json:"rules_succeeded"`
	RulesFailed        int      `json:"rules_failed"`
	ArtifactsProcessed int      `json:"artifacts_processed"`
	PublishedArtifacts int      `json:"published_artifacts"`
	ChangeCount        int      `json:"change_count"`
	WarningCount       int      `json:"warning_count"`
	IssueCount         int      `json:"issue_count"`
}

type updatesResponse struct {
	Items      []updateSummary `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type changesResponse struct {
	Items      []changeInfo `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type changeInfo struct {
	UpdateID       string                       `json:"update_id"`
	FinishedAt     string                       `json:"finished_at"`
	Origin         string                       `json:"origin"`
	Scope          string                       `json:"scope"`
	RuleID         string                       `json:"rule_id"`
	RuleName       string                       `json:"rule_name"`
	Files          []state.ArtifactChangeRecord `json:"files"`
	Added          int                          `json:"added"`
	Removed        int                          `json:"removed"`
	AddedSamples   []string                     `json:"added_samples"`
	RemovedSamples []string                     `json:"removed_samples"`
}

func (s *Server) history() []state.UpdateHistoryRecord {
	return s.State.ListUpdateHistory(time.Duration(s.Config.Update.HistoryRetention), s.Config.Update.HistoryLimit, time.Now())
}

func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	records := s.history()
	start, ok := historyCursorStart(w, r.URL.Query().Get("cursor"), records)
	if !ok {
		return
	}
	end := start + limit
	if end > len(records) {
		end = len(records)
	}
	resp := updatesResponse{Items: make([]updateSummary, 0, end-start)}
	for _, record := range records[start:end] {
		resp.Items = append(resp.Items, summarizeUpdate(record))
	}
	if end < len(records) && end > start {
		resp.NextCursor = encodeCursor(records[end-1].ID)
	}
	writeJSON(w, http.StatusOK, resp)
}

func historyCursorStart(w http.ResponseWriter, cursor string, records []state.UpdateHistoryRecord) (int, bool) {
	if cursor == "" {
		return 0, true
	}
	id, err := decodeCursor(cursor)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "分页游标无效", map[string]any{})
		return 0, false
	}
	for i := range records {
		if records[i].ID == id {
			return i + 1, true
		}
	}
	writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "分页游标已失效", map[string]any{})
	return 0, false
}

func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	items := make([]changeInfo, 0)
	keys := make([]string, 0)
	for _, record := range s.history() {
		for _, change := range record.Changes {
			if len(change.Files) == 0 {
				continue
			}
			items = append(items, changeInfo{
				UpdateID: record.ID, FinishedAt: record.FinishedAt, Origin: record.Origin, Scope: record.Scope,
				RuleID: change.RuleID, RuleName: change.RuleName,
				Files: append([]state.ArtifactChangeRecord(nil), change.Files...),
				Added: change.Added, Removed: change.Removed,
				AddedSamples:   append([]string{}, change.AddedSamples...),
				RemovedSamples: append([]string{}, change.RemovedSamples...),
			})
			keys = append(keys, record.ID+"\x00"+change.RuleID)
		}
	}
	start := 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		decoded, err := decodeCursor(cursor)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "分页游标无效", map[string]any{})
			return
		}
		found := false
		for i, key := range keys {
			if key == decoded {
				start, found = i+1, true
				break
			}
		}
		if !found {
			writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "分页游标已失效", map[string]any{})
			return
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	resp := changesResponse{Items: append([]changeInfo{}, items[start:end]...)}
	if end < len(items) && end > start {
		resp.NextCursor = encodeCursor(keys[end-1])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateUpdate(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求需要 application/json", map[string]any{})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body createUpdateRequest
	if err := decoder.Decode(&body); err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{"reason": err.Error()})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求只能包含一个 JSON 对象", map[string]any{})
		return
	}
	job, err := s.updates.Start(updates.Request{Scope: body.Scope, RuleIDs: body.RuleIDs}, "web")
	if err != nil {
		writeUpdateStartError(w, err)
		return
	}
	record, _ := s.State.GetUpdateHistory(job.ID)
	writeJSON(w, http.StatusAccepted, summarizeUpdate(record))
}

func writeUpdateStartError(w http.ResponseWriter, err error) {
	var validation *updates.ValidationError
	if errors.As(err, &validation) {
		writeAPIError(w, http.StatusUnprocessableEntity, validation.Code, validation.Message, validation.Details)
		return
	}
	var conflict *updates.ConflictError
	if errors.As(err, &conflict) {
		writeAPIError(w, http.StatusConflict, "update_in_progress", conflict.Error(), map[string]any{"current_update_id": conflict.CurrentUpdateID})
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "update_start_failed", "启动更新失败", map[string]any{})
}

func (s *Server) handleCurrentUpdate(w http.ResponseWriter, _ *http.Request) {
	job := s.updates.Current()
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	record, ok := s.State.GetUpdateHistory(job.ID)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "update_history_missing", "当前任务记录缺失", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, summarizeUpdate(record))
}

func (s *Server) handleUpdateDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "updateID")
	record, ok := s.State.GetUpdateHistory(id)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "update_not_found", "更新记录不存在", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, normalizeUpdateRecord(record))
}

func (s *Server) handleCancelUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "updateID")
	if err := s.updates.Cancel(id); err != nil {
		writeAPIError(w, http.StatusConflict, "update_not_running", "更新任务已经结束或不存在", map[string]any{})
		return
	}
	record, _ := s.State.GetUpdateHistory(id)
	writeJSON(w, http.StatusAccepted, summarizeUpdate(record))
}

func (s *Server) handleUpdateEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "updateID")
	job := s.updates.Job(id)
	if job == nil {
		if _, ok := s.State.GetUpdateHistory(id); ok {
			writeAPIError(w, http.StatusGone, "update_events_expired", "更新事件已经过期，请读取任务详情", map[string]any{})
		} else {
			writeAPIError(w, http.StatusNotFound, "update_not_found", "更新记录不存在", map[string]any{})
		}
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "stream_unsupported", "服务端不支持事件流", map[string]any{})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	lastSequence, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	sendPending := func() error {
		for _, event := range job.EventsAfter(lastSequence) {
			data, _ := json.Marshal(event)
			if _, err := fmt.Fprintf(w, "id: %d\nevent: progress\ndata: %s\n\n", event.Sequence, data); err != nil {
				return err
			}
			lastSequence = event.Sequence
		}
		flusher.Flush()
		return nil
	}
	if sendPending() != nil {
		return
	}
	pulse := time.NewTicker(500 * time.Millisecond)
	defer pulse.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-job.Notify():
			if sendPending() != nil {
				return
			}
		case <-pulse.C:
			if sendPending() != nil {
				return
			}
		case <-job.Done():
			if sendPending() != nil {
				return
			}
			record, _ := s.State.GetUpdateHistory(id)
			data, _ := json.Marshal(normalizeUpdateRecord(record))
			if _, err := fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
			return
		case <-heartbeat.C:
			status, _, _ := job.Snapshot()
			if _, err := fmt.Fprintf(w, "event: ping\ndata: {\"status\":%q}\n\n", status); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func summarizeUpdate(record state.UpdateHistoryRecord) updateSummary {
	return updateSummary{
		ID: record.ID, Origin: record.Origin, Scope: record.Scope,
		RequestedRuleIDs: append([]string{}, record.RequestedRuleIDs...), Status: record.Status,
		StartedAt: record.StartedAt, FinishedAt: record.FinishedAt,
		RulesTotal: record.RulesTotal, RulesSucceeded: record.RulesSucceeded, RulesFailed: record.RulesFailed,
		ArtifactsProcessed: record.ArtifactsProcessed, PublishedArtifacts: record.PublishedArtifacts,
		ChangeCount: len(record.Changes), WarningCount: len(record.Warnings), IssueCount: len(record.Issues),
	}
}

func normalizeUpdateRecord(record state.UpdateHistoryRecord) state.UpdateHistoryRecord {
	record.RequestedRuleIDs = append([]string{}, record.RequestedRuleIDs...)
	record.EffectiveRuleIDs = append([]string{}, record.EffectiveRuleIDs...)
	record.Warnings = append([]string{}, record.Warnings...)
	record.Issues = append([]state.UpdateIssueRecord{}, record.Issues...)
	record.Changes = append([]state.RuleChangeRecord{}, record.Changes...)
	for i := range record.Changes {
		record.Changes[i].Files = append([]state.ArtifactChangeRecord{}, record.Changes[i].Files...)
		record.Changes[i].AddedSamples = append([]string{}, record.Changes[i].AddedSamples...)
		record.Changes[i].RemovedSamples = append([]string{}, record.Changes[i].RemovedSamples...)
	}
	return record
}
