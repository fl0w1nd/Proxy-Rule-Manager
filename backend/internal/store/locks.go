package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LockTTL is the default lifetime for runtime locks.
const LockTTL = 5 * time.Minute

// AcquireLock returns true if it could claim the lock or refresh its own.
// Runs under the global write mutex so the cleanup+insert pair is atomic.
func (s *Store) AcquireLock(ctx context.Context, key string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM locks WHERE expires_at < ?`, time.Now().UnixMilli()); err != nil {
		return false, err
	}
	now := time.Now().UnixMilli()
	expires := now + LockTTL.Milliseconds()
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO locks (key, expires_at) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET expires_at = excluded.expires_at WHERE locks.expires_at < ?`,
		key, expires, now)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// ReleaseLock removes a lock.
func (s *Store) ReleaseLock(ctx context.Context, key string) error {
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx, `DELETE FROM locks WHERE key = ?`, key)
		return err
	})
}

// IsLocked returns whether the named lock is currently held.
func (s *Store) IsLocked(ctx context.Context, key string) (bool, error) {
	if err := s.cleanupLocks(ctx); err != nil {
		return false, err
	}
	var expires int64
	err := s.DB.QueryRowContext(ctx, `SELECT expires_at FROM locks WHERE key = ?`, key).Scan(&expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return expires > time.Now().UnixMilli(), nil
}

// HasActiveRuleLocks reports whether any rule:* lock exists.
func (s *Store) HasActiveRuleLocks(ctx context.Context) (bool, error) {
	if err := s.cleanupLocks(ctx); err != nil {
		return false, err
	}
	var count int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM locks WHERE key LIKE 'rule:%' AND expires_at > ?`,
		time.Now().UnixMilli()).Scan(&count)
	return count > 0, err
}

// IsGlobalSyncLocked returns the global sync lock state.
func (s *Store) IsGlobalSyncLocked(ctx context.Context) (bool, error) {
	return s.IsLocked(ctx, "sync:global")
}

// AcquireGlobalSyncLock guards against concurrent partial syncs.
func (s *Store) AcquireGlobalSyncLock(ctx context.Context) (bool, string, error) {
	hasRuleLocks, err := s.HasActiveRuleLocks(ctx)
	if err != nil {
		return false, "", err
	}
	if hasRuleLocks {
		return false, "Partial sync is in progress", nil
	}
	acquired, err := s.AcquireLock(ctx, "sync:global")
	if err != nil {
		return false, "", err
	}
	if !acquired {
		return false, "Another sync is already running", nil
	}
	hasRuleLocks, err = s.HasActiveRuleLocks(ctx)
	if err != nil {
		return false, "", err
	}
	if hasRuleLocks {
		_ = s.ReleaseLock(ctx, "sync:global")
		return false, "Partial sync started, please retry", nil
	}
	return true, "", nil
}

// ReleaseGlobalSyncLock releases the named lock.
func (s *Store) ReleaseGlobalSyncLock(ctx context.Context) error {
	return s.ReleaseLock(ctx, "sync:global")
}

// AcquireRuleLock takes a per-rule lock, blocked if the global sync is running.
func (s *Store) AcquireRuleLock(ctx context.Context, ruleName string) (bool, string, error) {
	globalHeld, err := s.IsGlobalSyncLocked(ctx)
	if err != nil {
		return false, "", err
	}
	if globalHeld {
		return false, "Global sync is in progress", nil
	}
	acquired, err := s.AcquireLock(ctx, "rule:"+ruleName)
	if err != nil {
		return false, "", err
	}
	if !acquired {
		return false, "Rule is already being processed", nil
	}
	globalHeld, err = s.IsGlobalSyncLocked(ctx)
	if err != nil {
		return false, "", err
	}
	if globalHeld {
		_ = s.ReleaseLock(ctx, "rule:"+ruleName)
		return false, "Global sync started, please retry", nil
	}
	return true, "", nil
}

// ReleaseRuleLock releases a per-rule lock.
func (s *Store) ReleaseRuleLock(ctx context.Context, ruleName string) error {
	return s.ReleaseLock(ctx, "rule:"+ruleName)
}

func (s *Store) cleanupLocks(ctx context.Context) error {
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx, `DELETE FROM locks WHERE expires_at < ?`, time.Now().UnixMilli())
		return err
	})
}
