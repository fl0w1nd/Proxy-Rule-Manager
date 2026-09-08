package serve

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
	"github.com/fl0w1nd/proxy-rule-manager/templates"
)

type templateItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Codec     string `json:"codec"`
	Extension string `json:"extension"`
	Builtin   bool   `json:"builtin"`
}
type templateDetail struct {
	templateItem
	YAML    string `json:"yaml"`
	Version string `json:"version"`
}
type templateRequest struct {
	YAML    *string    `json:"yaml"`
	Version string     `json:"version"`
	Sample  []ir.Entry `json:"sample,omitempty"`
}

func builtinTemplate(id string) bool {
	found := false
	_ = fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		t, err := render.ParseTemplate(data)
		if err == nil && t.ID == id {
			found = true
		}
		return nil
	})
	return found
}

func templateMeta(t *render.Template) templateItem {
	return templateItem{ID: t.ID, Name: t.Name, Codec: t.Codec, Extension: t.Extension, Builtin: builtinTemplate(t.ID)}
}

func templateVersion(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func (s *Server) handleTemplates(w http.ResponseWriter, _ *http.Request) {
	items := []templateItem{}
	for _, id := range s.Engine.Registry.IDs() {
		t, _ := s.Engine.Registry.Get(id)
		items = append(items, templateMeta(t))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Find custom templates by their declared ID; existing files may use other names.
func (s *Server) templateSource(id string) (string, []byte, error) {
	dir := filepath.Join(s.DataDir, "templates")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, os.ErrNotExist
	}
	if err != nil {
		return "", nil, err
	}
	var path string
	var raw []byte
	for _, e := range entries {
		if !e.Type().IsRegular() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return "", nil, err
		}
		t, err := render.ParseTemplate(data)
		if err != nil || t.ID != id {
			continue
		}
		if path != "" {
			return "", nil, fmt.Errorf("模板 ID %q 存在多个文件", id)
		}
		path, raw = filepath.Join(dir, e.Name()), data
	}
	if path == "" {
		return "", nil, os.ErrNotExist
	}
	return path, raw, nil
}

func (s *Server) handleTemplate(w http.ResponseWriter, r *http.Request) {
	s.templateMu.Lock()
	defer s.templateMu.Unlock()
	t, ok := s.Engine.Registry.Get(chi.URLParam(r, "id"))
	if !ok {
		writeAPIError(w, http.StatusNotFound, "template_not_found", "找不到该模板", map[string]any{})
		return
	}
	_, data, err := s.templateSource(t.ID)
	if errors.Is(err, os.ErrNotExist) || builtinTemplate(t.ID) {
		data, err = yaml.Marshal(t)
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "template_read_failed", err.Error(), map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, templateDetail{templateMeta(t), string(data), templateVersion(data)})
}

var templateLine = regexp.MustCompile(`line (\d+)`)

func writeTemplateError(w http.ResponseWriter, err error) {
	line := 0
	if match := templateLine.FindStringSubmatch(err.Error()); len(match) > 1 {
		_, _ = fmt.Sscan(match[1], &line)
	}
	writeAPIError(w, http.StatusUnprocessableEntity, "invalid_template", "模板校验失败", map[string]any{"errors": []configIssue{{Path: "yaml", Line: line, Message: err.Error()}}})
}

func parseTemplateRequest(w http.ResponseWriter, r *http.Request) (*render.Template, templateRequest, bool) {
	var req templateRequest
	if !decodeConfigRequest(w, r, &req) {
		return nil, req, false
	}
	if req.YAML == nil {
		writeTemplateError(w, fmt.Errorf("yaml is required"))
		return nil, req, false
	}
	t, err := render.ParseTemplate([]byte(*req.YAML))
	if err == nil {
		err = util.EnsureSafeSegment(t.ID, "template id")
	}
	if err == nil && (strings.TrimSpace(t.ID) != t.ID || strings.HasPrefix(t.ID, ".") || strings.ContainsAny(t.ID, "\x00\r\n") || len(t.ID) > 200) {
		err = fmt.Errorf("invalid template id")
	}
	if err != nil {
		writeTemplateError(w, err)
		return nil, req, false
	}
	return t, req, true
}

func templateSample() []ir.Entry {
	return []ir.Entry{
		{Kind: ir.KindDomain, Value: "example.com"},
		{Kind: ir.KindDomainSuffix, Value: "example.org"},
		{Kind: ir.KindDomainKeyword, Value: "example"},
		{Kind: ir.KindIPCIDR, Value: "192.0.2.0/24", Flags: []string{ir.FlagNoResolve}},
		{Kind: ir.KindIPCIDR, Value: "2001:db8::/32"},
	}
}

func (s *Server) handleTemplateValidate(w http.ResponseWriter, r *http.Request) {
	t, req, ok := parseTemplateRequest(w, r)
	if !ok {
		return
	}
	sample := req.Sample
	if len(sample) == 0 {
		sample = templateSample()
	}
	output, err := render.Render(t, sample)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "errors": []configIssue{}, "output": string(output), "sample": sample, "extension": t.Extension})
}

func (s *Server) handleTemplateSave(w http.ResponseWriter, r *http.Request) {
	t, req, ok := parseTemplateRequest(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id != "" && id != t.ID {
		writeTemplateError(w, fmt.Errorf("模板 ID 必须与原模板一致"))
		return
	}
	if builtinTemplate(t.ID) {
		writeAPIError(w, http.StatusConflict, "builtin_template", "内置模板为只读，请使用新的模板 ID", map[string]any{})
		return
	}
	if _, err := render.Render(t, templateSample()); err != nil {
		writeTemplateError(w, err)
		return
	}
	s.templateMu.Lock()
	defer s.templateMu.Unlock()
	status := http.StatusOK
	err := s.updates.Reconfigure(false, func() (*config.Config, error) {
		path, data, err := s.templateSource(t.ID)
		if id == "" {
			if _, exists := s.Engine.Registry.Get(t.ID); exists || err == nil {
				return nil, fmt.Errorf("模板 ID 已存在")
			}
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			path = filepath.Join(s.DataDir, "templates", t.ID+".yaml")
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("模板文件已存在或无法访问")
			}
			status = http.StatusCreated
		} else {
			if err != nil {
				return nil, err
			}
			if req.Version == "" || req.Version != templateVersion(data) {
				return nil, fmt.Errorf("模板已发生变化，请重新载入后保存")
			}
		}
		if err := util.AtomicWriteFile(path, []byte(*req.YAML)); err != nil {
			return nil, err
		}
		if err := s.Engine.Registry.Register(t); err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		writeAPIError(w, http.StatusConflict, "template_save_failed", err.Error(), map[string]any{})
		return
	}
	writeJSON(w, status, templateDetail{templateMeta(t), *req.YAML, templateVersion([]byte(*req.YAML))})
}
