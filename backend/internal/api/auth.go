package api

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// AuthLevel mirrors the TS checkAuth result.
type AuthLevel string

const (
	AuthAdmin   AuthLevel = "admin"
	AuthPublic  AuthLevel = "public"
	AuthInvalid AuthLevel = "invalid"
)

// CheckAuth returns the authorization level for an incoming request.
func (s *Server) CheckAuth(authHeader string) AuthLevel {
	if s.AdminToken == "" {
		return AuthAdmin
	}
	if authHeader == "" {
		return AuthPublic
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return AuthInvalid
	}
	if authHeader[len("Bearer "):] == s.AdminToken {
		return AuthAdmin
	}
	return AuthInvalid
}

// VerifyAdmin returns whether the Authorization header is the admin token.
// When ADMIN_TOKEN is unset it allows everyone (matches TS).
func (s *Server) VerifyAdmin(authHeader string) bool {
	if s.AdminToken == "" {
		return true
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	return authHeader[len("Bearer "):] == s.AdminToken
}

// RateLimiter implements the per-IP exponential back-off seen in TS.
//
// mu guards the in-memory failures map; cfgMu guards the tunable knobs so
// that admin-driven Configure() calls never tear under read traffic. Splitting
// the mutexes keeps the hot failure map contention-free.
type RateLimiter struct {
	mu       sync.Mutex
	failures map[string]failureRecord

	cfgMu             sync.RWMutex
	BaseDelay         time.Duration
	Exponent          float64
	MaxBlockDuration  time.Duration
	PermanentBanLimit int
	RecordMaxAge      time.Duration
}

type failureRecord struct {
	count        int
	lastFailedAt time.Time
}

// NewRateLimiter constructs the default limiter (mirrors TS CONFIG).
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		failures:          make(map[string]failureRecord),
		BaseDelay:         5 * time.Second,
		Exponent:          2,
		MaxBlockDuration:  60 * time.Minute,
		PermanentBanLimit: 10,
		RecordMaxAge:      24 * time.Hour,
	}
}

// Configure swaps the rate-limiter knobs atomically. Existing failure records
// are preserved; in-flight IsBlocked queries observe the new values on the
// next call (calculateBlockDuration re-reads each time).
func (r *RateLimiter) Configure(baseDelay, maxBlock, recordMaxAge time.Duration, permanentBanLimit int) {
	if baseDelay <= 0 {
		baseDelay = 5 * time.Second
	}
	if maxBlock <= 0 {
		maxBlock = 60 * time.Minute
	}
	if recordMaxAge <= 0 {
		recordMaxAge = 24 * time.Hour
	}
	if permanentBanLimit <= 0 {
		permanentBanLimit = 10
	}
	r.cfgMu.Lock()
	defer r.cfgMu.Unlock()
	r.BaseDelay = baseDelay
	r.MaxBlockDuration = maxBlock
	r.RecordMaxAge = recordMaxAge
	r.PermanentBanLimit = permanentBanLimit
}

// readConfig returns a copy of the current tunables under a read lock.
func (r *RateLimiter) readConfig() (time.Duration, float64, time.Duration, int, time.Duration) {
	r.cfgMu.RLock()
	defer r.cfgMu.RUnlock()
	return r.BaseDelay, r.Exponent, r.MaxBlockDuration, r.PermanentBanLimit, r.RecordMaxAge
}

func (r *RateLimiter) calculateBlockDuration(count int) time.Duration {
	baseDelay, exponent, maxBlock, _, _ := r.readConfig()
	dur := time.Duration(math.Pow(float64(count), exponent))*time.Second + baseDelay
	if dur > maxBlock {
		return maxBlock
	}
	return dur
}

// IsBlocked reports the current block status; first checks SQLite bans, then in-memory failures.
func (r *RateLimiter) IsBlocked(ctx context.Context, st *store.Store, ip string) (bool, int, string, error) {
	ban, err := st.CheckBan(ctx, ip)
	if err != nil {
		return false, 0, "", err
	}
	if ban != nil {
		if ban.ExpiresAt == nil {
			return true, 0, ban.Reason, nil
		}
		t, err := time.Parse(time.RFC3339Nano, *ban.ExpiresAt)
		if err == nil {
			now := time.Now().UTC()
			if now.Before(t) {
				return true, int(t.Sub(now).Seconds() + 0.5), ban.Reason, nil
			}
		}
		_, _ = st.RemoveBan(ctx, ip)
	}
	r.mu.Lock()
	rec, ok := r.failures[ip]
	r.mu.Unlock()
	if !ok {
		return false, 0, "", nil
	}
	deadline := rec.lastFailedAt.Add(r.calculateBlockDuration(rec.count))
	if time.Now().Before(deadline) {
		return true, int(time.Until(deadline).Seconds() + 0.5), "too_many_attempts", nil
	}
	return false, 0, "", nil
}

// RecordFailure increments and optionally promotes to a permanent ban.
func (r *RateLimiter) RecordFailure(ctx context.Context, st *store.Store, ip string) error {
	r.mu.Lock()
	rec := r.failures[ip]
	rec.count++
	rec.lastFailedAt = time.Now()
	r.failures[ip] = rec
	count := rec.count
	r.mu.Unlock()

	_, _, _, permanentBanLimit, _ := r.readConfig()
	if count >= permanentBanLimit {
		if err := st.UpsertBan(ctx, schema.BanRecord{
			IP:        ip,
			Reason:    "auto_permanent_ban_brute_force",
			BannedAt:  util.NowISO(),
			ExpiresAt: nil,
			FailCount: int64(count),
		}); err != nil {
			return err
		}
		r.mu.Lock()
		delete(r.failures, ip)
		r.mu.Unlock()
	}
	return nil
}

// Clear removes a single IP's failure record.
func (r *RateLimiter) Clear(ip string) {
	r.mu.Lock()
	delete(r.failures, ip)
	r.mu.Unlock()
}

// Snapshot returns a copy of the in-memory failure map (for /api/waf/failures).
func (r *RateLimiter) Snapshot() map[string]struct {
	Count        int
	LastFailedAt time.Time
} {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]struct {
		Count        int
		LastFailedAt time.Time
	}, len(r.failures))
	for k, v := range r.failures {
		out[k] = struct {
			Count        int
			LastFailedAt time.Time
		}{Count: v.count, LastFailedAt: v.lastFailedAt}
	}
	return out
}

// StartGC kicks off the periodic cleaner.
func (r *RateLimiter) StartGC(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				r.gcOnce()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (r *RateLimiter) gcOnce() {
	_, _, _, _, recordMaxAge := r.readConfig()
	cutoff := time.Now().Add(-recordMaxAge)
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, rec := range r.failures {
		if rec.lastFailedAt.Before(cutoff) {
			delete(r.failures, ip)
		}
	}
}
