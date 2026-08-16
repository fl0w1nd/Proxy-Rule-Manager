// Package admin provides the embedded Svelte 5 management application frontend.
package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the sub-filesystem containing only the dist content.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// distSub is computed once at init time; it cannot fail because "dist" is
// statically embedded.
var distSub = func() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("admin: embedded dist sub-fs: " + err.Error())
	}
	return sub
}()

var fileServer = http.FileServer(http.FS(distSub))

// Handler returns an http.Handler that serves the Svelte 5 SPA.
// Requests to existing files under /admin are served directly; other paths
// fall back to index.html to support client-side routing.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		p := strings.TrimPrefix(r.URL.Path, "/admin")
		p = strings.TrimPrefix(p, "/")
		if p == "" || p == "index.html" || p == "admin.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}

		// Check if the requested file exists as a regular file in the embedded filesystem
		if f, err := distSub.Open(p); err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !stat.IsDir() {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/" + p
				fileServer.ServeHTTP(w, r2)
				return
			}
		}

		// Fallback to index.html for SPA
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
