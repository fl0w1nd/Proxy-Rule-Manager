package api

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

var iconExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".webp": true,
	".ico":  true,
}

func (s *Server) registerIconSetRoutes(r chi.Router) {
	// GET /api/iconset is intentionally public — the public homepage calls it
	// without an admin token to render icon thumbnails (see src/components/home.tsx).
	r.Get("/iconset", s.handleListIcons)
	r.Post("/iconset/upload", s.adminGuard(s.handleUploadIcons))
	r.Put("/iconset/{id}", s.adminGuard(s.handleRenameIcon))
	r.Delete("/iconset/{id}", s.adminGuard(s.handleDeleteIcon))
}

type iconInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"createdAt"`
}

func (s *Server) ensureIconDir() (string, error) {
	if err := util.EnsureDir(s.Config.IconSetDir); err != nil {
		return "", err
	}
	return s.Config.IconSetDir, nil
}

func (s *Server) handleListIcons(w http.ResponseWriter, r *http.Request) {
	dir, err := s.ensureIconDir()
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	icons := make([]iconInfo, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !iconExts[ext] {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		bt := fileBirthtime(filepath.Join(dir, name), info)
		icons = append(icons, iconInfo{
			ID:        name,
			Name:      strings.TrimSuffix(name, ext),
			URL:       "/IconSet/" + url.PathEscape(name),
			Size:      info.Size(),
			CreatedAt: bt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}
	// Sort by createdAt descending; use ID as tiebreaker for a stable order.
	sort.Slice(icons, func(i, j int) bool {
		if icons[i].CreatedAt != icons[j].CreatedAt {
			return icons[i].CreatedAt > icons[j].CreatedAt
		}
		return icons[i].ID < icons[j].ID
	})
	s.JSON(w, http.StatusOK, map[string]any{"icons": icons})
}

func sanitizeIconFilename(name string) string {
	base := filepath.Base(name)
	if base == "." || base == "/" || base == "\\" {
		return ""
	}
	return base
}

func uniqueIconName(dir, original string) (string, error) {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(original, ext)
	candidate := original
	for i := 0; ; i++ {
		path := filepath.Join(dir, candidate)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
		if i == 0 {
			candidate = base + "_copy" + ext
		} else {
			candidate = base + "_copy_" + itoa(i+1) + ext
		}
	}
}

type renamedEntry struct {
	Original string `json:"original"`
	Renamed  string `json:"renamed"`
}

type uploadError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

func (s *Server) handleUploadIcons(w http.ResponseWriter, r *http.Request) {
	dir, err := s.ensureIconDir()
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	form := r.MultipartForm
	if form == nil || len(form.File["files"]) == 0 {
		s.Error(w, http.StatusBadRequest, "No files provided")
		return
	}
	uploaded := []iconInfo{}
	renamed := []renamedEntry{}
	errors := []uploadError{}
	for _, fh := range form.File["files"] {
		safe := sanitizeIconFilename(fh.Filename)
		if safe == "" || strings.HasPrefix(safe, ".") {
			errors = append(errors, uploadError{Name: fh.Filename, Error: "Invalid filename"})
			continue
		}
		ext := strings.ToLower(filepath.Ext(safe))
		if !iconExts[ext] {
			errors = append(errors, uploadError{Name: fh.Filename, Error: "Not a valid image file"})
			continue
		}
		uniq, err := uniqueIconName(dir, safe)
		if err != nil {
			errors = append(errors, uploadError{Name: fh.Filename, Error: err.Error()})
			continue
		}
		src, err := fh.Open()
		if err != nil {
			errors = append(errors, uploadError{Name: fh.Filename, Error: err.Error()})
			continue
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			errors = append(errors, uploadError{Name: fh.Filename, Error: err.Error()})
			continue
		}
		dest := filepath.Join(dir, uniq)
		if err := util.AtomicWriteFile(dest, data); err != nil {
			errors = append(errors, uploadError{Name: fh.Filename, Error: err.Error()})
			continue
		}
		if uniq != fh.Filename {
			renamed = append(renamed, renamedEntry{Original: fh.Filename, Renamed: uniq})
		}
		stat, _ := os.Stat(dest)
		uploaded = append(uploaded, iconInfo{
			ID:        uniq,
			Name:      strings.TrimSuffix(uniq, filepath.Ext(uniq)),
			URL:       "/IconSet/" + url.PathEscape(uniq),
			Size:      stat.Size(),
			CreatedAt: fileBirthtime(dest, stat).UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"uploaded": uploaded,
		"renamed":  renamed,
		"errors":   errors,
	})
}

func (s *Server) handleRenameIcon(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	safe := sanitizeIconFilename(id)
	if safe != id || safe == "" {
		s.Error(w, http.StatusBadRequest, "Invalid icon id")
		return
	}
	var body struct {
		NewName string `json:"newName"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.NewName == "" {
		s.Error(w, http.StatusBadRequest, "newName is required")
		return
	}
	dir, err := s.ensureIconDir()
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ext := filepath.Ext(safe)
	newName := body.NewName
	if !strings.HasSuffix(newName, ext) {
		newName += ext
	}
	safeNew := sanitizeIconFilename(newName)
	if safeNew != newName {
		s.Error(w, http.StatusBadRequest, "Invalid new name")
		return
	}
	oldPath := filepath.Join(dir, safe)
	newPath := filepath.Join(dir, safeNew)
	if _, err := os.Stat(oldPath); err != nil {
		s.Error(w, http.StatusNotFound, "Icon not found")
		return
	}
	if oldPath != newPath {
		if _, err := os.Stat(newPath); err == nil {
			s.Error(w, http.StatusConflict, "An icon with this name already exists")
			return
		}
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	stat, _ := os.Stat(newPath)
	icon := iconInfo{
		ID:        safeNew,
		Name:      strings.TrimSuffix(safeNew, ext),
		URL:       "/IconSet/" + url.PathEscape(safeNew),
		Size:      stat.Size(),
		CreatedAt: fileBirthtime(newPath, stat).UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "icon": icon})
}

func (s *Server) handleDeleteIcon(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	safe := sanitizeIconFilename(id)
	if safe != id || safe == "" {
		s.Error(w, http.StatusBadRequest, "Invalid icon id")
		return
	}
	dir, err := s.ensureIconDir()
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	path := filepath.Join(dir, safe)
	if _, err := os.Stat(path); err != nil {
		s.Error(w, http.StatusNotFound, "Icon not found")
		return
	}
	if err := os.Remove(path); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "deleted": safe})
}
