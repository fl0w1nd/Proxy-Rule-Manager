package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// ErrConfigConflict indicates that the caller saved from a stale config rev.
var ErrConfigConflict = errors.New("config revision conflict")

// ErrConfigCorrupted is returned when the persisted config JSON cannot be
// decoded. Callers MUST NOT silently fall back to a default config in this
// case, otherwise a successful PUT /api/config would overwrite the original
// payload with empty defaults and destroy whatever was recoverable.
var ErrConfigCorrupted = errors.New("config payload is corrupted")

// GetConfig returns the persisted RulesConfig (with local sources hydrated).
func (s *Store) GetConfig(ctx context.Context) (schema.RulesConfig, error) {
	cfg, err := s.GetConfigRaw(ctx)
	if err != nil {
		return cfg, err
	}
	return s.hydrateLocalSources(ctx, cfg)
}

// GetConfigRaw returns the persisted RulesConfig without local-source hydration.
func (s *Store) GetConfigRaw(ctx context.Context) (schema.RulesConfig, error) {
	var payload string
	row := s.DB.QueryRowContext(ctx, `SELECT config_json FROM config WHERE id = 1`)
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schema.DefaultConfig(), nil
		}
		return schema.DefaultConfig(), err
	}
	var cfg schema.RulesConfig
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		// Returning ErrConfigCorrupted (instead of silently producing a
		// DefaultConfig) makes the API surface a 500 so an admin notices
		// the problem before a save round-trip wipes their data. The
		// dedicated reset/restore paths bypass GetConfig entirely and
		// remain usable in this state.
		return schema.RulesConfig{}, fmt.Errorf("%w: %v", ErrConfigCorrupted, err)
	}
	cfg.EnsureDefaults()
	return cfg, nil
}

// GetConfigRev returns the current config revision counter.
func (s *Store) GetConfigRev(ctx context.Context) (int64, error) {
	var rev int64
	if err := s.DB.QueryRowContext(ctx, `SELECT rev FROM config WHERE id = 1`).Scan(&rev); err != nil {
		return 0, err
	}
	return rev, nil
}

// SaveConfig persists the validated config, increments rev, externalizes
// inline local-source content, and refreshes the rules index tables.
// Returns the new rev. The whole operation runs in a single SQLite
// transaction under the global write mutex, so concurrent SaveConfig
// callers serialise cleanly without losing updates.
func (s *Store) SaveConfig(ctx context.Context, cfg schema.RulesConfig) (int64, error) {
	return s.saveConfig(ctx, cfg, nil)
}

// SaveConfigWithExpectedRev persists config only when the current rev matches expectedRev.
func (s *Store) SaveConfigWithExpectedRev(ctx context.Context, cfg schema.RulesConfig, expectedRev int64) (int64, error) {
	return s.saveConfig(ctx, cfg, &expectedRev)
}

func (s *Store) saveConfig(ctx context.Context, cfg schema.RulesConfig, expectedRev *int64) (int64, error) {
	cfg.EnsureDefaults()
	if err := ValidateConfigPaths(cfg); err != nil {
		return 0, err
	}

	// Hold the write lock for the whole save so the rev check, SQL write, and
	// post-commit source-file sync stay serialised.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Pass 1: collect local source rows and canonical refs.
	keepRefs := map[string]struct{}{}
	pendingRows := make(map[string]string)
	for ri := range cfg.Rules {
		rule := &cfg.Rules[ri]
		for si := range rule.Sources {
			src := &rule.Sources[si]
			if src.SourceType() != "local" {
				continue
			}
			if src.Content != nil {
				ref := normalizeLocalSourceRef(src.ContentRef)
				if ref == "" {
					ref = newLocalSourceRef()
				}
				pendingRows[ref] = *src.Content
				src.ContentRef = ref
				src.Content = nil
				keepRefs[ref] = struct{}{}
			} else if src.ContentRef != "" {
				if normalized := normalizeLocalSourceRef(src.ContentRef); normalized != "" {
					src.ContentRef = normalized
					keepRefs[normalized] = struct{}{}
				}
			}
		}
	}

	payload, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}

	var (
		newRev    int64
		prunedRef []string
	)
	err = s.txLocked(ctx, func(tx *sql.Tx) error {
		// Upsert local_sources rows.
		for ref, content := range pendingRows {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO local_sources (ref, content) VALUES (?, ?)
				 ON CONFLICT(ref) DO UPDATE SET content = excluded.content`,
				ref, content,
			); err != nil {
				return err
			}
		}
		// Prune obsolete refs.
		rows, err := tx.QueryContext(ctx, `SELECT ref FROM local_sources`)
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
		for _, r := range toDelete {
			if _, err := tx.ExecContext(ctx, `DELETE FROM local_sources WHERE ref = ?`, r); err != nil {
				return err
			}
		}
		prunedRef = toDelete

		// Bump rev.
		var rev int64
		if err := tx.QueryRowContext(ctx, `SELECT rev FROM config WHERE id = 1`).Scan(&rev); err != nil {
			return err
		}
		if expectedRev != nil && rev != *expectedRev {
			return fmt.Errorf("%w: current rev %d, expected rev %d", ErrConfigConflict, rev, *expectedRev)
		}
		rev++
		newRev = rev
		if _, err := tx.ExecContext(ctx,
			`UPDATE config SET config_json = ?, rev = ?, updated_at = ? WHERE id = 1`,
			string(payload), rev, util.NowISO(),
		); err != nil {
			return err
		}

		// Refresh rules + rule_deps indexes.
		if _, err := tx.ExecContext(ctx, `DELETE FROM rules`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_deps`); err != nil {
			return err
		}

		insertRule, err := tx.PrepareContext(ctx, `INSERT INTO rules (name, is_geosite, payload_json, geosite_provider, geosite_list, geosite_attrs_key, position) VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insertRule.Close()
		insertDep, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO rule_deps (rule_name, dep_name) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer insertDep.Close()

		for i, rule := range cfg.Rules {
			ruleJSON, _ := json.Marshal(rule)
			isGeo := schema.IsGeositeRule(&rule)
			var provider, list, attrsKey sql.NullString
			if isGeo {
				src := schema.PrimaryGeositeSource(&rule)
				if src != nil {
					if src.Provider != "" {
						provider = sql.NullString{String: src.Provider, Valid: true}
					}
					list = sql.NullString{String: strings.TrimSpace(src.List), Valid: true}
					if len(src.Attrs) > 0 {
						attrsKey = sql.NullString{String: strings.Join(schema.NormalizeGeositeAttrs(src.Attrs), "+"), Valid: true}
					}
				}
			}
			if _, err := insertRule.ExecContext(ctx, rule.Name, boolToInt(isGeo), string(ruleJSON), provider, list, attrsKey, i); err != nil {
				return err
			}
			for _, src := range rule.Sources {
				if src.SourceType() == "ref" && src.Ref != "" {
					if _, err := insertDep.ExecContext(ctx, rule.Name, src.Ref); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Post-commit: write the current source files, then prune stale ones.
	if s.SourcesDir != "" {
		for ref, content := range pendingRows {
			if err := writeLocalSourceFile(s.SourcesDir, ref, content); err != nil {
				return 0, err
			}
		}
	}
	if s.SourcesDir != "" {
		for _, r := range prunedRef {
			_ = removeLocalSourceFile(s.SourcesDir, r)
		}
	}
	return newRev, nil
}

// ValidateConfigPaths rejects config values that can escape on-disk storage paths.
func ValidateConfigPaths(cfg schema.RulesConfig) error {
	seenRuleNames := map[string]struct{}{}
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if err := util.EnsureSafeSegment(rule.Name, "rule name"); err != nil {
			return err
		}
		if _, ok := seenRuleNames[rule.Name]; ok {
			return fmt.Errorf("duplicate rule name: %s", rule.Name)
		}
		seenRuleNames[rule.Name] = struct{}{}
		for _, clientID := range rule.Output.Clients {
			if err := ValidateClientID(clientID); err != nil {
				return err
			}
		}
		for key := range rule.Output.ClientOverrides {
			if err := ValidateClientID(key); err != nil {
				return err
			}
		}
		for _, src := range rule.Sources {
			switch src.SourceType() {
			case "ref":
				if src.Ref != "" {
					if err := util.EnsureSafeSegment(src.Ref, "rule ref"); err != nil {
						return err
					}
				}
			case "local":
				if src.ContentRef != "" && normalizeLocalSourceRef(src.ContentRef) == "" {
					return fmt.Errorf("invalid local source ref: %q", src.ContentRef)
				}
			case "geosite":
				if src.Provider != "" {
					if err := schema.ValidateGeositeProvider(src.Provider); err != nil {
						return err
					}
				}
				if src.List != "" {
					if err := util.EnsureSafeSegment(src.List, "geosite list"); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// ResetConfig wipes runtime tables and reseeds with the given config + clients.
func (s *Store) ResetConfig(ctx context.Context, cfg schema.RulesConfig, clients []schema.ClientConfig) (int64, error) {
	cfg.EnsureDefaults()
	if clients == nil {
		clients = schema.DefaultClients
	}
	if err := s.WithTx(ctx, func(tx *sql.Tx) error {
		statements := []string{
			`DELETE FROM artifacts`,
			`DELETE FROM jobs`,
			`DELETE FROM change_records`,
			`DELETE FROM failure_records`,
			`DELETE FROM daily_stats`,
			`DELETE FROM bans`,
			`DELETE FROM locks`,
			`DELETE FROM clients`,
			`DELETE FROM client_files`,
			`DELETE FROM rules`,
			`DELETE FROM rule_deps`,
			`DELETE FROM local_sources`,
		}
		for _, sqlStmt := range statements {
			if _, err := tx.ExecContext(ctx, sqlStmt); err != nil {
				return err
			}
		}
		for i, c := range clients {
			transformsJSON, _ := json.Marshal(c.Transforms)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO clients (id, display_name, transforms_json, position) VALUES (?, ?, ?, ?)`,
				c.ID, c.DisplayName, string(transformsJSON), i,
			); err != nil {
				return err
			}
		}
		// reset rev to 0 so SaveConfig below bumps to 1
		if _, err := tx.ExecContext(ctx,
			`UPDATE config SET config_json = ?, rev = 0, updated_at = ? WHERE id = 1`,
			`{"version":1,"transformers":{},"rules":[]}`,
			util.NowISO(),
		); err != nil {
			return err
		}
		// reset last_sync_info
		if payload, err := json.Marshal(schema.DefaultLastSyncInfo()); err == nil {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO kv_settings (key, value_json) VALUES ('last_sync_info', ?)
				 ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json`, string(payload)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return s.SaveConfig(ctx, cfg)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
