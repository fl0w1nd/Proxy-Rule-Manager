package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

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
	DataDir  string
	Config   *config.Config
	Registry *render.Registry
	Engine   *engine.UpdateEngine
	State    *state.Store
	Buffer   *logging.Buffer
	Logger   *slog.Logger
	Updates  *updates.Manager
}

// buildApp creates a fully initialized App from the config file.
func buildApp(dataDir string) (*App, error) {
	cfg, err := config.Load(cfgFile, dataDir)
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
	overrideDir := filepath.Join(dataDir, "templates")
	if err := registry.LoadDir(overrideDir); err != nil {
		return nil, fmt.Errorf("load override templates: %w", err)
	}

	// Validate that every expanded output target references a loaded template.
	targets := config.ExpandOutputTargets(cfg.Clients)
	for _, target := range targets {
		if _, ok := registry.Get(target.Template); !ok {
			return nil, fmt.Errorf("output target %q references unknown template %q", target.ID, target.Template)
		}
	}

	// Open state
	st, err := state.Open(dataDir)
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
	clientIDs := make([]string, len(targets))
	for i, target := range targets {
		clientIDs[i] = target.ID
	}
	if err := engine.EnsureArtifactDirs(dataDir, clientIDs); err != nil {
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
	geositeManager := geosite.NewManager(filepath.Join(dataDir, "geosite"))

	eng := &engine.UpdateEngine{
		DataDir:      dataDir,
		Config:       cfg,
		Registry:     registry,
		Fetcher:      fetcher,
		Preprocessor: preprocessor,
		State:        st,
		Geosite:      geositeManager,
		Logger:       logger,
	}
	updateManager, err := updates.NewManager(cfg, dataDir, st, eng, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize update manager: %w", err)
	}

	// Drop root privileges: when the container starts as root (needed to fix
	// root-owned bind-mounted volumes), chown the data dir to the unprivileged
	// uid and switch to it so the server runs as non-root from here on.
	if err := dropPrivileges(dataDir); err != nil {
		return nil, fmt.Errorf("drop privileges: %w", err)
	}

	return &App{
		DataDir:  dataDir,
		Config:   cfg,
		Registry: registry,
		Engine:   eng,
		State:    st,
		Buffer:   buf,
		Logger:   logger,
		Updates:  updateManager,
	}, nil
}
