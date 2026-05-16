package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// GetAllBans returns every ban row.
func (s *Store) GetAllBans(ctx context.Context) ([]schema.BanRecord, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT ip, reason, banned_at, expires_at, fail_count FROM bans ORDER BY banned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.BanRecord
	for rows.Next() {
		var r schema.BanRecord
		var expires sql.NullString
		if err := rows.Scan(&r.IP, &r.Reason, &r.BannedAt, &expires, &r.FailCount); err != nil {
			return nil, err
		}
		if expires.Valid {
			v := expires.String
			r.ExpiresAt = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CheckBan returns a single ban record or nil.
func (s *Store) CheckBan(ctx context.Context, ip string) (*schema.BanRecord, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT ip, reason, banned_at, expires_at, fail_count FROM bans WHERE ip = ?`, ip)
	var r schema.BanRecord
	var expires sql.NullString
	if err := row.Scan(&r.IP, &r.Reason, &r.BannedAt, &expires, &r.FailCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if expires.Valid {
		v := expires.String
		r.ExpiresAt = &v
	}
	return &r, nil
}

// UpsertBan inserts or updates a ban row.
func (s *Store) UpsertBan(ctx context.Context, record schema.BanRecord) error {
	var expires sql.NullString
	if record.ExpiresAt != nil {
		expires = sql.NullString{String: *record.ExpiresAt, Valid: true}
	}
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO bans (ip, reason, banned_at, expires_at, fail_count) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(ip) DO UPDATE SET reason = excluded.reason, banned_at = excluded.banned_at,
			   expires_at = excluded.expires_at, fail_count = excluded.fail_count`,
			record.IP, record.Reason, record.BannedAt, expires, record.FailCount,
		)
		return err
	})
}

// RemoveBan deletes a ban row; returns whether anything was removed.
func (s *Store) RemoveBan(ctx context.Context, ip string) (bool, error) {
	var removed bool
	err := s.withWriteLock(func() error {
		res, err := s.DB.ExecContext(ctx, `DELETE FROM bans WHERE ip = ?`, ip)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		removed = rows > 0
		return nil
	})
	return removed, err
}

// CleanupExpiredBans removes bans whose expiry time has passed.
func (s *Store) CleanupExpiredBans(ctx context.Context) (int, error) {
	var count int
	err := s.withWriteLock(func() error {
		res, err := s.DB.ExecContext(ctx,
			`DELETE FROM bans WHERE expires_at IS NOT NULL AND expires_at <= ?`,
			util.NowISO())
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		count = int(rows)
		return nil
	})
	return count, err
}

// BanStats summarises permanent/temporary counts.
type BanStats struct {
	Total     int64 `json:"total"`
	Permanent int64 `json:"permanent"`
	Temporary int64 `json:"temporary"`
}

// GetBanStats returns ban counts.
func (s *Store) GetBanStats(ctx context.Context) (BanStats, error) {
	var stats BanStats
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bans`).Scan(&stats.Total); err != nil {
		return stats, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bans WHERE expires_at IS NULL`).Scan(&stats.Permanent); err != nil {
		return stats, err
	}
	stats.Temporary = stats.Total - stats.Permanent
	return stats, nil
}
