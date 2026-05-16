package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func (s *Server) registerSystemRoutes(r chi.Router) {
	r.Get("/system-settings", s.adminGuard(s.handleGetSystemSettings))
	r.Put("/system-settings", s.adminGuard(s.handlePutSystemSettings))
	r.Get("/system/disk-usage", s.adminGuard(s.handleSystemDiskUsage))
}

func (s *Server) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Store.GetSystemSettings(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"settings": settings,
		"defaults": schema.DefaultSystemSettings(),
	})
}

func (s *Server) handlePutSystemSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Settings schema.SystemSettings `json:"settings"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	body.Settings.MergeDefaults()
	if err := body.Settings.Validate(); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	saved, err := s.Store.SaveSystemSettings(r.Context(), body.Settings)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	applied := s.ApplySystemSettings(saved)
	s.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"settings": applied,
	})
}

// diskBucket is one row in the /api/system/disk-usage response. Bytes is
// always present (0 if the path is missing); Path is informational.
type diskBucket struct {
	Key   string `json:"key"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// handleSystemDiskUsage walks the data-dir buckets the dashboard cares about
// and returns each one's on-disk size. Rules/ + geosite/ are typically the
// largest by far; we list them separately so admins can pinpoint where the
// space is going. Errors on individual files are swallowed (we just
// undercount) so a single permission glitch doesn't fail the whole call.
func (s *Server) handleSystemDiskUsage(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config
	dirs := []struct {
		key, path string
	}{
		{"rules", cfg.RulesDir},
		{"geosite", cfg.GeositeDir},
		{"sources", cfg.SourcesDir},
		{"iconset", cfg.IconSetDir},
		{"client", cfg.ClientFileDir},
	}
	out := make([]diskBucket, 0, len(dirs)+1)
	var total int64
	for _, d := range dirs {
		size := dirSizeBytes(d.path)
		out = append(out, diskBucket{Key: d.key, Path: d.path, Bytes: size})
		total += size
	}
	if dbSize := fileSizeBytes(cfg.DBPath); dbSize >= 0 {
		out = append(out, diskBucket{Key: "db", Path: cfg.DBPath, Bytes: dbSize})
		total += dbSize
	} else {
		out = append(out, diskBucket{Key: "db", Path: cfg.DBPath, Bytes: 0})
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"total":   total,
		"buckets": out,
	})
}

// dirSizeBytes returns the cumulative size of all regular files under root.
// Missing root is reported as 0 (not an error) so that fresh installs render
// cleanly. Walk errors on individual entries are skipped — we'd rather
// undercount than 500 the whole disk-usage endpoint.
func dirSizeBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// fileSizeBytes returns -1 if the file doesn't exist; otherwise the file's
// size in bytes (0 for empty files).
func fileSizeBytes(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}
