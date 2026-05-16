package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// ClientFileInput is the create/update payload.
type ClientFileInput struct {
	ConfigID    string
	DisplayName string
	Description *string
	Ext         string
	IsPublic    bool
	IsPublicSet bool
	Content     string
	ContentSet  bool
}

// CreateClientFile stores a new client file (db row + disk file).
//
// Ordering: write content to a temp file first, then insert the DB row, then
// atomically rename the temp file into place. Failures at any step roll back
// the earlier work so a successful return implies "DB row AND on-disk file
// both exist". A DB insert failure removes the temp file; a final rename
// failure deletes the just-inserted row so the next attempt is not blocked by
// the unique (client_id, config_id, ext) constraint.
func (s *Store) CreateClientFile(ctx context.Context, clientID string, input ClientFileInput) (schema.ClientFileMeta, error) {
	if err := ValidateClientID(clientID); err != nil {
		return schema.ClientFileMeta{}, err
	}
	input.Ext = normalizeClientFileExt(input.Ext)
	now := util.NowISO()
	id := uuid.New().String()
	desc := sql.NullString{}
	if input.Description != nil {
		desc.String = *input.Description
		desc.Valid = true
	}

	finalPath := clientFilePath(s.ClientFileDir, clientID, input.ConfigID, input.Ext)
	tempPath, err := util.WriteTempFile(finalPath, []byte(input.Content))
	if err != nil {
		return schema.ClientFileMeta{}, fmt.Errorf("stage client file: %w", err)
	}
	// From here on the temp file is owned by us; ensure it's cleaned up
	// unless we deliberately commit it via CommitTempFile.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	if err := s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO client_files (id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, clientID, input.ConfigID, input.DisplayName, desc, input.Ext, boolToInt(input.IsPublic), now, now,
		)
		return err
	}); err != nil {
		return schema.ClientFileMeta{}, fmt.Errorf("insert client file: %w", err)
	}

	if err := util.CommitTempFile(tempPath, finalPath); err != nil {
		// Roll back the DB row so the user can retry without hitting the
		// unique constraint, and so listings don't expose a row whose disk
		// file is missing.
		_ = s.withWriteLock(func() error {
			_, derr := s.DB.ExecContext(ctx, `DELETE FROM client_files WHERE id = ?`, id)
			return derr
		})
		return schema.ClientFileMeta{}, fmt.Errorf("publish client file: %w", err)
	}
	committed = true

	meta := schema.ClientFileMeta{
		ID:          id,
		ClientID:    clientID,
		ConfigID:    input.ConfigID,
		DisplayName: input.DisplayName,
		Description: input.Description,
		Ext:         input.Ext,
		IsPublic:    input.IsPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return meta, nil
}

// UpdateClientFile updates the metadata and (optionally) content.
//
// Ordering: stage the new content (if any) to a temp file alongside the
// destination, then update the DB row under the write lock, then publish the
// new content / rename the file. If the DB update fails the old file is left
// intact and only the temp file is cleaned up. If the final filesystem step
// fails after the DB succeeded, the old file is still preserved (we delete
// the previous path only after the new one is in place), so a single bad
// rename does not destroy user data; the caller sees an error and can retry.
func (s *Store) UpdateClientFile(ctx context.Context, fileID string, updates ClientFileInput) (schema.ClientFileMeta, error) {
	current, err := s.GetClientFileMeta(ctx, fileID)
	if err != nil {
		return schema.ClientFileMeta{}, err
	}
	if err := ValidateClientID(current.ClientID); err != nil {
		return schema.ClientFileMeta{}, err
	}

	nextConfigID := current.ConfigID
	if updates.ConfigID != "" {
		if err := schema.ValidateConfigID(updates.ConfigID); err != nil {
			return schema.ClientFileMeta{}, err
		}
		nextConfigID = updates.ConfigID
	}
	nextDisplayName := current.DisplayName
	if updates.DisplayName != "" {
		nextDisplayName = updates.DisplayName
	}
	nextDescription := current.Description
	if updates.Description != nil {
		nextDescription = updates.Description
	}
	nextExt := current.Ext
	if updates.Ext != "" {
		nextExt = normalizeClientFileExt(updates.Ext)
	}
	nextIsPublic := current.IsPublic
	if updates.IsPublicSet {
		nextIsPublic = updates.IsPublic
	}

	oldPath := clientFilePath(s.ClientFileDir, current.ClientID, current.ConfigID, current.Ext)
	newPath := clientFilePath(s.ClientFileDir, current.ClientID, nextConfigID, nextExt)

	identifierChanged := nextConfigID != current.ConfigID || nextExt != current.Ext
	if identifierChanged {
		var exists string
		err := s.DB.QueryRowContext(ctx,
			`SELECT id FROM client_files WHERE client_id = ? AND config_id = ? AND ext = ? AND id <> ?`,
			current.ClientID, nextConfigID, nextExt, fileID,
		).Scan(&exists)
		if err == nil {
			return schema.ClientFileMeta{}, fmt.Errorf("file with config %q and ext %q already exists for client %q",
				nextConfigID, nextExt, current.ClientID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return schema.ClientFileMeta{}, err
		}
	}

	// Stage new content (if updated) to a temp file alongside the final
	// destination so the rename in the commit phase is atomic on-fs.
	var (
		stagedPath    string
		hasStaged     bool
		cleanupStaged = func() {
			if hasStaged && stagedPath != "" {
				_ = os.Remove(stagedPath)
			}
		}
	)
	if updates.ContentSet {
		p, err := util.WriteTempFile(newPath, []byte(updates.Content))
		if err != nil {
			return schema.ClientFileMeta{}, fmt.Errorf("stage client file: %w", err)
		}
		stagedPath = p
		hasStaged = true
	}
	defer cleanupStaged()

	var oldContentBackup string
	if updates.ContentSet {
		if data, err := os.ReadFile(oldPath); err == nil {
			oldContentBackup = string(data)
		}
	}

	publishedNewContent := false
	movedExistingFile := false
	if hasStaged {
		if err := util.CommitTempFile(stagedPath, newPath); err != nil {
			return schema.ClientFileMeta{}, fmt.Errorf("publish client file: %w", err)
		}
		publishedNewContent = true
	} else if identifierChanged && oldPath != newPath {
		if _, statErr := os.Stat(oldPath); statErr == nil {
			if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
				return schema.ClientFileMeta{}, fmt.Errorf("ensure target dir: %w", err)
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				return schema.ClientFileMeta{}, fmt.Errorf("rename client file: %w", err)
			}
			movedExistingFile = true
		}
	}

	now := util.NowISO()
	desc := sql.NullString{}
	if nextDescription != nil {
		desc.String = *nextDescription
		desc.Valid = true
	}
	if err := s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`UPDATE client_files SET config_id = ?, display_name = ?, description = ?, ext = ?, is_public = ?, updated_at = ? WHERE id = ?`,
			nextConfigID, nextDisplayName, desc, nextExt, boolToInt(nextIsPublic), now, fileID,
		)
		return err
	}); err != nil {
		if publishedNewContent {
			if identifierChanged && oldPath != newPath {
				_ = os.Remove(newPath)
			} else if oldContentBackup != "" {
				_ = util.AtomicWriteFile(oldPath, []byte(oldContentBackup))
			} else {
				_ = os.Remove(newPath)
			}
		} else if movedExistingFile {
			_ = os.Rename(newPath, oldPath)
		}
		return schema.ClientFileMeta{}, err
	}

	// DB committed; now drop the old path when the target moved.
	if publishedNewContent {
		if identifierChanged && oldPath != newPath {
			if _, statErr := os.Stat(oldPath); statErr == nil {
				_ = os.Remove(oldPath)
			}
		}
	}

	return schema.ClientFileMeta{
		ID:          fileID,
		ClientID:    current.ClientID,
		ConfigID:    nextConfigID,
		DisplayName: nextDisplayName,
		Description: nextDescription,
		Ext:         nextExt,
		IsPublic:    nextIsPublic,
		CreatedAt:   current.CreatedAt,
		UpdatedAt:   now,
	}, nil
}

// RestoreClientFile inserts (or upserts) a fully-formed client_files row,
// preserving the caller's ID and timestamps. Intended for one-shot migration
// from a legacy backup; production code paths must keep using CreateClientFile.
//
// Note: this writes metadata only. The caller is responsible for ensuring the
// matching content file exists at clientFilePath().
func (s *Store) RestoreClientFile(ctx context.Context, meta schema.ClientFileMeta) error {
	if meta.ID == "" || meta.ClientID == "" || meta.ConfigID == "" {
		return fmt.Errorf("restore client file: id/clientId/configId required")
	}
	if err := ValidateClientID(meta.ClientID); err != nil {
		return err
	}
	if err := schema.ValidateConfigID(meta.ConfigID); err != nil {
		return err
	}
	desc := sql.NullString{}
	if meta.Description != nil {
		desc.String = *meta.Description
		desc.Valid = true
	}
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO client_files (id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   client_id = excluded.client_id,
			   config_id = excluded.config_id,
			   display_name = excluded.display_name,
			   description = excluded.description,
			   ext = excluded.ext,
			   is_public = excluded.is_public,
			   created_at = excluded.created_at,
			   updated_at = excluded.updated_at`,
			meta.ID, meta.ClientID, meta.ConfigID, meta.DisplayName, desc, meta.Ext, boolToInt(meta.IsPublic), meta.CreatedAt, meta.UpdatedAt,
		)
		return err
	})
}

// DeleteClientFile removes the file row + disk content.
func (s *Store) DeleteClientFile(ctx context.Context, fileID string) error {
	meta, err := s.GetClientFileMeta(ctx, fileID)
	if err != nil {
		return err
	}
	if err := ValidateClientID(meta.ClientID); err != nil {
		return err
	}
	if err := s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx, `DELETE FROM client_files WHERE id = ?`, fileID)
		return err
	}); err != nil {
		return err
	}
	_ = os.Remove(clientFilePath(s.ClientFileDir, meta.ClientID, meta.ConfigID, meta.Ext))
	dir := filepath.Join(s.ClientFileDir, meta.ClientID)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
	return nil
}

// GetClientFileMeta returns the metadata for fileID.
func (s *Store) GetClientFileMeta(ctx context.Context, fileID string) (schema.ClientFileMeta, error) {
	return s.scanClientFileRow(s.DB.QueryRowContext(ctx,
		`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
		 FROM client_files WHERE id = ?`, fileID))
}

// GetClientFileContent returns the on-disk content for the file.
func (s *Store) GetClientFileContent(ctx context.Context, fileID string) (string, error) {
	meta, err := s.GetClientFileMeta(ctx, fileID)
	if err != nil {
		return "", err
	}
	if err := ValidateClientID(meta.ClientID); err != nil {
		return "", err
	}
	data, err := os.ReadFile(clientFilePath(s.ClientFileDir, meta.ClientID, meta.ConfigID, meta.Ext))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListClientFiles returns all metadata for a client.
func (s *Store) ListClientFiles(ctx context.Context, clientID string) ([]schema.ClientFileMeta, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
		 FROM client_files WHERE client_id = ? ORDER BY created_at ASC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.ClientFileMeta
	for rows.Next() {
		m, err := s.scanClientFileRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListAllClientFiles returns metadata for every client_files row, ordered by
// (client_id, created_at). Used by the backup snapshot so a Go-era backup
// preserves displayName / description / isPublic etc. across restore.
func (s *Store) ListAllClientFiles(ctx context.Context) ([]schema.ClientFileMeta, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
		 FROM client_files ORDER BY client_id ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.ClientFileMeta
	for rows.Next() {
		m, err := s.scanClientFileRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListPublicClientFiles returns public files for all clients.
func (s *Store) ListPublicClientFiles(ctx context.Context) ([]schema.ClientFileMeta, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
		 FROM client_files WHERE is_public = 1 ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.ClientFileMeta
	for rows.Next() {
		m, err := s.scanClientFileRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetPublicClientFile returns metadata + content if the file is public.
func (s *Store) GetPublicClientFile(ctx context.Context, clientID, configID, ext string) (schema.ClientFileMeta, string, error) {
	if err := ValidateClientID(clientID); err != nil {
		return schema.ClientFileMeta{}, "", err
	}
	if err := schema.ValidateConfigID(configID); err != nil {
		return schema.ClientFileMeta{}, "", err
	}
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
		 FROM client_files WHERE client_id = ? AND config_id = ? AND ext = ? AND is_public = 1`,
		clientID, configID, ext)
	meta, err := s.scanClientFileRow(row)
	if err != nil {
		if !strings.HasPrefix(ext, ".") {
			row = s.DB.QueryRowContext(ctx,
				`SELECT id, client_id, config_id, display_name, description, ext, is_public, created_at, updated_at
				 FROM client_files WHERE client_id = ? AND config_id = ? AND ext = ? AND is_public = 1`,
				clientID, configID, "."+ext)
			meta, err = s.scanClientFileRow(row)
			if err != nil {
				return meta, "", err
			}
			ext = "." + ext
		} else {
			return meta, "", err
		}
	}
	data, err := os.ReadFile(clientFilePath(s.ClientFileDir, clientID, configID, ext))
	if err != nil {
		return meta, "", err
	}
	return meta, string(data), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanClientFileRow(row rowScanner) (schema.ClientFileMeta, error) {
	var m schema.ClientFileMeta
	var desc sql.NullString
	var pub int
	err := row.Scan(&m.ID, &m.ClientID, &m.ConfigID, &m.DisplayName, &desc, &m.Ext, &pub, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return m, err
	}
	if desc.Valid {
		v := desc.String
		m.Description = &v
	}
	m.IsPublic = pub != 0
	return m, nil
}

func clientFilePath(baseDir, clientID, configID, ext string) string {
	return filepath.Join(baseDir, clientID, fmt.Sprintf("%s.%s", configID, ext))
}

func normalizeClientFileExt(ext string) string {
	return strings.TrimPrefix(ext, ".")
}
