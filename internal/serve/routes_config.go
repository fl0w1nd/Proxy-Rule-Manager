package serve

import (
	"net/http"
	"os"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

// handleConfigDirty reports whether the config file on disk has been modified
// since the last load (startup or reload). The frontend polls this to decide
// whether to show the "config changed, reload?" banner.
func (s *Server) handleConfigDirty(w http.ResponseWriter, _ *http.Request) {
	if s.configFile == "" {
		writeJSON(w, http.StatusOK, map[string]any{"changed": false})
		return
	}
	fi, err := os.Stat(s.configFile)
	if err != nil {
		// File deleted or unreadable — not a change the user can reload.
		writeJSON(w, http.StatusOK, map[string]any{"changed": false})
		return
	}
	s.mu.RLock()
	recorded := s.configMtime
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"changed": fi.ModTime().After(recorded)})
}

// handleConfigReload hot-reloads the business config, atomically swaps it in
// Server/Engine/Manager, rebuilds the scheduler, refreshes state, and
// re-renders the site. Runtime settings stay fixed for the process lifetime.
func (s *Server) handleConfigReload(w http.ResponseWriter, _ *http.Request) {
	if s.configFile == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "reload_unavailable", "配置热重载未启用", map[string]any{})
		return
	}

	// 1. Load + validate.
	newCfg, err := config.Load(s.configFile, s.DataDir)
	if err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, "config_invalid", "配置文件无效", map[string]any{"reason": err.Error()})
		return
	}

	// 2. Swap manager config + rebuild scheduler (rejects if update running).
	if err := s.updates.ReloadConfig(newCfg); err != nil {
		if err == updates.ErrUpdateInProgress {
			writeAPIError(w, http.StatusConflict, "update_in_progress", "更新进行中，请稍后重载", map[string]any{})
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "reload_failed", "重载配置失败", map[string]any{"reason": err.Error()})
		return
	}

	// 3. Swap server config (under lock).
	s.mu.Lock()
	s.Config = newCfg
	s.mu.Unlock()

	// 4. Swap engine config (safe — Manager confirmed no update running).
	s.Engine.Config = newCfg

	// 5. Reconfigure fetcher + preprocessor from new config.
	s.Engine.Fetcher.Configure(
		time.Duration(newCfg.Update.Fetch.Timeout),
		int64(newCfg.Update.Fetch.MaxDownload),
		newCfg.Update.Fetch.Concurrency,
		newCfg.Update.Fetch.PerHostConcurrency,
		newCfg.Update.Fetch.Retries,
		time.Duration(newCfg.Update.Fetch.RetryDelay),
		newCfg.Update.Fetch.UserAgent,
	)
	s.Engine.Preprocessor.Configure(
		time.Duration(newCfg.Update.Preprocess.Timeout),
		int(int64(newCfg.Update.Preprocess.MaxOutput)),
	)

	// 6. Refresh state entry counts for the new rule set.
	ruleIDs := make([]string, 0, len(newCfg.Rules))
	for _, rule := range newCfg.Rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if changed, err := s.State.BackfillEntryCounts(ruleIDs); err != nil {
		s.Engine.Logger.Error("reload: backfill entry counts failed", "error", err)
	} else if changed {
		if err := s.State.Save(); err != nil {
			s.Engine.Logger.Error("reload: save state failed", "error", err)
		}
	}

	// 7. Ensure artifact directories exist for new client set.
	targets := config.ExpandOutputTargets(newCfg.Clients)
	clientIDs := make([]string, len(targets))
	for i, target := range targets {
		clientIDs[i] = target.ID
	}
	if err := engine.EnsureArtifactDirs(s.DataDir, clientIDs); err != nil {
		s.Engine.Logger.Error("reload: ensure artifact dirs failed", "error", err)
	}

	// 8. Re-render the public site from the new config.
	if err := s.Engine.EnsureSite(); err != nil {
		s.Engine.Logger.Error("reload: site re-render failed", "error", err)
	}

	// 9. Update recorded mtime so dirty returns false until next edit.
	if fi, err := os.Stat(s.configFile); err == nil {
		s.mu.Lock()
		s.configMtime = fi.ModTime()
		s.mu.Unlock()
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "reloaded"})
}
