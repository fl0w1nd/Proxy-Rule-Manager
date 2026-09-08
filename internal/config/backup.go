package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const (
	maxConfigBackups  = 20
	backupTimeLayout  = "20060102-150405"
	backupNamePattern = `^config-(\d{8}-\d{6})(?:-(\d+))?\.yaml$`
)

var (
	ErrBackupNotFound = errors.New("config backup not found")
	backupName        = regexp.MustCompile(backupNamePattern)
)

// Backup is one retained copy of the managed config source.
type Backup struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Size      int64  `json:"size"`
	Added     int    `json:"added"`
	Removed   int    `json:"removed"`
}

// BackupDetail is one snapshot plus its line diff against the current source.
type BackupDetail struct {
	Backup
	YAML  string            `json:"yaml"`
	Lines []util.LineChange `json:"lines"`
}

type backupFile struct {
	name    string
	created time.Time
	seq     int
}

func (m *Manager) backupDir() (string, bool) {
	if m.dataDir == "" {
		return "", false
	}
	return filepath.Join(m.dataDir, ".state", "backups"), true
}

// SaveBackup writes the current managed source to the rolling backup directory.
func (m *Manager) SaveBackup(now time.Time) error {
	m.mu.RLock()
	raw := append([]byte(nil), m.raw...)
	dataDir, path := m.dataDir, m.path
	m.mu.RUnlock()
	if path == "" || dataDir == "" {
		return nil
	}
	dir := filepath.Join(dataDir, ".state", "backups")
	if err := util.EnsureDir(dir); err != nil {
		return fmt.Errorf("create config backups dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read config backups dir: %w", err)
	}
	used := make(map[string]bool, len(entries))
	backups := make([]backupFile, 0, len(entries)+1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		used[name] = true
		created, seq, ok := parseBackupName(name)
		if !ok {
			continue
		}
		backups = append(backups, backupFile{name: name, created: created, seq: seq})
	}
	name := uniqueBackupName(now, used)
	created, seq, ok := parseBackupName(name)
	if !ok {
		return fmt.Errorf("invalid config backup name %q", name)
	}
	if err := util.AtomicWriteFile(filepath.Join(dir, name), raw); err != nil {
		return fmt.Errorf("write config backup: %w", err)
	}
	backups = append(backups, backupFile{name: name, created: created, seq: seq})
	sortBackupFiles(backups)
	if extra := len(backups) - maxConfigBackups; extra > 0 {
		for _, old := range backups[:extra] {
			if err := os.Remove(filepath.Join(dir, old.name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove old config backup: %w", err)
			}
		}
	}
	return nil
}

// ListBackups returns retained backups, newest first.
func (m *Manager) ListBackups() ([]Backup, error) {
	dir, ok := m.backupDir()
	if !ok {
		return []Backup{}, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config backups dir: %w", err)
	}
	m.mu.RLock()
	current := string(m.raw)
	m.mu.RUnlock()
	files := make([]backupFile, 0, len(entries))
	details := make(map[string]Backup, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		created, seq, ok := parseBackupName(name)
		if !ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read config backup %s: %w", name, err)
		}
		diff := util.DiffLines(current, string(raw))
		files = append(files, backupFile{name: name, created: created, seq: seq})
		details[name] = Backup{
			ID: name, CreatedAt: util.FormatISO(created), Size: int64(len(raw)),
			Added: diff.Added, Removed: diff.Removed,
		}
	}
	sortBackupFiles(files)
	items := make([]Backup, 0, len(files))
	for i := len(files) - 1; i >= 0; i-- {
		items = append(items, details[files[i].name])
	}
	return items, nil
}

// BackupDetail returns one snapshot and the line diff that restore would apply.
func (m *Manager) BackupDetail(id string) (*BackupDetail, error) {
	raw, err := m.ReadBackup(id)
	if err != nil {
		return nil, err
	}
	created, _, ok := parseBackupName(id)
	if !ok {
		return nil, ErrBackupNotFound
	}
	m.mu.RLock()
	current := string(m.raw)
	m.mu.RUnlock()
	diff := util.DiffLines(current, string(raw))
	if diff.Lines == nil {
		diff.Lines = []util.LineChange{}
	}
	return &BackupDetail{
		Backup: Backup{
			ID: id, CreatedAt: util.FormatISO(created), Size: int64(len(raw)),
			Added: diff.Added, Removed: diff.Removed,
		},
		YAML: string(raw), Lines: diff.Lines,
	}, nil
}

// ReadBackup returns the source bytes of one backup.
func (m *Manager) ReadBackup(id string) ([]byte, error) {
	if _, _, ok := parseBackupName(id); !ok {
		return nil, ErrBackupNotFound
	}
	if err := util.EnsureSafeSegment(id, "backup"); err != nil {
		return nil, ErrBackupNotFound
	}
	dir, ok := m.backupDir()
	if !ok {
		return nil, ErrBackupNotFound
	}
	path, err := util.JoinInside(dir, id)
	if err != nil {
		return nil, ErrBackupNotFound
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrBackupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read config backup: %w", err)
	}
	return raw, nil
}

func uniqueBackupName(now time.Time, used map[string]bool) string {
	stamp := now.UTC().Format(backupTimeLayout)
	name := "config-" + stamp + ".yaml"
	if !used[name] {
		return name
	}
	for n := 2; ; n++ {
		name = fmt.Sprintf("config-%s-%d.yaml", stamp, n)
		if !used[name] {
			return name
		}
	}
}

func parseBackupName(name string) (time.Time, int, bool) {
	match := backupName.FindStringSubmatch(name)
	if match == nil {
		return time.Time{}, 0, false
	}
	created, err := time.ParseInLocation(backupTimeLayout, match[1], time.UTC)
	if err != nil {
		return time.Time{}, 0, false
	}
	seq := 1
	if match[2] != "" {
		seq, err = strconv.Atoi(match[2])
		if err != nil || seq < 2 {
			return time.Time{}, 0, false
		}
	}
	return created, seq, true
}

func sortBackupFiles(files []backupFile) {
	sort.Slice(files, func(i, j int) bool {
		if !files[i].created.Equal(files[j].created) {
			return files[i].created.Before(files[j].created)
		}
		return files[i].seq < files[j].seq
	})
}
