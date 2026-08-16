package admin_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/admin"
)

func TestDistFSLoads(t *testing.T) {
	fsys, err := admin.DistFS()
	if err != nil {
		t.Fatalf("DistFS error: %v", err)
	}

	if _, err := fsys.Open("index.html"); err != nil {
		t.Fatalf("index.html not found in DistFS: %v", err)
	}
}

func TestHandlerServesIndexAndFallsBack(t *testing.T) {
	h := admin.Handler()

	// Test GET /admin
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="app"`) || !strings.Contains(body, "PRM · 管理系统") {
		t.Fatalf("unexpected body content: %s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}

	// Test GET /admin/unknown-route (SPA fallback)
	req2 := httptest.NewRequest(http.MethodGet, "/admin/rules/any/deep/path", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusOK)
	}
	if !strings.Contains(rec2.Body.String(), `id="app"`) {
		t.Fatalf("SPA fallback did not return index.html")
	}

	// Test GET /admin/assets/... (Asset serving)
	// Check what assets exist in dist/assets
	sub, _ := admin.DistFS()
	entries, err := fs.ReadDir(sub, "assets")
	if err == nil && len(entries) > 0 {
		assetName := entries[0].Name()
		req3 := httptest.NewRequest(http.MethodGet, "/admin/assets/"+assetName, nil)
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusOK {
			t.Fatalf("asset status = %d, want %d", rec3.Code, http.StatusOK)
		}
	}
}
