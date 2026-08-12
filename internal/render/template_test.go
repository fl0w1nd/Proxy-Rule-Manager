package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/templates"
)

func TestLoadEmbeddedTemplates(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadEmbedded(templates.FS); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	expected := []string{"mihomo-classical", "mihomo-yaml", "singbox", "surge", "shadowrocket"}
	for _, id := range expected {
		if _, ok := r.Get(id); !ok {
			t.Errorf("template %q not loaded", id)
		}
	}
}

func TestTemplateValidation(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		tmpl := &Template{Codec: "linelist", Extension: ".list", KindMap: map[string]KindMapping{"x": {TypeName: "X"}}}
		if err := tmpl.Validate(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("invalid codec", func(t *testing.T) {
		tmpl := &Template{ID: "test", Codec: "bad", Extension: ".list", KindMap: map[string]KindMapping{"x": {TypeName: "X"}}}
		if err := tmpl.Validate(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("unsafe extension", func(t *testing.T) {
		tmpl := &Template{ID: "test", Codec: "linelist", Extension: "../list", KindMap: map[string]KindMapping{"x": {TypeName: "X"}}}
		if err := tmpl.Validate(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("valid", func(t *testing.T) {
		tmpl := &Template{ID: "test", Codec: "linelist", Extension: ".list", KindMap: map[string]KindMapping{"x": {TypeName: "X"}}}
		if err := tmpl.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	content := `
id: custom
name: Custom
codec: linelist
extension: .txt
kind_map:
  domain:
    type_name: DOMAIN
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if _, ok := r.Get("custom"); !ok {
		t.Error("custom template not loaded")
	}
}

func TestLoadDirMissing(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadDir("/nonexistent/path"); err != nil {
		t.Errorf("LoadDir should silently skip missing dirs: %v", err)
	}
}
