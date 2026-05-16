package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

const localSourceExt = ".txt"

var localSourceRefPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func normalizeLocalSourceRef(ref string) string {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ""
	}
	if !strings.HasSuffix(trimmed, localSourceExt) {
		trimmed += localSourceExt
	}
	if !localSourceRefPattern.MatchString(trimmed) {
		return ""
	}
	return trimmed
}

// WriteLocalSource saves content under the given ref (or a new UUID if missing)
// and mirrors it to both the database and disk for backup compatibility.
// Returns the canonical ref. Serialised through the global write mutex to
// keep the DB row and the on-disk file consistent.
func (s *Store) WriteLocalSource(ctx context.Context, ref, content string) (string, error) {
	normalized := normalizeLocalSourceRef(ref)
	if normalized == "" {
		normalized = newLocalSourceRef()
	}
	if err := s.withWriteLock(func() error {
		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO local_sources (ref, content) VALUES (?, ?)
			 ON CONFLICT(ref) DO UPDATE SET content = excluded.content`,
			normalized, content,
		); err != nil {
			return err
		}
		return writeLocalSourceFile(s.SourcesDir, normalized, content)
	}); err != nil {
		return "", err
	}
	return normalized, nil
}

// newLocalSourceRef mints a random ref for inline content without one.
func newLocalSourceRef() string {
	return uuid.New().String() + localSourceExt
}

// writeLocalSourceFile mirrors content to sources/<ref>.txt atomically.
func writeLocalSourceFile(dir, ref, content string) error {
	if dir == "" {
		return nil
	}
	return util.AtomicWriteFile(filepath.Join(dir, ref), []byte(content))
}

// removeLocalSourceFile deletes sources/<ref>.txt, swallowing errors caused
// by the file already being gone.
func removeLocalSourceFile(dir, ref string) error {
	if dir == "" {
		return nil
	}
	err := os.Remove(filepath.Join(dir, ref))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ReadLocalSource returns the content stored for ref.
func (s *Store) ReadLocalSource(ctx context.Context, ref string) (string, error) {
	normalized := normalizeLocalSourceRef(ref)
	if normalized == "" {
		return "", fmt.Errorf("invalid local source ref")
	}
	var content string
	err := s.DB.QueryRowContext(ctx, `SELECT content FROM local_sources WHERE ref = ?`, normalized).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		// Fallback: legacy file under sources/.
		if s.SourcesDir != "" {
			data, ferr := os.ReadFile(filepath.Join(s.SourcesDir, normalized))
			if ferr == nil {
				_, _ = s.WriteLocalSource(ctx, normalized, string(data))
				return string(data), nil
			}
		}
		return "", err
	}
	if err != nil {
		return "", err
	}
	return content, nil
}

// PruneLocalSources removes all local source rows + files not in keepRefs.
func (s *Store) PruneLocalSources(ctx context.Context, keepRefs map[string]struct{}) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT ref FROM local_sources`)
	if err != nil {
		return err
	}
	var toDelete []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			rows.Close()
			return err
		}
		if _, ok := keepRefs[r]; !ok {
			toDelete = append(toDelete, r)
		}
	}
	rows.Close()
	if len(toDelete) == 0 {
		return nil
	}
	for _, r := range toDelete {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM local_sources WHERE ref = ?`, r); err != nil {
			return err
		}
		if s.SourcesDir != "" {
			_ = os.Remove(filepath.Join(s.SourcesDir, r))
		}
	}
	return nil
}

// hydrateLocalSources copies persisted content into source.Content fields.
func (s *Store) hydrateLocalSources(ctx context.Context, cfg schema.RulesConfig) (schema.RulesConfig, error) {
	for ri := range cfg.Rules {
		rule := &cfg.Rules[ri]
		for si := range rule.Sources {
			src := &rule.Sources[si]
			if src.SourceType() != "local" {
				continue
			}
			if src.Content != nil {
				continue
			}
			if src.ContentRef == "" {
				continue
			}
			content, err := s.ReadLocalSource(ctx, src.ContentRef)
			if err == nil {
				v := content
				src.Content = &v
			}
		}
	}
	return cfg, nil
}
