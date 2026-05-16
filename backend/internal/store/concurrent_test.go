package store

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestConcurrent_SaveConfig_DoesNotCorrupt verifies that two parallel
// SaveConfig writers do not corrupt the database. Last-writer-wins for the
// JSON body is acceptable, but the per-table indexes must remain consistent.
func TestConcurrent_SaveConfig_DoesNotCorrupt(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := schema.DefaultConfig()
			cfg.Rules = append(cfg.Rules, schema.RuleConfig{
				Name:   fmt.Sprintf("rule_w%d", w),
				Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
			})
			for i := 0; i < 25; i++ {
				if _, err := s.SaveConfig(ctx, cfg); err != nil {
					t.Errorf("worker %d iter %d: %v", w, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Verify final state is internally consistent.
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected exactly 1 rule, got %d", len(cfg.Rules))
	}

	// The rules table must mirror what's in the JSON.
	var jsonCount, tableCount int
	jsonCount = len(cfg.Rules)
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM rules`).Scan(&tableCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if jsonCount != tableCount {
		t.Fatalf("rules JSON has %d rules but rules table has %d (consistency lost)", jsonCount, tableCount)
	}
}

func TestConcurrent_DailyStats_AreAtomic(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := s.IncrementDailyStats(ctx, "2026-05-16", schema.DailyStats{
					SyncCount:           1,
					RulesChanged:        2,
					TotalRulesProcessed: 3,
				}); err != nil {
					t.Errorf("increment: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, err := s.GetDailyStats(ctx, "2026-05-16")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncCount != 16*50 || got.RulesChanged != 16*50*2 || got.TotalRulesProcessed != 16*50*3 {
		t.Fatalf("lost increments: %+v", got)
	}
}

func TestConcurrent_KVSettings_LastWriterWins(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// 8 goroutines each setting a *different field* on the same schedule
	// concurrently. With the existing read-modify-write logic we lose
	// updates; once a writeMu is in place each merge becomes atomic.
	updates := []schema.SyncSchedule{
		{Mode: "interval", IntervalHours: 6},
		{Mode: "cron", CronExpression: "0 0 * * *"},
		{IntervalHours: 12},
		{CronExpression: "*/15 * * * *"},
	}

	var wg sync.WaitGroup
	for _, upd := range updates {
		upd := upd
		for w := 0; w < 4; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					if _, err := s.UpdateSyncSchedule(ctx, upd); err != nil {
						t.Errorf("update: %v", err)
						return
					}
				}
			}()
		}
	}
	wg.Wait()

	got, err := s.GetSyncSchedule(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// We don't assert exact field values (mode/interval/cron all valid
	// depending on last writer) — just that the schedule is consistent.
	if got.IntervalHours <= 0 {
		t.Fatalf("interval lost: %+v", got)
	}
	if got.CronExpression == "" {
		t.Fatalf("cron lost: %+v", got)
	}
}
