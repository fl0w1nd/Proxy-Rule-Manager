package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// RetentionDays mirrors the previous behaviour (30 days).
const RetentionDays = 30

// ChangeRecordInput is what the engine writes.
type ChangeRecordInput struct {
	ID         string
	Timestamp  string
	RuleName   string
	Client     string
	ChangeType string
	SizeBytes  *int64
	Diff       string
}

// RecordRuleFileChanges inserts a batch of change records (single tx).
func (s *Store) RecordRuleFileChanges(ctx context.Context, items []ChangeRecordInput) error {
	if len(items) == 0 {
		return nil
	}
	if err := s.PruneActivity(ctx); err != nil {
		return err
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO change_records (id, date, timestamp, rule_name, client, change_type, size_bytes, diff)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range items {
			date := dateOf(r.Timestamp)
			var sz sql.NullInt64
			if r.SizeBytes != nil {
				sz = sql.NullInt64{Int64: *r.SizeBytes, Valid: true}
			}
			if _, err := stmt.ExecContext(ctx, r.ID, date, r.Timestamp, r.RuleName, r.Client, r.ChangeType, sz, r.Diff); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordFailureRecords inserts a batch of failure records.
func (s *Store) RecordFailureRecords(ctx context.Context, items []schema.FailureRecord) error {
	if len(items) == 0 {
		return nil
	}
	if err := s.PruneActivity(ctx); err != nil {
		return err
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO failure_records (id, date, timestamp, rule_name, client, source, message, stage, job_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range items {
			date := dateOf(r.Timestamp)
			var client, source, jobID sql.NullString
			if r.Client != "" {
				client = sql.NullString{String: r.Client, Valid: true}
			}
			if r.Source != "" {
				source = sql.NullString{String: r.Source, Valid: true}
			}
			if r.JobID != "" {
				jobID = sql.NullString{String: r.JobID, Valid: true}
			}
			if _, err := stmt.ExecContext(ctx, r.ID, date, r.Timestamp, r.RuleName, client, source, r.Message, r.Stage, jobID); err != nil {
				return err
			}
		}
		return nil
	})
}

// PruneActivity removes records older than RetentionDays.
func (s *Store) PruneActivity(ctx context.Context) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -(RetentionDays - 1)).Format("2006-01-02")
	return s.withWriteLock(func() error {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM change_records WHERE date < ?`, cutoff); err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM failure_records WHERE date < ?`, cutoff); err != nil {
			return err
		}
		return nil
	})
}

// Geosite naming conventions surfaced in records:
//
//   - "geosite_{provider}_{list}"   — per-list pseudo rule name. High volume,
//     intentionally hidden from the activity UI to avoid drowning out the
//     hundreds of regular rules an admin actually manages.
//   - "geosite:{provider}"          — provider-level outage marker. Sparse
//     and high-signal; surfaced in failure-records reads.
//
// SQL note: in LIKE, underscore is a wildcard, so `'geosite_%'` would also
// match `'geosite:foo'`. We escape it with `\_` + ESCAPE so only the per-list
// pattern is filtered out, leaving provider-level rows visible.
const (
	filterPerListGeosite = `rule_name NOT LIKE 'geosite\_%' ESCAPE '\'`
	filterAllGeosite     = filterPerListGeosite + ` AND rule_name NOT LIKE 'geosite:%'`
)

// ListChangeRecords returns a paginated set, filtering by date/client/days.
//
// Change records use filterAllGeosite: the engine never writes geosite changes
// (per-list noise OR provider-level), so this is purely defensive — keeps the
// activity feed clean even if a future code path accidentally emits one.
func (s *Store) ListChangeRecords(ctx context.Context, date string, page, pageSize int, client string, days int) (schema.ActivityList[schema.ChangeRecordSummary], error) {
	if err := s.PruneActivity(ctx); err != nil {
		return schema.ActivityList[schema.ChangeRecordSummary]{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	args := []any{}
	where := []string{filterAllGeosite}
	if date != "" {
		where = append(where, `date = ?`)
		args = append(args, date)
	} else if days > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
		where = append(where, `date >= ?`)
		args = append(args, cutoff)
	}
	if client != "" {
		where = append(where, `client = ?`)
		args = append(args, client)
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM change_records WHERE "+whereClause, args...).Scan(&total); err != nil {
		return schema.ActivityList[schema.ChangeRecordSummary]{}, err
	}

	offset := (page - 1) * pageSize
	rows, err := s.DB.QueryContext(ctx,
		"SELECT id, date, timestamp, rule_name, client, change_type, size_bytes FROM change_records WHERE "+whereClause+
			" ORDER BY timestamp DESC, id ASC LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...)
	if err != nil {
		return schema.ActivityList[schema.ChangeRecordSummary]{}, err
	}
	defer rows.Close()
	items := make([]schema.ChangeRecordSummary, 0, pageSize)
	for rows.Next() {
		var r schema.ChangeRecordSummary
		var sz sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Date, &r.Timestamp, &r.RuleName, &r.Client, &r.ChangeType, &sz); err != nil {
			return schema.ActivityList[schema.ChangeRecordSummary]{}, err
		}
		r.Timestamp = normalizeISO(r.Timestamp)
		if sz.Valid {
			v := sz.Int64
			r.SizeBytes = &v
		}
		r.FileName = buildChangeFileName(r)
		items = append(items, r)
	}
	return schema.ActivityList[schema.ChangeRecordSummary]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListFailureRecords mirrors ListChangeRecords for failures.
//
// Failure records use filterPerListGeosite — we keep "geosite:{provider}"
// rows so admins can see when a whole provider stops fetching, while still
// hiding the per-list "geosite_{provider}_{list}" rows that the engine
// (defensively) might leave behind from artifact-write failures.
func (s *Store) ListFailureRecords(ctx context.Context, date string, page, pageSize int, client string, days int) (schema.ActivityList[schema.FailureRecord], error) {
	if err := s.PruneActivity(ctx); err != nil {
		return schema.ActivityList[schema.FailureRecord]{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	args := []any{}
	where := []string{filterPerListGeosite}
	if date != "" {
		where = append(where, `date = ?`)
		args = append(args, date)
	} else if days > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
		where = append(where, `date >= ?`)
		args = append(args, cutoff)
	}
	if client != "" {
		where = append(where, `client = ?`)
		args = append(args, client)
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM failure_records WHERE "+whereClause, args...).Scan(&total); err != nil {
		return schema.ActivityList[schema.FailureRecord]{}, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.DB.QueryContext(ctx,
		"SELECT id, timestamp, rule_name, client, source, message, stage, job_id FROM failure_records WHERE "+whereClause+
			" ORDER BY timestamp DESC, id ASC LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...)
	if err != nil {
		return schema.ActivityList[schema.FailureRecord]{}, err
	}
	defer rows.Close()
	items := make([]schema.FailureRecord, 0, pageSize)
	for rows.Next() {
		var r schema.FailureRecord
		var client, source, jobID sql.NullString
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.RuleName, &client, &source, &r.Message, &r.Stage, &jobID); err != nil {
			return schema.ActivityList[schema.FailureRecord]{}, err
		}
		if client.Valid {
			r.Client = client.String
		}
		if source.Valid {
			r.Source = source.String
		}
		if jobID.Valid {
			r.JobID = jobID.String
		}
		r.Timestamp = normalizeISO(r.Timestamp)
		items = append(items, r)
	}
	return schema.ActivityList[schema.FailureRecord]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// FailingSource is a per-rule aggregation of recent failures, suitable for
// the dashboard "本周失败源 Top N" widget. Counts every failure_records row
// that's not per-list geosite noise; provider-level "geosite:{provider}"
// outages do show up here so admins can spot whole-provider problems.
type FailingSource struct {
	RuleName    string `json:"ruleName"`
	Count       int64  `json:"count"`
	LastTime    string `json:"lastTimestamp"`
	LastMessage string `json:"lastMessage"`
	LastStage   string `json:"lastStage,omitempty"`
}

// ListFailingSources returns up to `limit` rule names ranked by failure count
// over the past `days` days, ties broken by most-recent failure first. The
// caller is responsible for clamping reasonable bounds — we still cap days
// at the existing retention window so we never scan more than RetentionDays
// worth of rows.
func (s *Store) ListFailingSources(ctx context.Context, days, limit int) ([]FailingSource, error) {
	if days <= 0 {
		days = 7
	}
	if days > RetentionDays {
		days = RetentionDays
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	// Strategy: aggregate counts per rule_name in one pass, then for each
	// of the top-N rules read the most-recent row's fields. This keeps the
	// query simple (no window functions) and bounds per-row work to the
	// final N. RetentionDays is small enough that the aggregate scan is
	// always cheap.
	rows, err := s.DB.QueryContext(ctx,
		`SELECT rule_name, COUNT(*) AS n, MAX(timestamp) AS last_ts FROM failure_records
		 WHERE date >= ? AND `+filterPerListGeosite+`
		 GROUP BY rule_name
		 ORDER BY n DESC, last_ts DESC
		 LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type head struct {
		name string
		n    int64
		ts   string
	}
	var heads []head
	for rows.Next() {
		var h head
		if err := rows.Scan(&h.name, &h.n, &h.ts); err != nil {
			return nil, err
		}
		heads = append(heads, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]FailingSource, 0, len(heads))
	for _, h := range heads {
		var msg, stage string
		err := s.DB.QueryRowContext(ctx,
			`SELECT message, stage FROM failure_records
			 WHERE rule_name = ? AND timestamp = ?
			 LIMIT 1`, h.name, h.ts).Scan(&msg, &stage)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		out = append(out, FailingSource{
			RuleName:    h.name,
			Count:       h.n,
			LastTime:    normalizeISO(h.ts),
			LastMessage: msg,
			LastStage:   stage,
		})
	}
	return out, nil
}

// GetChangeDiff fetches a single diff body by file name (id with .diff suffix).
func (s *Store) GetChangeDiff(ctx context.Context, date, fileName string) (string, error) {
	if !strings.HasSuffix(fileName, ".diff") {
		return "", nil
	}
	if unescaped, err := url.PathUnescape(fileName); err == nil {
		fileName = unescaped
	}
	id := changeFileID(fileName)
	var diff sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT diff FROM change_records WHERE id = ? AND date = ?`, id, date).Scan(&diff)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !diff.Valid {
		return "", nil
	}
	return diff.String, nil
}

// CountChangeRecords returns the number of distinct (rule_name, client) pairs for a given date,
// matching the TS original which used a Set<string> keyed on `${ruleName}:${client}`.
func (s *Store) CountChangeRecords(ctx context.Context, date string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT rule_name || ':' || client) FROM change_records WHERE date = ? AND `+filterAllGeosite, date).Scan(&n)
	return n, err
}

// CountFailureRecords returns the count for a given date, including provider-
// level geosite outages (a single number per failed provider) so the dashboard
// "今日异常" KPI doesn't undercount when geosite is down.
func (s *Store) CountFailureRecords(ctx context.Context, date string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM failure_records WHERE date = ? AND `+filterPerListGeosite, date).Scan(&n)
	return n, err
}

// ListActivityDates returns sorted distinct dates that have any records.
//
// We include geosite provider-level failures here so the date filter in the
// activity UI surfaces days that ONLY had a geosite outage (otherwise such
// days would silently disappear from the dropdown).
func (s *Store) ListActivityDates(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT DISTINCT date FROM (
			SELECT date FROM change_records WHERE `+filterAllGeosite+`
			UNION SELECT date FROM failure_records WHERE `+filterPerListGeosite+`
		) ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ClearActivity deletes all activity rows.
func (s *Store) ClearActivity(ctx context.Context) error {
	return s.withWriteLock(func() error {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM change_records`); err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM failure_records`); err != nil {
			return err
		}
		return nil
	})
}

func dateOf(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}
	return time.Now().UTC().Format("2006-01-02")
}

func normalizeISO(ts string) string {
	if t, err := util.ParseISO(ts); err == nil {
		return util.FormatISO(t)
	}
	return ts
}

func buildChangeFileName(r schema.ChangeRecordSummary) string {
	t, err := util.ParseISO(r.Timestamp)
	if err != nil {
		t = time.Now().UTC()
	}
	size := int64(0)
	if r.SizeBytes != nil {
		size = *r.SizeBytes
	}
	return fmt.Sprintf("%d@%s@%s@%s@%d@%s.diff",
		t.UnixMilli(),
		url.PathEscape(r.RuleName),
		url.PathEscape(r.Client),
		r.ChangeType,
		size,
		r.ID,
	)
}

func changeFileID(fileName string) string {
	base := strings.TrimSuffix(fileName, ".diff")
	parts := strings.Split(base, "@")
	if len(parts) == 6 {
		return parts[5]
	}
	return base
}
