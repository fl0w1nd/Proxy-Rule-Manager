// Package state provides JSON-based update state persistence. State tracks
// per-artifact content hashes and IR snapshots for diff computation.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// UpdateState is the JSON-serialized state file.
type UpdateState struct {
	LastCheck      string                       `json:"last_check,omitempty"`
	Artifacts      map[string]map[string]string `json:"artifacts,omitempty"` // rule ID -> client ID -> hash
	RuleUpdates    map[string]UpdateRecord      `json:"rule_updates,omitempty"`
	GeositeUpdates map[string]string            `json:"geosite_updates,omitempty"`
	GeositeChecks  map[string]string            `json:"geosite_checks,omitempty"`
	UpdateHistory  []UpdateHistoryRecord        `json:"update_history,omitempty"`
}

// UpdateHistoryRecord is one persisted update execution.
type UpdateHistoryRecord struct {
	ID                 string              `json:"id"`
	Origin             string              `json:"origin"`
	Scope              string              `json:"scope"`
	RequestedRuleIDs   []string            `json:"requested_rule_ids"`
	EffectiveRuleIDs   []string            `json:"effective_rule_ids"`
	Status             string              `json:"status"`
	StartedAt          string              `json:"started_at"`
	FinishedAt         string              `json:"finished_at,omitempty"`
	RulesTotal         int                 `json:"rules_total"`
	RulesSucceeded     int                 `json:"rules_succeeded"`
	RulesFailed        int                 `json:"rules_failed"`
	ArtifactsProcessed int                 `json:"artifacts_processed"`
	PublishedArtifacts int                 `json:"published_artifacts"`
	Warnings           []string            `json:"warnings"`
	Issues             []UpdateIssueRecord `json:"issues"`
	Changes            []RuleChangeRecord  `json:"changes"`
}

// UpdateIssueRecord is one actionable update issue.
type UpdateIssueRecord struct {
	Stage   string `json:"stage,omitempty"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message"`
}

// RuleChangeRecord captures one persisted logical rule diff.
type RuleChangeRecord struct {
	RuleID         string   `json:"rule_id"`
	RuleName       string   `json:"rule_name"`
	Added          int      `json:"added"`
	Removed        int      `json:"removed"`
	AddedSamples   []string `json:"added_samples"`
	RemovedSamples []string `json:"removed_samples"`
}

// UpdateRecord tracks the current content version and the latest check.
type UpdateRecord struct {
	Result     string `json:"result"`
	CheckedAt  string `json:"checked_at"`
	VersionAt  string `json:"version_at,omitempty"`
	EntryCount *int   `json:"entry_count,omitempty"`
}

const (
	RuleUpdated   = "updated"
	RuleUnchanged = "unchanged"
	RuleFailed    = "failed"
	RuleCancelled = "cancelled"

	GeositeUpdated   = "updated"
	GeositeUnchanged = "unchanged"
	GeositeFailed    = "failed"
)

// Store manages state persistence under dataDir/.state/.
type Store struct {
	mu      sync.RWMutex
	dataDir string
	state   UpdateState
}

// Open loads or initializes state from the given data directory.
func Open(dataDir string) (*Store, error) {
	stateDir := filepath.Join(dataDir, ".state")
	if err := util.EnsureDir(stateDir); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	if err := util.EnsureDir(filepath.Join(stateDir, "snapshots")); err != nil {
		return nil, fmt.Errorf("create snapshots dir: %w", err)
	}

	s := &Store{dataDir: dataDir}
	stateFile := filepath.Join(stateDir, "update.json")
	legacyErrorHistory := false
	if data, err := os.ReadFile(stateFile); err == nil {
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, fmt.Errorf("parse state file: %w", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err == nil {
			_, legacyErrorHistory = fields["error_history"]
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read state file: %w", err)
	}
	if s.state.Artifacts == nil {
		s.state.Artifacts = make(map[string]map[string]string)
	}
	if s.state.RuleUpdates == nil {
		s.state.RuleUpdates = make(map[string]UpdateRecord)
	}
	if s.state.GeositeUpdates == nil {
		s.state.GeositeUpdates = make(map[string]string)
	}
	if s.state.GeositeChecks == nil {
		s.state.GeositeChecks = make(map[string]string)
	}
	if legacyErrorHistory {
		if err := s.Save(); err != nil {
			return nil, fmt.Errorf("clear legacy error history: %w", err)
		}
	}
	return s, nil
}

// GetArtifactHash returns the stored hash for a rule/client artifact.
func (s *Store) GetArtifactHash(ruleID, clientID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.state.Artifacts[ruleID]; ok {
		return m[clientID]
	}
	return ""
}

// SetArtifactHash sets the stored hash for a rule/client artifact.
func (s *Store) SetArtifactHash(ruleID, clientID, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Artifacts[ruleID] == nil {
		s.state.Artifacts[ruleID] = make(map[string]string)
	}
	s.state.Artifacts[ruleID][clientID] = hash
}

// DeleteArtifactHash removes the stored hash for one rule/client artifact.
func (s *Store) DeleteArtifactHash(ruleID, clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clients, ok := s.state.Artifacts[ruleID]
	if !ok {
		return
	}
	delete(clients, clientID)
	if len(clients) == 0 {
		delete(s.state.Artifacts, ruleID)
	}
}

// ReconcileRuleArtifacts retains only the expected output targets for the
// supplied rules and leaves all other rule state untouched.
func (s *Store) ReconcileRuleArtifacts(expected map[string]map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ruleID, expectedTargets := range expected {
		targets, ok := s.state.Artifacts[ruleID]
		if !ok {
			continue
		}
		for targetID := range targets {
			if _, keep := expectedTargets[targetID]; !keep {
				delete(targets, targetID)
			}
		}
		if len(targets) == 0 {
			delete(s.state.Artifacts, ruleID)
		}
	}
}

// SetLastCheck records when the latest update check finished.
func (s *Store) SetLastCheck(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LastCheck = util.FormatISO(t)
}

// LastCheck returns the latest completed check time, if available.
func (s *Store) LastCheck() (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.LastCheck == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s.state.LastCheck)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// SetRuleCheck records a check outcome. The first successful check establishes
// a version baseline; later content changes advance it.
func (s *Store) SetRuleCheck(ruleID, result string, checkedAt time.Time, versionChanged bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.state.RuleUpdates[ruleID]
	record.Result = result
	record.CheckedAt = util.FormatISO(checkedAt)
	firstSuccessfulCheck := record.VersionAt == "" && (result == RuleUpdated || result == RuleUnchanged)
	if versionChanged || firstSuccessfulCheck {
		record.VersionAt = util.FormatISO(checkedAt)
	}
	s.state.RuleUpdates[ruleID] = record
}

// RuleUpdate returns the latest check outcome and content version.
func (s *Store) RuleUpdate(ruleID string) (string, time.Time, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.state.RuleUpdates[ruleID]
	if !ok {
		return "", time.Time{}, time.Time{}, false
	}
	checkedAt, err := time.Parse(time.RFC3339, record.CheckedAt)
	if err != nil {
		return "", time.Time{}, time.Time{}, false
	}
	var versionAt time.Time
	if record.VersionAt != "" {
		versionAt, err = time.Parse(time.RFC3339, record.VersionAt)
		if err != nil {
			return "", time.Time{}, time.Time{}, false
		}
	}
	return record.Result, checkedAt, versionAt, true
}

// SetRuleEntryCount records the latest successful logical entry count.
func (s *Store) SetRuleEntryCount(ruleID string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.state.RuleUpdates[ruleID]
	record.EntryCount = new(int)
	*record.EntryCount = count
	s.state.RuleUpdates[ruleID] = record
}

// RuleEntryCount returns the persisted logical entry count when known.
func (s *Store) RuleEntryCount(ruleID string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.state.RuleUpdates[ruleID]
	if !ok || record.EntryCount == nil {
		return 0, false
	}
	return *record.EntryCount, true
}

// BackfillEntryCounts reads legacy snapshots once to populate missing counts.
func (s *Store) BackfillEntryCounts(ruleIDs []string) (bool, error) {
	changed := false
	for _, ruleID := range ruleIDs {
		if _, ok := s.RuleEntryCount(ruleID); ok {
			continue
		}
		entries, exists, err := s.LoadSnapshotIfExists(ruleID)
		if err != nil {
			return changed, err
		}
		if !exists {
			continue
		}
		s.SetRuleEntryCount(ruleID, len(entries))
		changed = true
	}
	return changed, nil
}

// SetGeositeUpdate records the latest version-fetch outcome for one provider.
func (s *Store) SetGeositeUpdate(provider, result string, checkedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.GeositeUpdates[provider] = result
	s.state.GeositeChecks[provider] = util.FormatISO(checkedAt)
}

// GeositeUpdate returns the latest version-fetch outcome for one provider.
func (s *Store) GeositeUpdate(provider string) (string, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.state.GeositeUpdates[provider]
	if !ok {
		return "", time.Time{}, false
	}
	// Preserve state written before geosite gained updated/unchanged outcomes.
	if result == "success" {
		result = GeositeUpdated
	}
	checkedAtValue := s.state.GeositeChecks[provider]
	if checkedAtValue == "" {
		checkedAtValue = s.state.LastCheck
	}
	checkedAt, _ := time.Parse(time.RFC3339, checkedAtValue)
	return result, checkedAt, true
}

// PutUpdateHistory inserts or replaces one update record and applies retention.
func (s *Store) PutUpdateHistory(record UpdateHistoryRecord, retention time.Duration, limit int, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.state.UpdateHistory {
		if s.state.UpdateHistory[i].ID == record.ID {
			s.state.UpdateHistory[i] = cloneUpdateHistoryRecord(record)
			replaced = true
			break
		}
	}
	if !replaced {
		s.state.UpdateHistory = append(s.state.UpdateHistory, cloneUpdateHistoryRecord(record))
	}
	s.state.UpdateHistory = retainedUpdateHistory(s.state.UpdateHistory, retention, limit, now)
}

// GetUpdateHistory returns one persisted update record.
func (s *Store) GetUpdateHistory(id string) (UpdateHistoryRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.state.UpdateHistory {
		if s.state.UpdateHistory[i].ID == id {
			return cloneUpdateHistoryRecord(s.state.UpdateHistory[i]), true
		}
	}
	return UpdateHistoryRecord{}, false
}

// DeleteUpdateHistory removes a record that could not be durably created.
func (s *Store) DeleteUpdateHistory(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.UpdateHistory {
		if s.state.UpdateHistory[i].ID == id {
			s.state.UpdateHistory = append(s.state.UpdateHistory[:i], s.state.UpdateHistory[i+1:]...)
			return
		}
	}
}

// ListUpdateHistory returns retained records newest first.
func (s *Store) ListUpdateHistory(retention time.Duration, limit int, now time.Time) []UpdateHistoryRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.UpdateHistory = retainedUpdateHistory(s.state.UpdateHistory, retention, limit, now)
	out := make([]UpdateHistoryRecord, len(s.state.UpdateHistory))
	for i := range s.state.UpdateHistory {
		out[len(out)-1-i] = cloneUpdateHistoryRecord(s.state.UpdateHistory[i])
	}
	return out
}

// PruneUpdateHistory applies time and count retention and reports a change.
func (s *Store) PruneUpdateHistory(retention time.Duration, limit int, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.state.UpdateHistory)
	s.state.UpdateHistory = retainedUpdateHistory(s.state.UpdateHistory, retention, limit, now)
	return len(s.state.UpdateHistory) != before
}

// MarkInterruptedUpdates converts unfinished records left by a previous process.
func (s *Store) MarkInterruptedUpdates(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.state.UpdateHistory {
		record := &s.state.UpdateHistory[i]
		if record.Status != "running" && record.Status != "cancelling" {
			continue
		}
		record.Status = "interrupted"
		record.FinishedAt = util.FormatISO(now)
		changed = true
	}
	return changed
}

func retainedUpdateHistory(records []UpdateHistoryRecord, retention time.Duration, limit int, now time.Time) []UpdateHistoryRecord {
	if limit < 1 {
		limit = 1
	}
	cutoff := now.Add(-retention)
	active := make([]UpdateHistoryRecord, 0, 1)
	completed := make([]UpdateHistoryRecord, 0, len(records))
	for _, record := range records {
		if record.Status == "running" || record.Status == "cancelling" {
			active = append(active, record)
			continue
		}
		when := record.FinishedAt
		if when == "" {
			when = record.StartedAt
		}
		parsed, err := time.Parse(time.RFC3339, when)
		if err == nil && !parsed.Before(cutoff) {
			completed = append(completed, record)
		}
	}
	if len(completed) > limit {
		completed = completed[len(completed)-limit:]
	}
	return append(completed, active...)
}

func cloneUpdateHistoryRecord(record UpdateHistoryRecord) UpdateHistoryRecord {
	record.RequestedRuleIDs = append([]string(nil), record.RequestedRuleIDs...)
	record.EffectiveRuleIDs = append([]string(nil), record.EffectiveRuleIDs...)
	record.Warnings = append([]string(nil), record.Warnings...)
	record.Issues = append([]UpdateIssueRecord(nil), record.Issues...)
	record.Changes = append([]RuleChangeRecord(nil), record.Changes...)
	for i := range record.Changes {
		record.Changes[i].AddedSamples = append([]string(nil), record.Changes[i].AddedSamples...)
		record.Changes[i].RemovedSamples = append([]string(nil), record.Changes[i].RemovedSamples...)
	}
	return record
}

// Save writes the state to disk atomically.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.MarshalIndent(&s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return util.AtomicWriteFile(filepath.Join(s.dataDir, ".state", "update.json"), data)
}

// SaveSnapshot writes the IR entry snapshot for a rule ID (used for diff).
func (s *Store) SaveSnapshot(ruleID string, entries []ir.Entry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	path, err := s.snapshotPath(ruleID)
	if err != nil {
		return err
	}
	return util.AtomicWriteFile(path, data)
}

// LoadSnapshot loads the previous IR entry snapshot for a rule ID.
func (s *Store) LoadSnapshot(ruleID string) ([]ir.Entry, error) {
	entries, _, err := s.LoadSnapshotIfExists(ruleID)
	return entries, err
}

// LoadSnapshotIfExists loads a snapshot and reports whether its file exists.
// The existence bit distinguishes a valid empty snapshot from a missing one.
func (s *Store) LoadSnapshotIfExists(ruleID string) ([]ir.Entry, bool, error) {
	path, err := s.snapshotPath(ruleID)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var entries []ir.Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, true, fmt.Errorf("parse snapshot: %w", err)
	}
	return entries, true, nil
}

func (s *Store) snapshotPath(ruleID string) (string, error) {
	if err := util.EnsureSafeSegment(ruleID, "rule id"); err != nil {
		return "", err
	}
	return util.JoinInside(filepath.Join(s.dataDir, ".state", "snapshots"), ruleID+".json")
}

// Reconcile removes hashes and snapshots that are absent from the latest
// successful full-update manifest.
func (s *Store) Reconcile(expectedArtifacts map[string]map[string]struct{}, expectedRules map[string]struct{}) error {
	s.mu.Lock()
	for ruleID, clients := range s.state.Artifacts {
		expectedClients, ok := expectedArtifacts[ruleID]
		if !ok {
			delete(s.state.Artifacts, ruleID)
			continue
		}
		for clientID := range clients {
			if _, ok := expectedClients[clientID]; !ok {
				delete(clients, clientID)
			}
		}
		if len(clients) == 0 {
			delete(s.state.Artifacts, ruleID)
		}
	}
	for ruleID := range s.state.RuleUpdates {
		if _, ok := expectedRules[ruleID]; !ok {
			delete(s.state.RuleUpdates, ruleID)
		}
	}
	s.mu.Unlock()

	snapshotDir := filepath.Join(s.dataDir, ".state", "snapshots")
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		ruleID := strings.TrimSuffix(entry.Name(), ".json")
		if _, ok := expectedRules[ruleID]; ok {
			continue
		}
		path, err := util.JoinInside(snapshotDir, entry.Name())
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
