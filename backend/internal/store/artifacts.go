package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// GetArtifactMeta returns the metadata for (ruleName, client) or nil.
func (s *Store) GetArtifactMeta(ctx context.Context, ruleName, client string) (*schema.ArtifactMeta, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT rule_name, client, last_hash, last_updated_at, blob_path, blob_url, size_bytes,
		        last_attempted_at, last_attempt_status, last_attempt_error
		 FROM artifacts WHERE rule_name = ? AND client = ?`, ruleName, client)
	return scanArtifact(row)
}

// SaveArtifactMetas upserts a batch of artifact rows in a single transaction.
// It only writes the "successful publish" fields (hash, blob, size, last_updated_at)
// and treats the row as a successful attempt; the per-attempt status is set to
// "success" so dashboards can stop showing stale failure flags after recovery.
func (s *Store) SaveArtifactMetas(ctx context.Context, metas []schema.ArtifactMeta) error {
	if len(metas) == 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO artifacts (rule_name, client, last_hash, last_updated_at, blob_path, blob_url, size_bytes,
			                        last_attempted_at, last_attempt_status, last_attempt_error)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'success', '')
			 ON CONFLICT(rule_name, client) DO UPDATE SET
			   last_hash = excluded.last_hash,
			   last_updated_at = excluded.last_updated_at,
			   blob_path = excluded.blob_path,
			   blob_url = excluded.blob_url,
			   size_bytes = excluded.size_bytes,
			   last_attempted_at = excluded.last_attempted_at,
			   last_attempt_status = 'success',
			   last_attempt_error = ''`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, m := range metas {
			var sz sql.NullInt64
			if m.SizeBytes != nil {
				sz = sql.NullInt64{Int64: *m.SizeBytes, Valid: true}
			}
			var url sql.NullString
			if m.BlobURL != "" {
				url = sql.NullString{String: m.BlobURL, Valid: true}
			}
			attemptedAt := m.LastAttemptedAt
			if attemptedAt == "" {
				attemptedAt = m.LastUpdatedAt
			}
			if _, err := stmt.ExecContext(ctx, m.RuleName, m.Client, m.LastHash, m.LastUpdatedAt, m.BlobPath, url, sz, attemptedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// ArtifactAttempt records a single (ruleName, client) sync attempt outcome.
// Use this to mark failures or to refresh the attempt timestamp without
// touching the published blob (the engine still calls SaveArtifactMetas for
// successful writes, which already updates these columns atomically).
type ArtifactAttempt struct {
	RuleName    string
	Client      string
	AttemptedAt string
	Status      string
	Error       string
}

// RecordArtifactAttempts updates only the per-attempt columns. Rows that do
// not yet exist are created with empty hash/path placeholders so a stale
// record cannot accidentally surface as a "successful publish" elsewhere.
func (s *Store) RecordArtifactAttempts(ctx context.Context, items []ArtifactAttempt) error {
	if len(items) == 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO artifacts (rule_name, client, last_hash, last_updated_at, blob_path, blob_url, size_bytes,
			                        last_attempted_at, last_attempt_status, last_attempt_error)
			 VALUES (?, ?, '', '', '', NULL, NULL, ?, ?, ?)
			 ON CONFLICT(rule_name, client) DO UPDATE SET
			   last_attempted_at = excluded.last_attempted_at,
			   last_attempt_status = excluded.last_attempt_status,
			   last_attempt_error = excluded.last_attempt_error`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, it := range items {
			if _, err := stmt.ExecContext(ctx, it.RuleName, it.Client, it.AttemptedAt, it.Status, it.Error); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteArtifactMeta removes a single (rule, client) row.
func (s *Store) DeleteArtifactMeta(ctx context.Context, ruleName, client string) error {
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx, `DELETE FROM artifacts WHERE rule_name = ? AND client = ?`, ruleName, client)
		return err
	})
}

// DeleteArtifactsForRule removes all artifact rows for a given rule.
func (s *Store) DeleteArtifactsForRule(ctx context.Context, ruleName string) error {
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx, `DELETE FROM artifacts WHERE rule_name = ?`, ruleName)
		return err
	})
}

// DeleteArtifactMetas removes the listed pairs.
func (s *Store) DeleteArtifactMetas(ctx context.Context, entries []ArtifactKey) error {
	if len(entries) == 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `DELETE FROM artifacts WHERE rule_name = ? AND client = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range entries {
			if _, err := stmt.ExecContext(ctx, e.RuleName, e.Client); err != nil {
				return err
			}
		}
		return nil
	})
}

// ArtifactKey identifies a single (rule, client) pair.
type ArtifactKey struct {
	RuleName string
	Client   string
}

// GetAllArtifactMetas returns every artifact row.
func (s *Store) GetAllArtifactMetas(ctx context.Context) ([]schema.ArtifactMeta, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT rule_name, client, last_hash, last_updated_at, blob_path, blob_url, size_bytes,
		        last_attempted_at, last_attempt_status, last_attempt_error FROM artifacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.ArtifactMeta
	for rows.Next() {
		m, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		if m != nil {
			out = append(out, *m)
		}
	}
	return out, rows.Err()
}

// RenameRuleArtifacts cascades a rename across artifact rows.
func (s *Store) RenameRuleArtifacts(ctx context.Context, oldName, newName string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE artifacts SET rule_name = ?, blob_path = REPLACE(blob_path, ?, ?) WHERE rule_name = ?`,
			newName, "/"+oldName+".list", "/"+newName+".list", oldName,
		); err != nil {
			return err
		}
		return nil
	})
}

func scanArtifact(row rowScanner) (*schema.ArtifactMeta, error) {
	var m schema.ArtifactMeta
	var sz sql.NullInt64
	var url sql.NullString
	if err := row.Scan(&m.RuleName, &m.Client, &m.LastHash, &m.LastUpdatedAt, &m.BlobPath, &url, &sz,
		&m.LastAttemptedAt, &m.LastAttemptStatus, &m.LastAttemptError); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if sz.Valid {
		v := sz.Int64
		m.SizeBytes = &v
	}
	if url.Valid {
		m.BlobURL = url.String
	}
	return &m, nil
}
