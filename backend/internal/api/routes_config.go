package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

func (s *Server) registerConfigRoutes(r chi.Router) {
	r.Get("/config", s.adminGuard(s.handleGetConfig))
	r.Put("/config", s.adminGuard(s.handlePutConfig))
	r.Get("/database/backup", s.adminGuard(s.handleDatabaseBackup))
	r.Post("/database/restore", s.adminGuard(s.handleDatabaseRestore))
}

// validateRulesConfig validates enum fields across all rules in the config.
// Empty source types are allowed (they default to "url" at runtime).
func validateRulesConfig(cfg *schema.RulesConfig) error {
	for _, rule := range cfg.Rules {
		for i, src := range rule.Sources {
			if src.Type != "" {
				if err := schema.ValidateSourceType(src.Type); err != nil {
					return fmt.Errorf("rule %q source %d: %s", rule.Name, i, err.Error())
				}
			}
			if src.Type == "geosite" {
				if err := schema.ValidateGeositeProvider(src.Provider); err != nil {
					return fmt.Errorf("rule %q source %d: %s", rule.Name, i, err.Error())
				}
				if src.RenderProfile != "" {
					if err := schema.ValidateGeositeRenderProfile(src.RenderProfile); err != nil {
						return fmt.Errorf("rule %q source %d: %s", rule.Name, i, err.Error())
					}
				}
			}
		}
		for i, t := range rule.Transforms {
			if err := schema.ValidateTransformType(t.Type); err != nil {
				return fmt.Errorf("rule %q transform %d: %s", rule.Name, i, err.Error())
			}
		}
		if rule.Merge != nil && rule.Merge.Strategy != "" {
			if err := schema.ValidateMergeStrategy(rule.Merge.Strategy); err != nil {
				return fmt.Errorf("rule %q merge: %s", rule.Name, err.Error())
			}
		}
	}
	return nil
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("raw")
	useRaw := raw == "1" || raw == "true"
	var cfg schema.RulesConfig
	var err error
	if useRaw {
		cfg, err = s.Store.GetConfigRaw(r.Context())
	} else {
		cfg, err = s.Store.GetConfig(r.Context())
	}
	if err != nil {
		if errors.Is(err, store.ErrConfigCorrupted) {
			log.Printf("[config] payload is corrupted: %v", err)
			s.ErrorWith(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
				"code":  "CONFIG_CORRUPTED",
				"hint":  "请通过数据库备份恢复或调用 reset 接口重置后再保存。",
			})
			return
		}
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	rev, _ := s.Store.GetConfigRev(r.Context())
	s.JSON(w, http.StatusOK, map[string]any{"config": cfg, "rev": rev})
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config      schema.RulesConfig `json:"config"`
		ExpectedRev *int64             `json:"expectedRev,omitempty"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRulesConfigPayload(&body.Config); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	body.Config.EnsureDefaults()
	if err := validateRulesConfig(&body.Config); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := store.ValidateConfigPaths(body.Config); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if cycle := syncengine.DetectCircularDependency(body.Config.Rules); cycle != nil {
		s.Error(w, http.StatusBadRequest, "检测到循环依赖: "+strings.Join(cycle, " → "))
		return
	}
	var rev int64
	var err error
	if body.ExpectedRev != nil {
		rev, err = s.Store.SaveConfigWithExpectedRev(r.Context(), body.Config, *body.ExpectedRev)
	} else {
		rev, err = s.Store.SaveConfig(r.Context(), body.Config)
	}
	if err != nil {
		if errors.Is(err, store.ErrConfigConflict) {
			currentRev, _ := s.Store.GetConfigRev(r.Context())
			s.ErrorWith(w, http.StatusConflict, map[string]any{
				"error":       "Config has changed. Reload the latest config and try again.",
				"code":        "CONFIG_CONFLICT",
				"currentRev":  currentRev,
				"expectedRev": body.ExpectedRev,
			})
			return
		}
		s.ErrorWith(w, http.StatusInternalServerError, map[string]any{
			"error":             "Failed to save config",
			"message":           err.Error(),
			"validationMessage": "Invalid config format",
		})
		return
	}
	affected := make([]string, 0, len(body.Config.Rules))
	for _, rule := range body.Config.Rules {
		affected = append(affected, rule.Name)
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"rev":           rev,
		"affectedRules": affected,
	})
}

func (s *Server) handleDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	state, err := s.buildDatabaseSnapshot(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := writeZipFile(zw, "db.json", state); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, pair := range []struct{ src, prefix string }{
		{s.Config.ClientFileDir, "client-files"},
		{s.Config.SourcesDir, "sources"},
		{s.Config.WAFDir, "waf"},
		{s.Config.IconSetDir, "iconset"},
		{s.Config.GeositeDir, "geosite"},
		{s.Config.RulesDir, "Rules"},
	} {
		_ = addDirToZip(zw, pair.src, pair.prefix)
	}
	if err := zw.Close(); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"proxy-rule-manager-%s.zip\"", time.Now().UTC().Format("2006-01-02")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) buildDatabaseSnapshot(ctx context.Context) ([]byte, error) {
	// Previously every individual store read swallowed its error, so a
	// transient DB failure could produce a "successful" backup ZIP that
	// was actually missing whole tables (clients/artifacts/...). Restoring
	// such a backup would silently wipe state. We now fail fast: any read
	// error short-circuits the snapshot and the HTTP handler returns 500.
	cfg, err := s.Store.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	clients, err := s.Store.GetClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("read clients: %w", err)
	}
	arts, err := s.Store.GetAllArtifactMetas(ctx)
	if err != nil {
		return nil, fmt.Errorf("read artifacts: %w", err)
	}
	clientFiles, err := s.Store.ListAllClientFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("read client files: %w", err)
	}
	lastSync, err := s.Store.GetLastSyncInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("read last sync info: %w", err)
	}
	cdn, err := s.Store.GetCdnSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cdn settings: %w", err)
	}
	schedule, err := s.Store.GetSyncSchedule(ctx)
	if err != nil {
		return nil, fmt.Errorf("read sync schedule: %w", err)
	}
	systemSettings, err := s.Store.GetSystemSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read system settings: %w", err)
	}
	if clientFiles == nil {
		clientFiles = []schema.ClientFileMeta{}
	}
	return json.MarshalIndent(map[string]any{
		"version":        1,
		"config":         cfg,
		"clients":        clients,
		"artifacts":      arts,
		"clientFiles":    clientFiles,
		"lastSyncInfo":   lastSync,
		"cdnSettings":    cdn,
		"syncSchedule":   schedule,
		"systemSettings": systemSettings,
	}, "", "  ")
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func addDirToZip(zw *zip.Writer, sourceDir, prefix string) error {
	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		zipPath := prefix + "/" + filepath.ToSlash(rel)
		return writeZipFile(zw, zipPath, data)
	})
}

// handleDatabaseRestore restores from a Go-era backup zip uploaded from the
// dashboard. Accepts only Go-era backups (produced by /api/database/backup).
// Legacy TS-era db.json backups are no longer supported — see README.
//
// IMPORTANT: this restore is destructive; it resets config/clients before
// rewriting state. Callers should warn the user before invoking it.
func (s *Server) handleDatabaseRestore(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.Error(w, http.StatusBadRequest, "Missing import file")
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		s.Error(w, http.StatusBadRequest, "Invalid zip file: "+err.Error())
		return
	}
	if err := importGoBackupZip(zr, s.Store); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// Re-apply system tunables (timeouts, rate-limit, etc.) so a restored
	// backup takes effect without requiring a server restart. Failures here
	// are non-fatal; the persisted values still survive in the database.
	if ss, err := s.Store.GetSystemSettings(r.Context()); err == nil {
		s.ApplySystemSettings(ss)
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// ----- backup restore helpers (Go-era backups only) -----

// backupSnapshot mirrors the envelope written by buildDatabaseSnapshot.
type backupSnapshot struct {
	Version        int                     `json:"version"`
	Config         *schema.RulesConfig     `json:"config"`
	Clients        []schema.ClientConfig   `json:"clients"`
	Artifacts      []schema.ArtifactMeta   `json:"artifacts"`
	ClientFiles    []schema.ClientFileMeta `json:"clientFiles"`
	LastSyncInfo   *schema.LastSyncInfo    `json:"lastSyncInfo"`
	CdnSettings    *schema.CdnSettings     `json:"cdnSettings"`
	SyncSchedule   *schema.SyncSchedule    `json:"syncSchedule"`
	SystemSettings *schema.SystemSettings  `json:"systemSettings,omitempty"`
}

// swapEntry tracks a completed directory swap so it can be rolled back on
// failure.  Used by importGoBackupZip and rollbackDirSwaps.
type swapEntry struct {
	realDir string
	oldDir  string
}

func importGoBackupZip(zr *zip.Reader, st *store.Store) error {
	prefixMap := map[string]string{
		"client-files/": st.ClientFileDir,
		"sources/":      st.SourcesDir,
		"waf/":          st.WAFDir,
		"iconset/":      st.IconSetDir,
		"geosite/":      st.GeositeDir,
		"Rules/":        st.RulesDir,
	}

	// --- Phase 1: Read and validate db.json before any side-effects ---
	var dbBytes []byte
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(f.Name))
		if name == "db.json" {
			data, err := readZipEntry(f)
			if err != nil {
				return fmt.Errorf("read db.json: %w", err)
			}
			dbBytes = data
			break
		}
	}
	if len(dbBytes) == 0 {
		return fmt.Errorf("backup zip missing db.json")
	}
	var snap backupSnapshot
	if err := json.Unmarshal(dbBytes, &snap); err != nil {
		return fmt.Errorf("parse db.json: %w", err)
	}
	if snap.Config == nil {
		return fmt.Errorf("backup db.json has no config field — only Go-era backups are supported")
	}

	clients := snap.Clients
	if len(clients) == 0 {
		clients = schema.DefaultClients
	}
	for _, c := range clients {
		if err := store.ValidateClientID(c.ID); err != nil {
			return fmt.Errorf("validate client %q: %w", c.ID, err)
		}
	}
	if err := store.ValidateConfigPaths(*snap.Config); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// --- Phase 2: Extract zip entries into staging directories ---
	stagingDirs := make(map[string]string, len(prefixMap))
	for prefix, realDir := range prefixMap {
		stagingDir := realDir + ".restore-staging"
		if err := os.RemoveAll(stagingDir); err != nil {
			return fmt.Errorf("clean staging %s: %w", stagingDir, err)
		}
		if err := os.MkdirAll(stagingDir, 0o755); err != nil {
			return fmt.Errorf("create staging %s: %w", stagingDir, err)
		}
		stagingDirs[prefix] = stagingDir
	}
	cleanupStaging := func() {
		for _, dir := range stagingDirs {
			_ = os.RemoveAll(dir)
		}
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(f.Name))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			log.Printf("[restore] skip unsafe entry %q", f.Name)
			continue
		}
		if name == "db.json" {
			continue
		}
		var dstDir, rel string
		for prefix, stagingDir := range stagingDirs {
			if strings.HasPrefix(name, prefix) {
				dstDir = stagingDir
				rel = strings.TrimPrefix(name, prefix)
				break
			}
		}
		if dstDir == "" {
			log.Printf("[restore] skip unrelated entry %q", name)
			continue
		}
		if err := extractZipEntry(f, dstDir, rel); err != nil {
			cleanupStaging()
			return fmt.Errorf("extract %s: %w", name, err)
		}
	}

	// --- Phase 3: Atomic directory swap ---
	var swaps []swapEntry
	for prefix, realDir := range prefixMap {
		stagingDir := stagingDirs[prefix]
		oldDir := realDir + ".restore-old"
		_ = os.RemoveAll(oldDir)

		realExists := true
		if _, err := os.Stat(realDir); os.IsNotExist(err) {
			realExists = false
		}
		if realExists {
			if err := os.Rename(realDir, oldDir); err != nil {
				rollbackDirSwaps(swaps)
				cleanupStaging()
				return fmt.Errorf("rename %s → %s: %w", realDir, oldDir, err)
			}
		}
		if err := os.Rename(stagingDir, realDir); err != nil {
			if realExists {
				_ = os.Rename(oldDir, realDir)
			}
			rollbackDirSwaps(swaps)
			cleanupStaging()
			return fmt.Errorf("rename staging → %s: %w", realDir, err)
		}
		swaps = append(swaps, swapEntry{realDir: realDir, oldDir: oldDir})
	}

	// --- Phase 4: SQLite restore ---
	ctx := context.Background()
	if _, err := st.ResetConfig(ctx, *snap.Config, clients); err != nil {
		rollbackDirSwaps(swaps)
		cleanupStaging()
		return fmt.Errorf("reset config: %w", err)
	}
	if len(snap.Artifacts) > 0 {
		if err := st.SaveArtifactMetas(ctx, snap.Artifacts); err != nil {
			log.Printf("[restore] warning: artifacts: %v", err)
		}
	}
	for _, meta := range snap.ClientFiles {
		if err := st.RestoreClientFile(ctx, meta); err != nil {
			log.Printf("[restore] warning: client file %q: %v", meta.ID, err)
		}
	}
	if snap.SyncSchedule != nil {
		if _, err := st.UpdateSyncSchedule(ctx, *snap.SyncSchedule); err != nil {
			log.Printf("[restore] warning: sync schedule: %v", err)
		}
	}
	if snap.CdnSettings != nil {
		present := map[string]bool{
			"enabled": true, "cacheMode": true, "staleIfErrorSeconds": true,
			"customCacheControl": true, "cloudflareCdnCacheControl": true, "customHeaders": true,
		}
		if _, err := st.UpdateCdnSettings(ctx, *snap.CdnSettings, present); err != nil {
			log.Printf("[restore] warning: cdn settings: %v", err)
		}
	}
	if snap.LastSyncInfo != nil {
		present := map[string]bool{
			"lastFullSyncAt": true, "lastPartialSyncAt": true, "lastSuccessfulSyncAt": true,
			"totalRulesCount": true, "changedRulesCount": true, "failedRulesCount": true,
			"lastSyncDurationMs": true,
		}
		if err := st.UpdateLastSyncInfo(ctx, *snap.LastSyncInfo, present); err != nil {
			log.Printf("[restore] warning: last sync info: %v", err)
		}
	}
	if snap.SystemSettings != nil {
		ss := *snap.SystemSettings
		ss.MergeDefaults()
		if _, err := st.SaveSystemSettings(ctx, ss); err != nil {
			log.Printf("[restore] warning: system settings: %v", err)
		}
	}

	hydrateLocalSourcesFromDisk(ctx, st)
	for _, sw := range swaps {
		_ = os.RemoveAll(sw.oldDir)
	}
	cleanupStaging()
	return nil
}

// rollbackDirSwaps reverses a partial directory swap: for each completed
// entry, rename real back to staging and old back to real.
func rollbackDirSwaps(swaps []swapEntry) {
	for i := len(swaps) - 1; i >= 0; i-- {
		sw := swaps[i]
		stagingDir := sw.realDir + ".restore-staging"
		_ = os.Rename(sw.realDir, stagingDir)
		_ = os.Rename(sw.oldDir, sw.realDir)
	}
}

func hydrateLocalSourcesFromDisk(ctx context.Context, st *store.Store) {
	cfg, err := st.GetConfig(ctx)
	if err != nil {
		return
	}
	seen := map[string]struct{}{}
	for _, rule := range cfg.Rules {
		for _, src := range rule.Sources {
			if src.SourceType() != "local" || src.ContentRef == "" {
				continue
			}
			ref := src.ContentRef
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			data, ferr := os.ReadFile(filepath.Join(st.SourcesDir, ref))
			if ferr != nil {
				log.Printf("[restore] warning: missing source %s for rule %q", ref, rule.Name)
				continue
			}
			if _, werr := st.WriteLocalSource(ctx, ref, string(data)); werr != nil {
				log.Printf("[restore] warning: write source %s: %v", ref, werr)
			}
		}
	}
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 256<<20))
}

func extractZipEntry(f *zip.File, dstDir, rel string) error {
	if rel == "" {
		return nil
	}
	target := filepath.Join(dstDir, filepath.FromSlash(rel))
	cleanTarget := filepath.Clean(target)
	cleanDst := filepath.Clean(dstDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanTarget+string(filepath.Separator), cleanDst) {
		return fmt.Errorf("entry escapes destination: %s", rel)
	}
	if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
		return err
	}
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(in, 256<<20))
	return err
}
