package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
)

func TestListRootIconsAPI(t *testing.T) {
	s, _, _ := testServer(t)
	h := s.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/icons", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/icons", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body.String())
	}

	iconsDir := filepath.Join(s.DataDir, site.StaticDir, "icons")
	if err := os.MkdirAll(filepath.Join(iconsDir, "QureColor"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"surge.svg", "mihomo.svg", "readme.txt"} {
		if err := os.WriteFile(filepath.Join(iconsDir, name), []byte("<svg/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(iconsDir, "QureColor", "google.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, authorized(http.MethodGet, "/api/v1/icons", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct{ Items []site.RootIcon }
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 || listed.Items[0].ID != "mihomo" || listed.Items[0].File != "mihomo.svg" || listed.Items[1].ID != "surge" {
		t.Fatalf("items=%+v", listed.Items)
	}
}
