// Package store implements the SQLite-backed persistence layer.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// Store wraps a *sql.DB plus filesystem paths.
//
// writeMu serialises ALL writes that touch SQLite. modernc.org/sqlite
// supports concurrent connections, but only one writer at a time; under
// concurrent admin operations we observed SQLITE_BUSY despite the
// busy_timeout pragma. Serialising on the Go side makes the bottleneck
// explicit, eliminates BUSY errors, and lets us safely chain
// read-modify-write operations (config, kv_settings, local_sources, ...)
// without losing updates.
type Store struct {
	DB            *sql.DB
	DataDir       string
	RulesDir      string
	SourcesDir    string
	GeositeDir    string
	IconSetDir    string
	ClientFileDir string
	WAFDir        string

	writeMu sync.Mutex
}

// Open opens (or creates) the SQLite database at dbPath, applies migrations, and
// ensures all directory paths exist.
func Open(dbPath string, paths Paths) (*Store, error) {
	if err := util.EnsureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}
	for _, dir := range []string{paths.RulesDir, paths.SourcesDir, paths.GeositeDir, paths.IconSetDir, paths.ClientFileDir, paths.WAFDir} {
		if err := util.EnsureDir(dir); err != nil {
			return nil, fmt.Errorf("ensure dir %s: %w", dir, err)
		}
	}

	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite serializes writes via a single connection; allow many readers.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	s := &Store{
		DB:            db,
		DataDir:       paths.DataDir,
		RulesDir:      paths.RulesDir,
		SourcesDir:    paths.SourcesDir,
		GeositeDir:    paths.GeositeDir,
		IconSetDir:    paths.IconSetDir,
		ClientFileDir: paths.ClientFileDir,
		WAFDir:        paths.WAFDir,
	}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.seed(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return s, nil
}

// Paths bundles the on-disk locations the store coordinates with.
type Paths struct {
	DataDir       string
	RulesDir      string
	SourcesDir    string
	GeositeDir    string
	IconSetDir    string
	ClientFileDir string
	WAFDir        string
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// WithTx runs fn inside a write transaction, serialised across the process.
// The Go-level write mutex prevents two goroutines from racing on the same
// SQLite write lock and avoids SQLITE_BUSY under contention.
func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.txLocked(ctx, fn)
}

// txLocked is the inner implementation that assumes writeMu is already held.
func (s *Store) txLocked(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// withWriteLock serialises a non-transactional write block.
func (s *Store) withWriteLock(fn func() error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return fn()
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  config_json TEXT NOT NULL,
  rev INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS rules (
  name TEXT PRIMARY KEY,
  is_geosite INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  geosite_provider TEXT,
  geosite_list TEXT,
  geosite_attrs_key TEXT,
  position INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_rules_geosite ON rules(is_geosite);
CREATE INDEX IF NOT EXISTS idx_rules_position ON rules(position);

CREATE TABLE IF NOT EXISTS rule_deps (
  rule_name TEXT NOT NULL,
  dep_name TEXT NOT NULL,
  PRIMARY KEY (rule_name, dep_name)
);
CREATE INDEX IF NOT EXISTS idx_rule_deps_dep ON rule_deps(dep_name);

CREATE TABLE IF NOT EXISTS clients (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  transforms_json TEXT,
  output_ext TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_clients_position ON clients(position);

CREATE TABLE IF NOT EXISTS client_files (
  id TEXT PRIMARY KEY,
  client_id TEXT NOT NULL,
  config_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  description TEXT,
  ext TEXT NOT NULL,
  is_public INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(client_id, config_id, ext)
);
CREATE INDEX IF NOT EXISTS idx_client_files_client ON client_files(client_id);
CREATE INDEX IF NOT EXISTS idx_client_files_public ON client_files(is_public);

CREATE TABLE IF NOT EXISTS artifacts (
  rule_name TEXT NOT NULL,
  client TEXT NOT NULL,
  last_hash TEXT NOT NULL,
  last_updated_at TEXT NOT NULL,
  blob_path TEXT NOT NULL,
  blob_url TEXT,
  size_bytes INTEGER,
  last_attempted_at TEXT NOT NULL DEFAULT '',
  last_attempt_status TEXT NOT NULL DEFAULT '',
  last_attempt_error TEXT NOT NULL DEFAULT '',
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (rule_name, client)
);
CREATE INDEX IF NOT EXISTS idx_artifacts_rule ON artifacts(rule_name);

CREATE TABLE IF NOT EXISTS jobs (
  job_id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  affected_rules_json TEXT,
  changed_rules_json TEXT,
  failed_rules_json TEXT,
  logs_json TEXT
);

CREATE TABLE IF NOT EXISTS change_records (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  timestamp TEXT NOT NULL,
  rule_name TEXT NOT NULL,
  client TEXT NOT NULL,
  change_type TEXT NOT NULL,
  size_bytes INTEGER,
  diff TEXT
);
CREATE INDEX IF NOT EXISTS idx_change_date_ts ON change_records(date, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_change_rule ON change_records(rule_name);

CREATE TABLE IF NOT EXISTS failure_records (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  timestamp TEXT NOT NULL,
  rule_name TEXT NOT NULL,
  client TEXT,
  source TEXT,
  message TEXT NOT NULL,
  stage TEXT NOT NULL,
  job_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_failure_date_ts ON failure_records(date, timestamp DESC);

CREATE TABLE IF NOT EXISTS bans (
  ip TEXT PRIMARY KEY,
  reason TEXT NOT NULL,
  banned_at TEXT NOT NULL,
  expires_at TEXT,
  fail_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS local_sources (
  ref TEXT PRIMARY KEY,
  content TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS kv_settings (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS daily_stats (
  date TEXT PRIMARY KEY,
  sync_count INTEGER NOT NULL DEFAULT 0,
  blob_write_count INTEGER NOT NULL DEFAULT 0,
  rules_changed INTEGER NOT NULL DEFAULT 0,
  total_rules_processed INTEGER NOT NULL DEFAULT 0,
  failed_sources INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS locks (
  key TEXT PRIMARY KEY,
  expires_at INTEGER NOT NULL
);
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	// Idempotent ALTER TABLE migrations for older databases.
	addColumns := []struct {
		table string
		col   string
		def   string
	}{
		{"artifacts", "last_attempted_at", "TEXT NOT NULL DEFAULT ''"},
		{"artifacts", "last_attempt_status", "TEXT NOT NULL DEFAULT ''"},
		{"artifacts", "last_attempt_error", "TEXT NOT NULL DEFAULT ''"},
		{"artifacts", "consecutive_failures", "INTEGER NOT NULL DEFAULT 0"},
		// Custom output extension per client; blank = use schema.DefaultOutputExt.
		{"clients", "output_ext", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range addColumns {
		if err := s.ensureColumn(ctx, c.table, c.col, c.def); err != nil {
			return err
		}
	}
	return nil
}

// ensureColumn adds `col` to `table` if it does not already exist. Older
// databases that predate B1 will pick up the new artifact-attempt fields here.
func (s *Store) ensureColumn(ctx context.Context, table, col, def string) error {
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, def))
	return err
}
