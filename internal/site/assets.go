package site

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// assetRenderVersion is part of the asset fingerprint. Bump it whenever the
// rendering logic changes the generated output without touching any embedded
// asset (template funcs, view-model fields, ...), so serve regenerates pages
// after such an upgrade.
const assetRenderVersion = 2

// assetManifest records what UpdateBuiltinAssets wrote, enabling a three-way
// merge on later runs: files the user modified since the last managed write
// are preserved, orphaned files the user never touched are removed.
type assetManifest struct {
	Fingerprint string            `json:"fingerprint"`
	Files       map[string]string `json:"files"` // path relative to icons/ -> sha256 hex
}

// AssetUpdateResult reports what UpdateBuiltinAssets did.
type AssetUpdateResult struct {
	Changed            bool     // any file written or deleted
	FingerprintChanged bool     // embedded asset fingerprint differs from the manifest
	Written            []string // files (re)written, relative to icons/
	Skipped            []string // user-modified builtin files left untouched
	Deleted            []string // orphaned files removed
}

// AssetFingerprint hashes every embedded asset (icons + page templates) plus
// the render version. It identifies the binary's asset set, independent of
// whatever users placed under data/static/icons/.
func AssetFingerprint() string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "render-version:%d\n", assetRenderVersion)
	for _, f := range embeddedAssets() {
		_, _ = fmt.Fprintf(h, "%s:%s\n", f.rel, f.hash)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type embeddedAsset struct {
	rel     string // path relative to icons/ (icons) or template name
	content []byte
	hash    string
	shared  bool // templates are fingerprint-only; they are not written to icons/
}

// embeddedAssets enumerates the embedded icon files plus the page templates,
// sorted for a stable fingerprint.
func embeddedAssets() []embeddedAsset {
	var out []embeddedAsset
	_ = fs.WalkDir(iconAssets, "assets/icons", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := iconAssets.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("assets/icons", path)
		out = append(out, embeddedAsset{rel: rel, content: data, hash: hashBytes(data)})
		return nil
	})
	for _, name := range []string{IndexFileTemplate} {
		data, err := htmlFS.ReadFile(name)
		if err != nil {
			continue
		}
		out = append(out, embeddedAsset{rel: name, content: data, hash: hashBytes(data), shared: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// UpdateBuiltinAssets reconciles {staticDir}/icons/ with the embedded builtin
// icons using the manifest from the previous run:
//
//   - missing files are written
//   - files still matching the last managed write are upgraded in place
//   - files the user modified are preserved (reported via Skipped)
//   - files no longer embedded and never user-modified are removed
//
// User files with names outside the builtin set are never touched.
func UpdateBuiltinAssets(staticDir string) (AssetUpdateResult, error) {
	var res AssetUpdateResult
	iconsDir := filepath.Join(staticDir, "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return res, fmt.Errorf("create icons dir: %w", err)
	}

	old, oldErr := readAssetManifest(staticDir)
	managed := oldErr == nil && old.Files != nil
	res.FingerprintChanged = !managed || old.Fingerprint != AssetFingerprint()

	var icons []embeddedAsset
	newFiles := make(map[string]string)
	for _, a := range embeddedAssets() {
		if a.shared {
			continue
		}
		icons = append(icons, a)
		newFiles[a.rel] = a.hash
	}

	for _, icon := range icons {
		dest := filepath.Join(iconsDir, filepath.FromSlash(icon.rel))
		disk, err := os.ReadFile(dest)
		if err == nil {
			diskHash := hashBytes(disk)
			switch {
			case diskHash == icon.hash:
				continue // up to date
			case !managed:
				// No manifest (pre-manifest install): the previous writer was
				// unconditional, so disk content is not distinguishable from an
				// old builtin version. Overwrite once, then track via manifest.
			case old.Files[icon.rel] == diskHash:
				// Unmodified since the last managed write: safe to upgrade.
			default:
				res.Skipped = append(res.Skipped, icon.rel)
				continue
			}
		} else if !os.IsNotExist(err) {
			return res, fmt.Errorf("read icon %s: %w", icon.rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return res, fmt.Errorf("create icon dir for %s: %w", icon.rel, err)
		}
		if err := os.WriteFile(dest, icon.content, 0o644); err != nil {
			return res, fmt.Errorf("write icon %s: %w", icon.rel, err)
		}
		res.Written = append(res.Written, icon.rel)
	}

	// Remove orphans recorded in the previous manifest that the user did not
	// modify since (e.g. assets dropped from the binary).
	if managed {
		for rel, oldHash := range old.Files {
			if _, ok := newFiles[rel]; ok {
				continue
			}
			dest := filepath.Join(iconsDir, filepath.FromSlash(rel))
			disk, err := os.ReadFile(dest)
			if err != nil || hashBytes(disk) != oldHash {
				continue // gone already, or user-modified: keep
			}
			if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
				return res, fmt.Errorf("remove orphaned icon %s: %w", rel, err)
			}
			res.Deleted = append(res.Deleted, rel)
			// Drop directories left empty by the removal.
			for dir := filepath.Dir(dest); dir != iconsDir; dir = filepath.Dir(dir) {
				if err := os.Remove(dir); err != nil {
					break // not empty (or error): stop climbing
				}
			}
		}
	}

	res.Changed = len(res.Written) > 0 || len(res.Deleted) > 0
	manifest := assetManifest{Fingerprint: AssetFingerprint(), Files: newFiles}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return res, fmt.Errorf("marshal asset manifest: %w", err)
	}
	if err := util.AtomicWriteFile(assetManifestPath(staticDir), data); err != nil {
		return res, fmt.Errorf("write asset manifest: %w", err)
	}
	return res, nil
}

func assetManifestPath(staticDir string) string {
	return filepath.Join(staticDir, ".builtin-assets.json")
}

func readAssetManifest(staticDir string) (assetManifest, error) {
	var m assetManifest
	data, err := os.ReadFile(assetManifestPath(staticDir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse asset manifest: %w", err)
	}
	return m, nil
}
