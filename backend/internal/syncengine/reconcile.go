package syncengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

// ConsistencyIssue represents one cross-medium drift between SQLite and the
// filesystem. The detection-only pass produces these; a future repair pass
// can act on them, but the first version is read-only by design so we can
// observe what's actually broken before automating destructive cleanup.
type ConsistencyIssue struct {
	Type     string `json:"type"`     // e.g. "artifact_file_missing"
	Severity string `json:"severity"` // "info" | "warning" | "error"
	Path     string `json:"path,omitempty"`
	RuleName string `json:"ruleName,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	FileID   string `json:"fileId,omitempty"`
	Message  string `json:"message"`
}

// ConsistencyReport is the aggregated output of CheckConsistency.
type ConsistencyReport struct {
	Issues     []ConsistencyIssue `json:"issues"`
	Checked    int                `json:"checked"`
	GeneratedA string             `json:"generatedAt"`
}

// CheckConsistency inspects the DB and filesystem for drift. It is read-only
// and safe to call from any goroutine; it bypasses the engine's sync lock
// because it does not mutate anything.
//
// Detections (first pass):
//   - artifact_file_missing: artifact meta points at a non-existent file.
//   - artifact_orphan: on-disk rule artifact with no matching artifact row.
//     Catches stray files of any extension; useful for spotting leftovers
//     from a previous client.outputExt that wasn't fully rolled forward.
//   - client_file_missing: client_files row with no matching file on disk.
//   - client_file_orphan: on-disk client file with no matching DB row.
//   - client_dir_orphan: Rules/<client> or client/<client> dir for a client
//     id that no longer exists in the clients table.
//   - rule_artifact_mismatch: config has a rule whose artifact is missing
//     for one of its declared clients.
//   - temp_file: leftover .*.tmp file from a crashed atomic write.
func CheckConsistency(ctx context.Context, st *store.Store, rulesDir, clientFileDir string) (ConsistencyReport, error) {
	report := ConsistencyReport{}

	cfg, err := st.GetConfig(ctx)
	if err != nil {
		return report, fmt.Errorf("read config: %w", err)
	}
	clients, err := st.GetClients(ctx)
	if err != nil {
		return report, fmt.Errorf("read clients: %w", err)
	}
	clientSet := map[string]struct{}{}
	clientExt := map[string]string{}
	for _, c := range clients {
		clientSet[c.ID] = struct{}{}
		clientExt[c.ID] = c.ResolvedOutputExt()
	}

	arts, err := st.GetAllArtifactMetas(ctx)
	if err != nil {
		return report, fmt.Errorf("read artifacts: %w", err)
	}
	clientFiles, err := st.ListAllClientFiles(ctx)
	if err != nil {
		return report, fmt.Errorf("read client files: %w", err)
	}

	// Index rules by name so we can resolve geosite vs. plain rules when
	// computing a meta's expected file path.
	ruleByName := map[string]*schema.RuleConfig{}
	for i := range cfg.Rules {
		ruleByName[cfg.Rules[i].Name] = &cfg.Rules[i]
	}

	// --- 1. artifact meta → file on disk ---
	artifactFiles := map[string]struct{}{}
	for _, art := range arts {
		report.Checked++
		rule, ok := ruleByName[art.RuleName]
		if !ok {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "artifact_without_rule",
				Severity: "warning",
				RuleName: art.RuleName,
				ClientID: art.Client,
				Message:  "artifact references a rule that no longer exists in config",
			})
			continue
		}
		path, perr := artifactFilePath(rulesDir, rule, art.Client, clientExt[art.Client])
		if perr != nil {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "artifact_path_error",
				Severity: "warning",
				RuleName: art.RuleName,
				ClientID: art.Client,
				Message:  perr.Error(),
			})
			continue
		}
		artifactFiles[path] = struct{}{}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "artifact_file_missing",
				Severity: "error",
				Path:     path,
				RuleName: art.RuleName,
				ClientID: art.Client,
				Message:  "artifact meta exists but the on-disk file is missing",
			})
		} else if err != nil {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "artifact_file_stat_error",
				Severity: "warning",
				Path:     path,
				RuleName: art.RuleName,
				ClientID: art.Client,
				Message:  err.Error(),
			})
		}
	}

	// --- 2. config rule × output client → expected artifact present ---
	for ri := range cfg.Rules {
		rule := &cfg.Rules[ri]
		for _, clientID := range rule.Output.Clients {
			path, perr := artifactFilePath(rulesDir, rule, clientID, clientExt[clientID])
			if perr != nil {
				continue
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				report.Issues = append(report.Issues, ConsistencyIssue{
					Type:     "rule_artifact_missing",
					Severity: "warning",
					Path:     path,
					RuleName: rule.Name,
					ClientID: clientID,
					Message:  "rule declares this client but artifact has not been produced",
				})
			}
		}
	}

	// --- 3. orphan files in Rules/ ---
	_ = filepath.WalkDir(rulesDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		// Leftover temp files from a crashed AtomicWriteFile / WriteTempFile.
		if strings.HasPrefix(base, ".") && strings.HasSuffix(base, ".tmp") {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "temp_file",
				Severity: "info",
				Path:     path,
				Message:  "leftover temp file from a crashed atomic write",
			})
			return nil
		}
		// Every file under data/Rules/ is supposed to be a rule artifact;
		// extension may be anything per client.outputExt. Anything not in
		// our authoritative set is an orphan worth reporting.
		if _, known := artifactFiles[path]; !known {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "artifact_orphan",
				Severity: "info",
				Path:     path,
				Message:  "on-disk artifact has no matching artifact row",
			})
		}
		return nil
	})

	// --- 4. client_files ↔ files on disk ---
	clientFilePaths := map[string]struct{}{}
	for _, cf := range clientFiles {
		report.Checked++
		path := filepath.Join(clientFileDir, cf.ClientID, fmt.Sprintf("%s.%s", cf.ConfigID, cf.Ext))
		clientFilePaths[path] = struct{}{}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "client_file_missing",
				Severity: "error",
				Path:     path,
				ClientID: cf.ClientID,
				FileID:   cf.ID,
				Message:  "client_files row references a missing on-disk file",
			})
		} else if err != nil {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "client_file_stat_error",
				Severity: "warning",
				Path:     path,
				ClientID: cf.ClientID,
				FileID:   cf.ID,
				Message:  err.Error(),
			})
		}
	}

	_ = filepath.WalkDir(clientFileDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") && strings.HasSuffix(base, ".tmp") {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "temp_file",
				Severity: "info",
				Path:     path,
				Message:  "leftover temp file from a crashed atomic write",
			})
			return nil
		}
		if _, known := clientFilePaths[path]; !known {
			report.Issues = append(report.Issues, ConsistencyIssue{
				Type:     "client_file_orphan",
				Severity: "info",
				Path:     path,
				Message:  "on-disk client file has no matching client_files row",
			})
		}
		return nil
	})

	// --- 5. orphan client directories (rules + client_files trees) ---
	for _, base := range []string{rulesDir, clientFileDir} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, ok := store.ReservedClientDirs[e.Name()]; ok {
				continue
			}
			if _, ok := clientSet[e.Name()]; !ok {
				report.Issues = append(report.Issues, ConsistencyIssue{
					Type:     "client_dir_orphan",
					Severity: "warning",
					Path:     filepath.Join(base, e.Name()),
					ClientID: e.Name(),
					Message:  "directory belongs to a client that no longer exists",
				})
			}
		}
	}

	return report, nil
}
