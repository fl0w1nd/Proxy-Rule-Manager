package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// plainTextErr writes a plain-text error response, matching the TS reference
// behaviour where proxy clients (Clash, Shadowrocket, …) consume raw rule
// files and must never receive JSON on error paths.
func plainTextErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, msg)
}

func (s *Server) serveRuleFile(w http.ResponseWriter, r *http.Request, isGeosite bool) {
	client := chi.URLParam(r, "client")
	file := chi.URLParam(r, "file")
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	if err := util.EnsureSafeSegment(file, "file"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	// Enforce .list extension — matches TS rule-files.ts behaviour.
	if !strings.HasSuffix(file, ".list") {
		plainTextErr(w, http.StatusBadRequest, "# Invalid file format")
		return
	}
	var (
		fullPath string
		err      error
	)
	if isGeosite {
		provider := chi.URLParam(r, "provider")
		if errSeg := util.EnsureSafeSegment(provider, "provider"); errSeg != nil {
			plainTextErr(w, http.StatusBadRequest, "# Bad request")
			return
		}
		fullPath, err = util.JoinInside(s.Config.RulesDir, client, "geosite", provider, file)
	} else {
		fullPath, err = util.JoinInside(s.Config.RulesDir, client, file)
	}
	if err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		plainTextErr(w, http.StatusNotFound, "# Rule not found")
		return
	}
	s.applyCDNHeaders(r.Context(), w, "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) serveIconSet(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if err := util.EnsureSafeSegment(filename, "filename"); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	fullPath := filepath.Join(s.Config.IconSetDir, filename)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		s.Error(w, http.StatusNotFound, "Icon not found")
		return
	}
	mime := guessIconMime(filename)
	s.applyCDNHeaders(r.Context(), w, mime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) servePublicClientFile(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientId")
	file := chi.URLParam(r, "file")
	if err := util.EnsureSafeSegment(clientID, "client"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	if err := util.EnsureSafeSegment(file, "file"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	configID, ext, ok := splitFile(file)
	if !ok {
		plainTextErr(w, http.StatusBadRequest, "# Invalid file format")
		return
	}
	_, content, err := s.Store.GetPublicClientFile(r.Context(), clientID, configID, ext)
	if err != nil {
		plainTextErr(w, http.StatusNotFound, "# File not found")
		return
	}
	s.applyCDNHeaders(r.Context(), w, "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func splitFile(name string) (configID, ext string, ok bool) {
	dot := -1
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot == len(name)-1 {
		return "", "", false
	}
	return name[:dot], name[dot+1:], true
}

func guessIconMime(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	}
	return "application/octet-stream"
}
