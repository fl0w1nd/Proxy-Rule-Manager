package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/api"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "perform a healthcheck HTTP probe and exit (used by Docker HEALTHCHECK)")
	dev := flag.Bool("dev", false, "enable dev-mode logging")
	flag.Parse()

	// --healthcheck: probe /api/status and exit. Must run before any heavy init
	// so the distroless image can use the binary itself as its health probe.
	if *healthcheck {
		port := os.Getenv("PORT")
		if port == "" {
			port = "3000"
		}
		resp, err := http.Get("http://127.0.0.1:" + port + "/api/status")
		if err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "healthcheck: unexpected status %d\n", resp.StatusCode)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := config.Load()
	paths := store.Paths{
		DataDir:       cfg.DataDir,
		RulesDir:      cfg.RulesDir,
		SourcesDir:    cfg.SourcesDir,
		GeositeDir:    cfg.GeositeDir,
		IconSetDir:    cfg.IconSetDir,
		ClientFileDir: cfg.ClientFileDir,
		WAFDir:        cfg.WAFDir,
	}
	st, err := store.Open(cfg.DBPath, paths)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	srv := api.New(cfg, st)

	// Apply any persisted system tunables (timeouts, rate-limit, etc.) before
	// the listener starts so the very first request observes the right values.
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if ss, err := st.GetSystemSettings(bootCtx); err != nil {
		log.Printf("[boot] system settings: load failed: %v (using defaults)", err)
	} else {
		srv.ApplySystemSettings(ss)
	}

	// Recover from any prior crash: a hard kill mid-sync leaves jobs frozen
	// in 'running' status with the sync lock held. Without this sweep the
	// next sync attempt returns "Another sync is already running" for the
	// remainder of the lock TTL, and the dashboard's "正在同步..." badge
	// would stay pinned across restarts.
	if jobs, locks, err := st.RecoverStaleSyncState(bootCtx, "server restarted while sync was running"); err != nil {
		log.Printf("[boot] recover sync state failed: %v", err)
	} else if jobs > 0 || locks > 0 {
		log.Printf("[boot] recovered stale sync state: jobs=%d locks=%d", jobs, locks)
	}
	bootCancel()

	// Lightweight reconcile pass: detect-only. We never delete or repair
	// during startup — the goal is to make pre-existing drift visible in
	// the logs so operators notice it before the next sync amplifies it.
	// Done in a goroutine so a slow filesystem walk does not delay the
	// listener coming up.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report, err := syncengine.CheckConsistency(ctx, st, cfg.RulesDir, cfg.ClientFileDir)
		if err != nil {
			log.Printf("[boot] reconcile: check failed: %v", err)
			return
		}
		if len(report.Issues) == 0 {
			log.Printf("[boot] reconcile: ok (checked=%d)", report.Checked)
			return
		}
		counts := map[string]int{}
		for _, iss := range report.Issues {
			counts[iss.Type]++
		}
		log.Printf("[boot] reconcile: %d issues found (checked=%d): %v",
			len(report.Issues), report.Checked, counts)
		// Log error-severity issues individually so they show up in the
		// service logs without requiring a separate API call.
		for _, iss := range report.Issues {
			if iss.Severity == "error" {
				log.Printf("[boot] reconcile error: type=%s path=%s rule=%s client=%s file=%s msg=%s",
					iss.Type, iss.Path, iss.RuleName, iss.ClientID, iss.FileID, iss.Message)
			}
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	limiterStop := make(chan struct{})
	srv.RateLimiter.StartGC(limiterStop)
	defer close(limiterStop)

	scheduleStop := make(chan struct{})
	go runScheduledSync(rootCtx, srv, scheduleStop)
	defer close(scheduleStop)

	go func() {
		log.Printf("server listening on %s (data=%s dev=%v)", addr, cfg.DataDir, *dev)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("shutting down …")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

// runScheduledSync mirrors the TS scheduled-sync timer (1-minute cadence).
func runScheduledSync(ctx context.Context, srv *api.Server, stop <-chan struct{}) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	checkAndRun(ctx, srv)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-tick.C:
			checkAndRun(ctx, srv)
		}
	}
}

// scheduledSyncMaxDuration is a defensive backstop: even though Fetcher and
// ScriptRunner each enforce their own per-call timeouts, a buggy downstream
// path that ignores context could let a single scheduled sync run forever.
// Capping the whole call at an hour ensures we always get back to the
// 1-minute ticker loop even in the worst case.
const scheduledSyncMaxDuration = time.Hour

func checkAndRun(parent context.Context, srv *api.Server) {
	ctx, cancel := context.WithTimeout(parent, scheduledSyncMaxDuration)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduled sync] panic: %v", r)
		}
	}()
	cfg, err := srv.Store.GetConfig(ctx)
	if err != nil {
		log.Printf("[scheduled sync] load config: %v", err)
		return
	}
	if len(cfg.Rules) == 0 {
		return
	}
	sched, err := srv.Store.GetSyncSchedule(ctx)
	if err != nil {
		log.Printf("[scheduled sync] load schedule: %v", err)
		return
	}
	now := time.Now().UTC()
	nowISO := util.FormatISO(now)
	var nextSyncAt *time.Time
	if sched.NextSyncAt != nil {
		if parsed, err := time.Parse(time.RFC3339Nano, *sched.NextSyncAt); err == nil {
			nextSyncAt = &parsed
		}
	}
	hasNext := nextSyncAt != nil
	hasLast := sched.LastScheduledSyncAt != nil && *sched.LastScheduledSyncAt != ""

	needsSync := false
	if !hasNext && !hasLast {
		if sched.Mode == "cron" {
			next := syncengine.ComputeNextSyncAt(sched, &nowISO)
			if next != "" {
				_, _ = srv.Store.UpdateSyncSchedule(ctx, schema.SyncSchedule{NextSyncAt: &next})
			}
			return
		}
		needsSync = true
	} else {
		if !hasNext {
			base := sched.LastScheduledSyncAt
			if base == nil {
				base = &nowISO
			}
			next := syncengine.ComputeNextSyncAt(sched, base)
			if next != "" {
				_, _ = srv.Store.UpdateSyncSchedule(ctx, schema.SyncSchedule{NextSyncAt: &next})
				if parsed, err := time.Parse(time.RFC3339Nano, next); err == nil {
					nextSyncAt = &parsed
					hasNext = true
				}
			}
		}
		if hasNext && !now.Before(*nextSyncAt) {
			needsSync = true
		}
	}
	if !needsSync {
		return
	}
	log.Printf("[scheduled sync] running full sync …")
	res, err := srv.Engine.ExecuteFullSync(ctx)
	// IMPORTANT: always advance LastScheduledSyncAt + NextSyncAt, even on
	// failure. Otherwise a persistently failing sync re-fires every minute
	// (the tick interval) and produces a retry storm. The next attempt should
	// happen at the next scheduled time, not on the next tick.
	update := schema.SyncSchedule{LastScheduledSyncAt: &nowISO}
	if next := syncengine.ComputeNextSyncAt(sched, &nowISO); next != "" {
		update.NextSyncAt = &next
	}
	if _, uerr := srv.Store.UpdateSyncSchedule(ctx, update); uerr != nil {
		log.Printf("[scheduled sync] persist schedule: %v", uerr)
	}
	if err != nil {
		log.Printf("[scheduled sync] error: %v", err)
		return
	}
	log.Printf("[scheduled sync] done: changed=%d failed=%d", len(res.ChangedRules), len(res.FailedRules))
}
