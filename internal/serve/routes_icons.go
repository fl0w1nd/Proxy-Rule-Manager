package serve

import (
	"net/http"
	"path/filepath"

	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
)

func (s *Server) handleIcons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": site.ListRootIcons(filepath.Join(s.DataDir, site.StaticDir)),
	})
}
