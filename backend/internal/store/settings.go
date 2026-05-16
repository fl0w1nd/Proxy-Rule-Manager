package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// getKV reads a singleton settings row.
func (s *Store) getKV(ctx context.Context, key string, out any) error {
	var payload string
	if err := s.DB.QueryRowContext(ctx, `SELECT value_json FROM kv_settings WHERE key = ?`, key).Scan(&payload); err != nil {
		return err
	}
	return json.Unmarshal([]byte(payload), out)
}

// setKVLocked upserts a key while writeMu is already held by the caller.
func (s *Store) setKVLocked(ctx context.Context, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO kv_settings (key, value_json) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json`,
		key, string(payload))
	return err
}

// GetSyncSchedule returns the schedule, normalising defaults.
func (s *Store) GetSyncSchedule(ctx context.Context) (schema.SyncSchedule, error) {
	sch := schema.DefaultSyncSchedule()
	if err := s.getKV(ctx, "sync_schedule", &sch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sch, nil
		}
		return sch, err
	}
	if sch.Mode == "" {
		sch.Mode = "interval"
	}
	if sch.IntervalHours < 1 {
		sch.IntervalHours = 24
	}
	if sch.CronExpression == "" {
		sch.CronExpression = "0 0 * * *"
	}
	return sch, nil
}

// UpdateSyncSchedule merges updates into the persisted schedule. The whole
// read-modify-write cycle runs under the global write mutex so concurrent
// admin requests can't lose fields.
func (s *Store) UpdateSyncSchedule(ctx context.Context, updates schema.SyncSchedule) (schema.SyncSchedule, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	current, err := s.GetSyncSchedule(ctx)
	if err != nil {
		return current, err
	}
	if updates.Mode != "" {
		current.Mode = updates.Mode
	}
	if updates.IntervalHours > 0 {
		current.IntervalHours = updates.IntervalHours
	}
	if updates.CronExpression != "" {
		current.CronExpression = updates.CronExpression
	}
	if updates.LastScheduledSyncAt != nil {
		current.LastScheduledSyncAt = updates.LastScheduledSyncAt
	}
	if updates.NextSyncAt != nil {
		current.NextSyncAt = updates.NextSyncAt
	}
	if err := s.setKVLocked(ctx, "sync_schedule", current); err != nil {
		return current, err
	}
	return current, nil
}

// GetCdnSettings returns the cached CDN settings.
func (s *Store) GetCdnSettings(ctx context.Context) (schema.CdnSettings, error) {
	cdn := schema.DefaultCdnSettings()
	if err := s.getKV(ctx, "cdn_settings", &cdn); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cdn, nil
		}
		return cdn, err
	}
	if cdn.CustomHeaders == nil {
		cdn.CustomHeaders = []schema.CdnCustomHeader{}
	}
	if cdn.CacheMode == "" {
		cdn.CacheMode = "no-cache"
	}
	return cdn, nil
}

// UpdateCdnSettings merges fields into the existing CDN settings atomically.
func (s *Store) UpdateCdnSettings(ctx context.Context, updates schema.CdnSettings, present map[string]bool) (schema.CdnSettings, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	current, err := s.GetCdnSettings(ctx)
	if err != nil {
		return current, err
	}
	if present["enabled"] {
		current.Enabled = updates.Enabled
	}
	if present["cacheMode"] {
		current.CacheMode = updates.CacheMode
	}
	if present["staleIfErrorSeconds"] {
		current.StaleIfErrorSeconds = updates.StaleIfErrorSeconds
	}
	if present["customCacheControl"] {
		current.CustomCacheControl = updates.CustomCacheControl
	}
	if present["cloudflareCdnCacheControl"] {
		current.CloudflareCdnCacheControl = updates.CloudflareCdnCacheControl
	}
	if present["customHeaders"] {
		current.CustomHeaders = updates.CustomHeaders
		if current.CustomHeaders == nil {
			current.CustomHeaders = []schema.CdnCustomHeader{}
		}
	}
	if err := s.setKVLocked(ctx, "cdn_settings", current); err != nil {
		return current, err
	}
	return current, nil
}

// GetSystemSettings loads the runtime knobs (fetch / transformer / rate-limit).
// Missing fields are filled with built-in defaults so older databases (and
// migrated TS-era backups that lack the key entirely) round-trip cleanly.
func (s *Store) GetSystemSettings(ctx context.Context) (schema.SystemSettings, error) {
	out := schema.DefaultSystemSettings()
	if err := s.getKV(ctx, "system_settings", &out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return out, err
	}
	out.MergeDefaults()
	return out, nil
}

// SaveSystemSettings persists the full SystemSettings payload after validation.
// Callers must validate first; this function only enforces non-zero defaults
// so we can recover gracefully from partial writes.
func (s *Store) SaveSystemSettings(ctx context.Context, settings schema.SystemSettings) (schema.SystemSettings, error) {
	settings.MergeDefaults()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.setKVLocked(ctx, "system_settings", settings); err != nil {
		return settings, err
	}
	return settings, nil
}

// GetLastSyncInfo loads the last sync summary.
func (s *Store) GetLastSyncInfo(ctx context.Context) (schema.LastSyncInfo, error) {
	info := schema.DefaultLastSyncInfo()
	if err := s.getKV(ctx, "last_sync_info", &info); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		return info, err
	}
	return info, nil
}

// UpdateLastSyncInfo merges updates into the cached last-sync info atomically.
func (s *Store) UpdateLastSyncInfo(ctx context.Context, updates schema.LastSyncInfo, present map[string]bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	current, err := s.GetLastSyncInfo(ctx)
	if err != nil {
		return err
	}
	if present["lastFullSyncAt"] {
		current.LastFullSyncAt = updates.LastFullSyncAt
	}
	if present["lastPartialSyncAt"] {
		current.LastPartialSyncAt = updates.LastPartialSyncAt
	}
	if present["lastSuccessfulSyncAt"] {
		current.LastSuccessfulSyncAt = updates.LastSuccessfulSyncAt
	}
	if present["totalRulesCount"] {
		current.TotalRulesCount = updates.TotalRulesCount
	}
	if present["changedRulesCount"] {
		current.ChangedRulesCount = updates.ChangedRulesCount
	}
	if present["failedRulesCount"] {
		current.FailedRulesCount = updates.FailedRulesCount
	}
	if present["lastSyncDurationMs"] {
		current.LastSyncDurationMs = updates.LastSyncDurationMs
	}
	return s.setKVLocked(ctx, "last_sync_info", current)
}

// GetDailyStats returns stats for a date (zero value if missing).
func (s *Store) GetDailyStats(ctx context.Context, date string) (schema.DailyStats, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT date, sync_count, blob_write_count, rules_changed, total_rules_processed, failed_sources FROM daily_stats WHERE date = ?`, date)
	var stats schema.DailyStats
	if err := row.Scan(&stats.Date, &stats.SyncCount, &stats.BlobWriteCount, &stats.RulesChanged, &stats.TotalRulesProcessed, &stats.FailedSources); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schema.DailyStats{Date: date}, nil
		}
		return stats, err
	}
	return stats, nil
}

// IncrementDailyStats atomically bumps the counters for a date.
func (s *Store) IncrementDailyStats(ctx context.Context, date string, delta schema.DailyStats) error {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO daily_stats (date, sync_count, blob_write_count, rules_changed, total_rules_processed, failed_sources)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(date) DO UPDATE SET
			   sync_count = sync_count + excluded.sync_count,
			   blob_write_count = blob_write_count + excluded.blob_write_count,
			   rules_changed = rules_changed + excluded.rules_changed,
			   total_rules_processed = total_rules_processed + excluded.total_rules_processed,
			   failed_sources = failed_sources + excluded.failed_sources`,
			date, delta.SyncCount, delta.BlobWriteCount, delta.RulesChanged, delta.TotalRulesProcessed, delta.FailedSources)
		return err
	})
}
