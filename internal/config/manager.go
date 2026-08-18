package config

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
	"gopkg.in/yaml.v3"
)

var ErrPersistenceUnavailable = errors.New("config persistence is unavailable")

// VersionConflictError reports a stale optimistic-concurrency version.
type VersionConflictError struct {
	CurrentVersion int64
}

func (e *VersionConflictError) Error() string { return "config version conflict" }

// DirtyConfigError reports an external edit that must be reloaded first.
type DirtyConfigError struct{}

func (e *DirtyConfigError) Error() string { return "config file changed on disk" }

// Candidate is a validated, immutable configuration change awaiting commit.
type Candidate struct {
	baseVersion int64
	baseDigest  [sha256.Size]byte
	raw         []byte
	doc         *yaml.Node
	cfg         *Config
	changed     bool
	reload      bool
}

// Config returns an independent copy of the candidate runtime configuration.
func (c *Candidate) Config() *Config { return c.cfg.DeepCopy() }

// Changed reports whether committing the candidate advances the version.
func (c *Candidate) Changed() bool { return c.changed }

// Manager owns the source YAML, effective configuration, and process-local
// optimistic-concurrency version.
type Manager struct {
	mu      sync.RWMutex
	path    string
	dataDir string
	raw     []byte
	doc     *yaml.Node
	cfg     *Config
	digest  [sha256.Size]byte
	version int64
}

// NewManager loads a source-preserving configuration manager from path.
func NewManager(path, dataDir string) (*Manager, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg, doc, err := decodeDocument(raw, dataDir)
	if err != nil {
		return nil, err
	}
	return &Manager{
		path: path, dataDir: dataDir, raw: append([]byte(nil), raw...),
		doc: cloneYAMLNode(doc), cfg: cfg.DeepCopy(), digest: sha256.Sum256(raw), version: 1,
	}, nil
}

// NewMemoryManager creates a read-only manager for callers without a config
// file, primarily tests and embedded use.
func NewMemoryManager(cfg *Config) *Manager {
	raw, _ := yaml.Marshal(cfg)
	var doc yaml.Node
	_ = yaml.Unmarshal(raw, &doc)
	return &Manager{
		raw: append([]byte(nil), raw...), doc: &doc, cfg: cfg.DeepCopy(),
		digest: sha256.Sum256(raw), version: 1,
	}
}

// Snapshot returns an immutable runtime configuration and its version.
func (m *Manager) Snapshot() (*Config, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.DeepCopy(), m.version
}

// SourceSnapshot returns the unexpanded YAML document as JSON-compatible data.
func (m *Manager) SourceSnapshot() (any, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var source any
	if err := m.doc.Decode(&source); err != nil {
		return nil, 0, fmt.Errorf("decode config source: %w", err)
	}
	return source, m.version, nil
}

// Dirty reports whether the source file differs from the managed document.
func (m *Manager) Dirty() (bool, error) {
	m.mu.RLock()
	path, digest := m.path, m.digest
	m.mu.RUnlock()
	if path == "" {
		return false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return sha256.Sum256(raw) != digest, nil
}

// PrepareReload validates the current file contents for an external reload.
func (m *Manager) PrepareReload() (*Candidate, error) {
	m.mu.RLock()
	path, baseVersion, baseDigest := m.path, m.version, m.digest
	m.mu.RUnlock()
	if path == "" {
		return nil, ErrPersistenceUnavailable
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	effective, doc, err := decodeDocument(raw, m.dataDir)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		baseVersion: baseVersion, baseDigest: baseDigest, raw: raw,
		doc: doc, cfg: effective, changed: sha256.Sum256(raw) != baseDigest, reload: true,
	}, nil
}

// Commit atomically persists and installs a prepared candidate.
func (m *Manager) Commit(candidate *Candidate) (*Config, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if candidate.baseVersion != m.version {
		return nil, m.version, &VersionConflictError{CurrentVersion: m.version}
	}
	if !candidate.changed {
		if dirty, err := fileDigestDiffers(m.path, candidate.baseDigest); err != nil {
			return nil, m.version, err
		} else if dirty {
			return nil, m.version, &DirtyConfigError{}
		}
		return m.cfg.DeepCopy(), m.version, nil
	}
	if candidate.reload {
		raw, err := os.ReadFile(m.path)
		if err != nil {
			return nil, m.version, fmt.Errorf("read config: %w", err)
		}
		if !bytes.Equal(raw, candidate.raw) {
			return nil, m.version, &DirtyConfigError{}
		}
	} else {
		checkSource := func() error {
			raw, err := os.ReadFile(m.path)
			if err != nil {
				return fmt.Errorf("read config: %w", err)
			}
			if !bytes.Equal(raw, m.raw) {
				return &DirtyConfigError{}
			}
			return nil
		}
		if err := util.AtomicWriteFileChecked(m.path, candidate.raw, checkSource); err != nil {
			var dirty *DirtyConfigError
			if errors.As(err, &dirty) {
				return nil, m.version, err
			}
			return nil, m.version, fmt.Errorf("write config: %w", err)
		}
	}
	m.raw = append([]byte(nil), candidate.raw...)
	m.doc = cloneYAMLNode(candidate.doc)
	m.cfg = candidate.cfg.DeepCopy()
	m.digest = sha256.Sum256(candidate.raw)
	m.version++
	return m.cfg.DeepCopy(), m.version, nil
}

func fileDigestDiffers(path string, expected [sha256.Size]byte) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read config: %w", err)
	}
	return sha256.Sum256(raw) != expected, nil
}

func encodeDocument(doc *yaml.Node) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close config encoder: %w", err)
	}
	return out.Bytes(), nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clones := make(map[*yaml.Node]*yaml.Node)
	var clone func(*yaml.Node) *yaml.Node
	clone = func(source *yaml.Node) *yaml.Node {
		if source == nil {
			return nil
		}
		if existing, ok := clones[source]; ok {
			return existing
		}
		out := *source
		out.Content = make([]*yaml.Node, len(source.Content))
		clones[source] = &out
		for i, child := range source.Content {
			out.Content[i] = clone(child)
		}
		out.Alias = clone(source.Alias)
		return &out
	}
	return clone(node)
}
