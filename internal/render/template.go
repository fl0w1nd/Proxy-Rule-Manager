// Package render implements template-driven rule rendering. Templates are YAML
// files describing how IR entries map to output format syntax. Three codecs
// handle the actual serialization: linelist, yaml_payload, and singbox.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template is the deserialized YAML template for one output format.
type Template struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Codec       string `yaml:"codec"`
	Extension   string `yaml:"extension"`
	Description string `yaml:"description,omitempty"`

	KindMap   map[string]KindMapping `yaml:"kind_map"`
	FlagKinds []string               `yaml:"flag_kinds,omitempty"`

	// singbox-specific fields
	FieldGroups []FieldGroup `yaml:"field_groups,omitempty"`

	// Per-kind rendering hints
	Hints map[string]KindHint `yaml:"hints,omitempty"`
}

// KindMapping maps an IR kind to an output type or field name.
type KindMapping struct {
	TypeName  string   `yaml:"type_name,omitempty"`
	FieldName string   `yaml:"field_name,omitempty"`
	Aliases   []string `yaml:"aliases,omitempty"`
}

// FieldGroup defines which JSON fields share OR-semantics in a singbox rule object.
type FieldGroup struct {
	Name   string   `yaml:"name"`
	Fields []string `yaml:"fields"`
}

// KindHint provides special rendering instructions for a specific IR kind.
type KindHint struct {
	IPv6TypeName   string `yaml:"ipv6_type_name,omitempty"`
	SplitPortRange bool   `yaml:"split_port_range,omitempty"`
	PortRangeSep   string `yaml:"port_range_sep,omitempty"`
	Transform      string `yaml:"transform,omitempty"`
	LeadingDot     bool   `yaml:"leading_dot,omitempty"`
}

// UnmarshalYAML allows KindMapping to be specified as either a simple string
// (just the type/field name) or a full object.
func (km *KindMapping) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		km.TypeName = value.Value
		km.FieldName = value.Value
		return nil
	}
	allowed := map[string]struct{}{
		"type_name": {}, "field_name": {}, "aliases": {},
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if _, ok := allowed[value.Content[i].Value]; !ok {
			return fmt.Errorf("unknown kind mapping field %q", value.Content[i].Value)
		}
	}
	type raw KindMapping
	return value.Decode((*raw)(km))
}

func decodeTemplate(data []byte, target *Template) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(target)
}

// Validate checks the template for structural correctness.
func (t *Template) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("template: id is required")
	}
	switch t.Codec {
	case "linelist", "yaml_payload", "singbox":
	default:
		return fmt.Errorf("template %q: codec must be linelist, yaml_payload, or singbox", t.ID)
	}
	if t.Extension == "" {
		return fmt.Errorf("template %q: extension is required", t.ID)
	}
	if t.Extension == "." || !strings.HasPrefix(t.Extension, ".") || strings.ContainsAny(t.Extension, `/\`) {
		return fmt.Errorf("template %q: extension must be a file suffix such as .list", t.ID)
	}
	if len(t.KindMap) == 0 {
		return fmt.Errorf("template %q: kind_map is required", t.ID)
	}
	return nil
}

// Registry holds all loaded templates indexed by ID.
type Registry struct {
	templates map[string]*Template
}

// NewRegistry creates an empty template registry.
func NewRegistry() *Registry {
	return &Registry{templates: make(map[string]*Template)}
}

// Get returns the template with the given ID.
func (r *Registry) Get(id string) (*Template, bool) {
	t, ok := r.templates[id]
	return t, ok
}

// IDs returns all registered template IDs.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.templates))
	for id := range r.templates {
		ids = append(ids, id)
	}
	return ids
}

// Register adds a template to the registry.
func (r *Registry) Register(t *Template) error {
	if err := t.Validate(); err != nil {
		return err
	}
	r.templates[t.ID] = t
	return nil
}

// LoadEmbedded loads all templates from the given embedded filesystem. The fs
// should have template YAML files at the root level (or in a "templates/" dir).
func (r *Registry) LoadEmbedded(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read embedded template %s: %w", path, err)
		}
		var t Template
		if err := decodeTemplate(data, &t); err != nil {
			return fmt.Errorf("parse embedded template %s: %w", path, err)
		}
		return r.Register(&t)
	})
}

// LoadDir loads user-override templates from a directory. Templates with the
// same ID as a default template replace the default.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read template dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read template %s: %w", name, err)
		}
		var t Template
		if err := decodeTemplate(data, &t); err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}
		if err := r.Register(&t); err != nil {
			return fmt.Errorf("register template %s: %w", name, err)
		}
	}
	return nil
}
