package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// ReservedClientDirs blocks IDs that would collide with on-disk reserved
// directories (`rules`, `sources`, `db.json`, `client`).
var ReservedClientDirs = map[string]struct{}{
	"rules":   {},
	"sources": {},
	"db.json": {},
	"client":  {},
}

// GetClients returns all registered clients ordered by their stored position.
func (s *Store) GetClients(ctx context.Context) ([]schema.ClientConfig, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, display_name, transforms_json, output_ext FROM clients ORDER BY position ASC, rowid ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schema.ClientConfig
	for rows.Next() {
		var c schema.ClientConfig
		var tjson sql.NullString
		var ext sql.NullString
		if err := rows.Scan(&c.ID, &c.DisplayName, &tjson, &ext); err != nil {
			return nil, err
		}
		if tjson.Valid && tjson.String != "" {
			_ = json.Unmarshal([]byte(tjson.String), &c.Transforms)
		}
		if ext.Valid {
			c.OutputExt = ext.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddClient inserts a new client. Runs as a single transaction so the
// existence check and the insert are atomic with respect to concurrent
// admin operations.
func (s *Store) AddClient(ctx context.Context, c schema.ClientConfig) error {
	if err := ValidateClientID(c.ID); err != nil {
		return err
	}
	normalizedExt := schema.NormalizeOutputExt(c.OutputExt)
	if err := schema.ValidateOutputExt(normalizedExt); err != nil {
		return err
	}
	// Persist "" for the default so the DB has a single canonical form;
	// legacy rows (no migration value written) already store "".
	storedExt := schema.CanonicalStoredOutputExt(normalizedExt)
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients WHERE id = ?`, c.ID).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return fmt.Errorf(`client with id %q already exists`, c.ID)
		}
		var maxPos sql.NullInt64
		_ = tx.QueryRowContext(ctx, `SELECT MAX(position) FROM clients`).Scan(&maxPos)
		pos := int64(0)
		if maxPos.Valid {
			pos = maxPos.Int64 + 1
		}
		transformsJSON, _ := json.Marshal(c.Transforms)
		_, err := tx.ExecContext(ctx,
			`INSERT INTO clients (id, display_name, transforms_json, output_ext, position) VALUES (?, ?, ?, ?, ?)`,
			c.ID, c.DisplayName, string(transformsJSON), storedExt, pos,
		)
		return err
	})
}

// UpdateClient updates fields and cascades id/ext renames into artifacts,
// client files, on-disk directories, and the config (rules and overrides).
//
// When the client id changes we use a two-phase ordering so the DB and the
// filesystem stay in sync even on partial failure:
//
//  1. Pre-flight: refuse if reserved id, id collision, or any target on-disk
//     directory already exists at the new path (would mask data).
//  2. Move all on-disk directories. If the second move fails, the first is
//     moved back so we leave the filesystem exactly as we found it.
//  3. Commit the DB transaction (rename + cascade artifacts/client_files +
//     rewrite config). If the DB tx fails we move the directories back to
//     their original paths and return the error.
//
// When only outputExt changes we follow the same shape but operate on file
// extensions instead of directory names: walk data/Rules/{id} and rename
// every `*.<oldExt>` to `*.<newExt>`, then update the clients row + cascade
// artifacts blob_path/blob_url, rolling back the file moves if the DB tx
// fails.
//
// This guarantees that a successful return means {DB row == on-disk layout}
// and a failure leaves the filesystem in its original state.
func (s *Store) UpdateClient(ctx context.Context, clientID string, updates schema.ClientConfig) error {
	old, err := s.findClientByID(ctx, clientID)
	if err != nil {
		return err
	}
	newID := updates.ID
	if newID == "" {
		newID = clientID
	}
	if err := ValidateClientID(newID); err != nil {
		return err
	}
	newDisplayName := updates.DisplayName
	if newDisplayName == "" {
		newDisplayName = old.DisplayName
	}
	transforms := updates.Transforms
	if updates.Transforms == nil {
		transforms = old.Transforms
	}
	newExtNormalized := schema.NormalizeOutputExt(updates.OutputExt)
	if err := schema.ValidateOutputExt(newExtNormalized); err != nil {
		return err
	}
	// storedExt collapses "list" back to "" so the persisted column has a
	// single canonical representation for "use the default".
	storedExt := schema.CanonicalStoredOutputExt(newExtNormalized)
	oldResolvedExt := old.ResolvedOutputExt()
	newResolvedExt := newExtNormalized
	if newResolvedExt == "" {
		newResolvedExt = schema.DefaultOutputExt
	}

	idChanged := newID != clientID
	extChanged := oldResolvedExt != newResolvedExt

	if !idChanged {
		if !extChanged {
			return s.WithTx(ctx, func(tx *sql.Tx) error {
				transformsJSON, _ := json.Marshal(transforms)
				_, err := tx.ExecContext(ctx,
					`UPDATE clients SET display_name = ?, transforms_json = ?, output_ext = ? WHERE id = ?`,
					newDisplayName, string(transformsJSON), storedExt, clientID,
				)
				return err
			})
		}
		// Ext-only change: rename files first, then update DB.
		moved, err := renameClientArtifactFiles(s.RulesDir, clientID, oldResolvedExt, newResolvedExt)
		if err != nil {
			rollbackArtifactFileRenames(moved)
			return err
		}
		if err := s.WithTx(ctx, func(tx *sql.Tx) error {
			transformsJSON, _ := json.Marshal(transforms)
			if _, err := tx.ExecContext(ctx,
				`UPDATE clients SET display_name = ?, transforms_json = ?, output_ext = ? WHERE id = ?`,
				newDisplayName, string(transformsJSON), storedExt, clientID,
			); err != nil {
				return err
			}
			return cascadeArtifactExt(ctx, tx, clientID, oldResolvedExt, newResolvedExt)
		}); err != nil {
			rollbackArtifactFileRenames(moved)
			return fmt.Errorf("commit client ext change: %w", err)
		}
		return nil
	}

	// --- ID change: pre-flight validation ---
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients WHERE id = ?`, newID).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return fmt.Errorf(`client with id %q already exists`, newID)
	}

	type dirMove struct {
		oldPath string
		newPath string
		moved   bool // true once Rename succeeded
	}
	moves := []*dirMove{
		{oldPath: filepath.Join(s.RulesDir, clientID), newPath: filepath.Join(s.RulesDir, newID)},
		{oldPath: filepath.Join(s.ClientFileDir, clientID), newPath: filepath.Join(s.ClientFileDir, newID)},
	}
	// Pre-check: refuse if any target dir already exists. This avoids
	// silently mixing two clients' files together.
	for _, m := range moves {
		if _, err := os.Stat(m.newPath); err == nil {
			return fmt.Errorf(`target directory already exists for client %q: %s`, newID, m.newPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", m.newPath, err)
		}
	}

	rollbackMoves := func() {
		for i := len(moves) - 1; i >= 0; i-- {
			m := moves[i]
			if !m.moved {
				continue
			}
			if rerr := os.Rename(m.newPath, m.oldPath); rerr != nil {
				log.Printf("[client rename] WARNING: rollback %s -> %s failed: %v", m.newPath, m.oldPath, rerr)
			} else {
				m.moved = false
			}
		}
	}

	// --- Move directories first ---
	for _, m := range moves {
		if _, err := os.Stat(m.oldPath); os.IsNotExist(err) {
			// Nothing to move for this side.
			continue
		}
		if err := os.Rename(m.oldPath, m.newPath); err != nil {
			rollbackMoves()
			return fmt.Errorf("rename %s -> %s: %w", m.oldPath, m.newPath, err)
		}
		m.moved = true
	}

	// --- Optional ext rename inside the now-renamed dir ---
	var movedExtFiles []renamedFile
	if extChanged {
		movedExtFiles, err = renameClientArtifactFiles(s.RulesDir, newID, oldResolvedExt, newResolvedExt)
		if err != nil {
			rollbackArtifactFileRenames(movedExtFiles)
			rollbackMoves()
			return err
		}
	}

	// --- Commit DB changes; roll back filesystem on failure ---
	if err := s.WithTx(ctx, func(tx *sql.Tx) error {
		transformsJSON, _ := json.Marshal(transforms)
		if _, err := tx.ExecContext(ctx,
			`UPDATE clients SET id = ?, display_name = ?, transforms_json = ?, output_ext = ? WHERE id = ?`,
			newID, newDisplayName, string(transformsJSON), storedExt, clientID,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE artifacts SET client = ?, blob_path = REPLACE(blob_path, ?, ?), blob_url = REPLACE(COALESCE(blob_url, ''), ?, ?) WHERE client = ?`,
			newID,
			"/Rules/"+clientID+"/", "/Rules/"+newID+"/",
			"/Rules/"+clientID+"/", "/Rules/"+newID+"/",
			clientID,
		); err != nil {
			return err
		}
		if extChanged {
			if err := cascadeArtifactExt(ctx, tx, newID, oldResolvedExt, newResolvedExt); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_files SET client_id = ? WHERE client_id = ?`,
			newID, clientID,
		); err != nil {
			return err
		}

		var configJSON string
		if err := tx.QueryRowContext(ctx, `SELECT config_json FROM config WHERE id = 1`).Scan(&configJSON); err != nil {
			return err
		}
		var cfg schema.RulesConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return err
		}
		for ri := range cfg.Rules {
			out := &cfg.Rules[ri].Output
			for i, cid := range out.Clients {
				if cid == clientID {
					out.Clients[i] = newID
				}
			}
			if override, ok := out.ClientOverrides[clientID]; ok {
				if out.ClientOverrides == nil {
					out.ClientOverrides = map[string]schema.ClientOutputOverride{}
				}
				out.ClientOverrides[newID] = override
				delete(out.ClientOverrides, clientID)
			}
		}
		payload, _ := json.Marshal(cfg)
		if _, err := tx.ExecContext(ctx, `UPDATE config SET config_json = ? WHERE id = 1`, string(payload)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		rollbackArtifactFileRenames(movedExtFiles)
		rollbackMoves()
		return fmt.Errorf("commit client rename: %w", err)
	}
	return nil
}

// renamedFile records a successful file rename for rollback.
type renamedFile struct {
	old string
	new string
}

// renameClientArtifactFiles walks data/Rules/{clientID} recursively and
// renames every regular file ending in `.{oldExt}` to use `.{newExt}`. The
// returned slice lists the successful renames so the caller can roll back
// when a later step fails. The walk is aborted on the first error and the
// already-completed renames are returned so the caller can still undo them.
func renameClientArtifactFiles(rulesDir, clientID, oldExt, newExt string) ([]renamedFile, error) {
	if oldExt == newExt {
		return nil, nil
	}
	root := filepath.Join(rulesDir, clientID)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	oldSuffix := "." + oldExt
	newSuffix := "." + newExt
	var moved []renamedFile
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), oldSuffix) {
			return nil
		}
		newPath := strings.TrimSuffix(path, oldSuffix) + newSuffix
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf(`target artifact already exists: %s`, newPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", newPath, err)
		}
		if err := os.Rename(path, newPath); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", path, newPath, err)
		}
		moved = append(moved, renamedFile{old: path, new: newPath})
		return nil
	})
	if walkErr != nil {
		return moved, walkErr
	}
	return moved, nil
}

// rollbackArtifactFileRenames reverses the moves recorded by
// renameClientArtifactFiles. Best-effort; logs warnings on failure so the
// caller never panics partway through a multi-step UpdateClient rollback.
func rollbackArtifactFileRenames(moved []renamedFile) {
	for i := len(moved) - 1; i >= 0; i-- {
		p := moved[i]
		if err := os.Rename(p.new, p.old); err != nil {
			log.Printf("[client ext] WARNING: rollback %s -> %s failed: %v", p.new, p.old, err)
		}
	}
}

// cascadeArtifactExt updates blob_path/blob_url on every artifact row for
// the given client so the stored URLs match the renamed on-disk files.
// Must run inside a transaction; relies on the path/URL ending in
// `.<oldExt>` (true for every row written by ruledisk.go).
func cascadeArtifactExt(ctx context.Context, tx *sql.Tx, clientID, oldExt, newExt string) error {
	if oldExt == newExt {
		return nil
	}
	type artifactRow struct {
		ruleName string
		path     string
		url      sql.NullString
	}
	items, err := func() ([]artifactRow, error) {
		rows, err := tx.QueryContext(ctx,
			`SELECT rule_name, blob_path, blob_url FROM artifacts WHERE client = ?`, clientID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []artifactRow
		for rows.Next() {
			var it artifactRow
			if err := rows.Scan(&it.ruleName, &it.path, &it.url); err != nil {
				return nil, err
			}
			out = append(out, it)
		}
		return out, rows.Err()
	}()
	if err != nil {
		return err
	}
	oldSuffix := "." + oldExt
	newSuffix := "." + newExt
	for _, it := range items {
		newPath := it.path
		if strings.HasSuffix(newPath, oldSuffix) {
			newPath = strings.TrimSuffix(newPath, oldSuffix) + newSuffix
		}
		var newURL sql.NullString
		if it.url.Valid {
			u := it.url.String
			if strings.HasSuffix(u, oldSuffix) {
				u = strings.TrimSuffix(u, oldSuffix) + newSuffix
			}
			newURL = sql.NullString{String: u, Valid: true}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE artifacts SET blob_path = ?, blob_url = ? WHERE rule_name = ? AND client = ?`,
			newPath, newURL, it.ruleName, clientID,
		); err != nil {
			return err
		}
	}
	return nil
}

// DeleteClient removes a client, its artifacts, files, and references in the config.
func (s *Store) DeleteClient(ctx context.Context, clientID string) error {
	if err := ValidateClientID(clientID); err != nil {
		return err
	}
	if _, err := s.findClientByID(ctx, clientID); err != nil {
		return err
	}
	if err := s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM artifacts WHERE client = ?`, clientID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM client_files WHERE client_id = ?`, clientID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM clients WHERE id = ?`, clientID); err != nil {
			return err
		}

		// Rewrite config to drop references.
		var configJSON string
		if err := tx.QueryRowContext(ctx, `SELECT config_json FROM config WHERE id = 1`).Scan(&configJSON); err != nil {
			return err
		}
		var cfg schema.RulesConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return err
		}
		for ri := range cfg.Rules {
			out := &cfg.Rules[ri].Output
			pruned := out.Clients[:0]
			for _, cid := range out.Clients {
				if cid != clientID {
					pruned = append(pruned, cid)
				}
			}
			out.Clients = pruned
			delete(out.ClientOverrides, clientID)
		}
		payload, _ := json.Marshal(cfg)
		if _, err := tx.ExecContext(ctx, `UPDATE config SET config_json = ? WHERE id = 1`, string(payload)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Remove on-disk artifacts & files.
	_ = os.RemoveAll(filepath.Join(s.RulesDir, clientID))
	_ = os.RemoveAll(filepath.Join(s.ClientFileDir, clientID))
	return nil
}

func (s *Store) findClientByID(ctx context.Context, clientID string) (schema.ClientConfig, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, display_name, transforms_json, output_ext FROM clients WHERE id = ?`, clientID)
	var c schema.ClientConfig
	var tjson sql.NullString
	var ext sql.NullString
	if err := row.Scan(&c.ID, &c.DisplayName, &tjson, &ext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, fmt.Errorf(`client %q not found`, clientID)
		}
		return c, err
	}
	if tjson.Valid && tjson.String != "" {
		_ = json.Unmarshal([]byte(tjson.String), &c.Transforms)
	}
	if ext.Valid {
		c.OutputExt = ext.String
	}
	return c, nil
}

// IsReservedClientID reports whether the given id can be used.
func IsReservedClientID(id string) bool {
	_, ok := ReservedClientDirs[strings.ToLower(id)]
	return ok
}

// ValidateClientID rejects client IDs that can escape on-disk storage paths.
func ValidateClientID(id string) error {
	if err := util.EnsureSafeSegment(id, "client id"); err != nil {
		return err
	}
	if IsReservedClientID(id) {
		return fmt.Errorf(`client id %q is reserved`, id)
	}
	return nil
}
