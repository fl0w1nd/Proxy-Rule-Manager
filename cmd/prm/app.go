package main

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/logging"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
	"github.com/fl0w1nd/proxy-rule-manager/templates"
)

// App holds the fully initialized application components.
type App struct {
	Config   *config.Config
	Registry *render.Registry
	Engine   *engine.UpdateEngine
	State    *state.Store
	Buffer   *logging.Buffer
	Logger   *slog.Logger
	Updates  *updates.Manager
}

// buildApp creates a fully initialized App from the config file.
func buildApp() (*App, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	buf := logging.NewBuffer(2000)
	logger := logging.Setup("text", buf)

	// Load templates
	registry := render.NewRegistry()
	if err := registry.LoadEmbedded(templates.FS); err != nil {
		return nil, fmt.Errorf("load embedded templates: %w", err)
	}
	overrideDir := filepath.Join(cfg.DataDir, "templates")
	if err := registry.LoadDir(overrideDir); err != nil {
		return nil, fmt.Errorf("load override templates: %w", err)
	}

	// Validate that all clients reference valid templates
	for _, client := range cfg.Clients {
		if _, ok := registry.Get(client.Template); !ok {
			return nil, fmt.Errorf("client %q references unknown template %q", client.ID, client.Template)
		}
	}

	// Open state
	st, err := state.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open state: %w", err)
	}
	ruleIDs := make([]string, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if changed, err := st.BackfillEntryCounts(ruleIDs); err != nil {
		return nil, fmt.Errorf("backfill rule entry counts: %w", err)
	} else if changed {
		if err := st.Save(); err != nil {
			return nil, fmt.Errorf("save rule entry counts: %w", err)
		}
	}

	// Create artifact directories
	clientIDs := make([]string, len(cfg.Clients))
	for i, c := range cfg.Clients {
		clientIDs[i] = c.ID
	}
	if err := engine.EnsureArtifactDirs(cfg.DataDir, clientIDs); err != nil {
		return nil, fmt.Errorf("create artifact dirs: %w", err)
	}

	// Create fetcher
	fetcher := engine.NewFetcher()
	fetcher.Configure(
		time.Duration(cfg.Update.Fetch.Timeout),
		int64(cfg.Update.Fetch.MaxDownload),
		cfg.Update.Fetch.Concurrency,
		cfg.Update.Fetch.PerHostConcurrency,
		cfg.Update.Fetch.Retries,
		time.Duration(cfg.Update.Fetch.RetryDelay),
		cfg.Update.Fetch.UserAgent,
	)

	// Create preprocessor
	preprocessor := engine.NewPreprocessRunner()
	preprocessor.Configure(
		time.Duration(cfg.Update.Preprocess.Timeout),
		int(cfg.Update.Preprocess.MaxOutput),
	)

	// Create geosite manager
	geositeManager := geosite.NewManager(filepath.Join(cfg.DataDir, "geosite"))

	eng := &engine.UpdateEngine{
		Config:       cfg,
		Registry:     registry,
		Fetcher:      fetcher,
		Preprocessor: preprocessor,
		State:        st,
		Geosite:      geositeManager,
		Logger:       logger,
	}
	updateManager, err := updates.NewManager(cfg, st, eng, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize update manager: %w", err)
	}

	return &App{
		Config:   cfg,
		Registry: registry,
		Engine:   eng,
		State:    st,
		Buffer:   buf,
		Logger:   logger,
		Updates:  updateManager,
	}, nil
}

func (a *App) startScheduledUpdate() {
	job, err := a.Updates.Start(updates.Request{Scope: "all"}, "scheduled")
	var conflict *updates.ConflictError
	if errors.As(err, &conflict) {
		a.Logger.Info("scheduled update skipped", "current_update_id", conflict.CurrentUpdateID)
		return
	}
	if err != nil {
		a.Logger.Error("scheduled update failed to start", "error", err)
		return
	}
	a.Logger.Info("scheduled update started", "job_id", job.ID)
}

// StartScheduler starts the update scheduler based on config. Returns a stop function.
func (a *App) StartScheduler() func() {
	switch a.Config.Update.Schedule.Mode {
	case "interval":
		dur := time.Duration(a.Config.Update.Schedule.Interval)
		if dur <= 0 {
			return func() {}
		}
		ticker := time.NewTicker(dur)
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					a.startScheduledUpdate()
				}
			}
		}()
		return func() {
			close(done)
			ticker.Stop()
		}

	case "cron":
		loc, err := time.LoadLocation(a.Config.Update.Schedule.Timezone)
		if err != nil {
			loc = time.UTC
		}
		c := cron.New(cron.WithLocation(loc))
		_, err = c.AddFunc(a.Config.Update.Schedule.Cron, func() {
			a.startScheduledUpdate()
		})
		if err != nil {
			a.Logger.Error("invalid cron expression", "cron", a.Config.Update.Schedule.Cron, "error", err)
			return func() {}
		}
		c.Start()
		return func() {
			c.Stop()
		}

	default:
		return func() {}
	}
}
