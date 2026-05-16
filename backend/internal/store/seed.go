package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// seed ensures singleton rows exist (config, kv settings, default clients).
func (s *Store) seed(ctx context.Context) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		// Config row.
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT config_json FROM config WHERE id = 1`).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			payload, _ := json.Marshal(schema.DefaultConfig())
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO config (id, config_json, rev, updated_at) VALUES (1, ?, 0, ?)`,
				string(payload), util.NowISO(),
			); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		// kv_settings defaults.
		defaults := map[string]any{
			"sync_schedule":  schema.DefaultSyncSchedule(),
			"cdn_settings":   schema.DefaultCdnSettings(),
			"last_sync_info": schema.DefaultLastSyncInfo(),
		}
		for k, v := range defaults {
			var has int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kv_settings WHERE key = ?`, k).Scan(&has); err != nil {
				return err
			}
			if has > 0 {
				continue
			}
			payload, _ := json.Marshal(v)
			if _, err := tx.ExecContext(ctx, `INSERT INTO kv_settings (key, value_json) VALUES (?, ?)`, k, string(payload)); err != nil {
				return err
			}
		}

		// Default clients if table empty.
		var clientCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients`).Scan(&clientCount); err != nil {
			return err
		}
		if clientCount == 0 {
			for i, c := range schema.DefaultClients {
				transformsJSON, _ := json.Marshal(c.Transforms)
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO clients (id, display_name, transforms_json, position) VALUES (?, ?, ?, ?)`,
					c.ID, c.DisplayName, string(transformsJSON), i,
				); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
