package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// publicRuleFileExtRe restricts the trailing extension of a public rule
// filename to 1-16 lowercase alphanumerics so we can drop the historical
// hard-coded `.list` check without opening up arbitrary suffixes. The base
// name is still policed by util.EnsureSafeSegment, so we only need to
// validate that the ext segment is well-formed.
var publicRuleFileExtRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// validatePublicRuleFile enforces "<name>.<ext>" with a sane extension.
// Returns false (caller should respond 400) when the file is missing a
// dot, ends with a dot, or carries an extension we wouldn't accept from
// the admin form.
func validatePublicRuleFile(file string) bool {
	idx := strings.LastIndexByte(file, '.')
	if idx <= 0 || idx == len(file)-1 {
		return false
	}
	return publicRuleFileExtRe.MatchString(file[idx+1:])
}

// plainTextErr writes a plain-text error response, matching the TS reference
// behaviour where proxy clients (Clash, Shadowrocket, …) consume raw rule
// files and must never receive JSON on error paths.
func plainTextErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, msg)
}

func (s *Server) serveRuleFile(w http.ResponseWriter, r *http.Request, isGeosite bool) {
	client, err := pathParam(r, "client")
	if err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	file, err := pathParam(r, "file")
	if err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	if err := util.EnsureSafeSegment(file, "file"); err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	// File name must look like `<name>.<ext>` with a sane extension. The
	// hard `.list` restriction was relaxed so each client can publish in
	// its own format (yaml/json/...); the on-disk file still has to exist
	// for the read below to succeed, so we don't need a per-ext allow-list.
	if !validatePublicRuleFile(file) {
		plainTextErr(w, http.StatusBadRequest, "# Invalid file format")
		return
	}
	var fullPath string
	if isGeosite {
		provider, providerErr := pathParam(r, "provider")
		if providerErr != nil {
			plainTextErr(w, http.StatusBadRequest, "# Bad request")
			return
		}
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
	filename, err := pathParam(r, "filename")
	if err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
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
	// Force the browser to treat icons as downloadable resources rather
	// than navigations. Combined with the per-MIME CSP below this neuters
	// the "upload a malicious SVG, open the URL, exfiltrate localStorage"
	// vector.
	w.Header().Set("Content-Disposition", "inline; filename="+quoteContentDisposition(filename))
	if mime == "image/svg+xml" {
		// `sandbox` (with no allow-list tokens) forces the SVG into a unique
		// opaque origin, disables scripts, plugins, forms, popups and
		// same-origin access — so even if a malicious SVG slipped past the
		// upload-time blacklist, it cannot read localStorage or perform
		// authenticated fetches against this server.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// quoteContentDisposition produces a minimally-escaped filename for the
// Content-Disposition header. The filename is already restricted by
// util.EnsureSafeSegment, so we only have to escape backslashes and quotes.
func quoteContentDisposition(name string) string {
	var b strings.Builder
	b.Grow(len(name) + 2)
	b.WriteByte('"')
	for _, r := range name {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (s *Server) servePublicClientFile(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathParam(r, "clientId")
	if err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
	file, err := pathParam(r, "file")
	if err != nil {
		plainTextErr(w, http.StatusBadRequest, "# Bad request")
		return
	}
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

func pathParam(r *http.Request, key string) (string, error) {
	return url.PathUnescape(chi.URLParam(r, key))
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
