package syncengine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// UploadResult mirrors the storage adapter return shape.
type UploadResult struct {
	URL      string
	Path     string
	FilePath string
}

// resolveExt normalises and falls back to the default extension. Callers
// pass the client-resolved ext but defensively accept blank/dotted input
// without rejecting them — the validation already happens at the API edge.
func resolveExt(ext string) string {
	e := schema.NormalizeOutputExt(ext)
	if e == "" {
		return schema.DefaultOutputExt
	}
	return e
}

// UploadRuleContent writes a non-geosite rule's content to disk.
func UploadRuleContent(rulesDir, ruleName, client, ext, content string) (UploadResult, error) {
	result, err := RuleArtifactPath(rulesDir, ruleName, client, ext)
	if err != nil {
		return UploadResult{}, err
	}
	dir := filepath.Dir(result.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return UploadResult{}, err
	}
	if err := util.AtomicWriteFile(result.FilePath, []byte(content)); err != nil {
		return UploadResult{}, err
	}
	return result, nil
}

// RuleArtifactPath returns the non-geosite artifact path without writing files.
func RuleArtifactPath(rulesDir, ruleName, client, ext string) (UploadResult, error) {
	if err := util.EnsureSafeSegment(ruleName, "rule name"); err != nil {
		return UploadResult{}, err
	}
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		return UploadResult{}, err
	}
	fileName := ruleName + "." + resolveExt(ext)
	full := filepath.Join(rulesDir, client, fileName)
	return UploadResult{
		URL:      fmt.Sprintf("/Rules/%s/%s", client, fileName),
		Path:     fmt.Sprintf("/Rules/%s/%s", client, fileName),
		FilePath: full,
	}, nil
}

// UploadGeositeRuleContent writes a geosite rule output under .../geosite/<provider>/<file>.
func UploadGeositeRuleContent(rulesDir, client, provider, outputName, ext, content string) (UploadResult, error) {
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		return UploadResult{}, err
	}
	if err := util.EnsureSafeSegment(provider, "geosite provider"); err != nil {
		return UploadResult{}, err
	}
	if err := util.EnsureSafeSegment(outputName, "geosite output name"); err != nil {
		return UploadResult{}, err
	}
	dir := filepath.Join(rulesDir, client, "geosite", provider)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return UploadResult{}, err
	}
	fileName := outputName + "." + resolveExt(ext)
	full := filepath.Join(dir, fileName)
	if err := util.AtomicWriteFile(full, []byte(content)); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{
		URL:      fmt.Sprintf("/Rules/%s/geosite/%s/%s", client, provider, fileName),
		Path:     fmt.Sprintf("/Rules/%s/geosite/%s/%s", client, provider, fileName),
		FilePath: full,
	}, nil
}

// ReadRuleContent loads the on-disk content for a non-geosite rule, returning
// nil if missing.
func ReadRuleContent(rulesDir, ruleName, client, ext string) (string, error) {
	if err := util.EnsureSafeSegment(ruleName, "rule name"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(rulesDir, client, ruleName+"."+resolveExt(ext)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ReadGeositeRuleContent loads geosite rule output content.
func ReadGeositeRuleContent(rulesDir, client, provider, outputName, ext string) (string, error) {
	if err := util.EnsureSafeSegment(client, "client"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(provider, "geosite provider"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(outputName, "geosite output name"); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(rulesDir, client, "geosite", provider, outputName+"."+resolveExt(ext)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// UploadForRule dispatches the upload path based on whether `rule` is geosite.
func UploadForRule(rulesDir string, rule *schema.RuleConfig, client, ext, content string) (UploadResult, error) {
	if schema.IsGeositeRule(rule) {
		src := schema.PrimaryGeositeSource(rule)
		return UploadGeositeRuleContent(rulesDir, client, src.Provider, schema.GeositeOutputName(src), ext, content)
	}
	return UploadRuleContent(rulesDir, rule.Name, client, ext, content)
}

// ReadForRule mirrors UploadForRule for reads.
func ReadForRule(rulesDir string, rule *schema.RuleConfig, client, ext string) (string, error) {
	if schema.IsGeositeRule(rule) {
		src := schema.PrimaryGeositeSource(rule)
		return ReadGeositeRuleContent(rulesDir, client, src.Provider, schema.GeositeOutputName(src), ext)
	}
	return ReadRuleContent(rulesDir, rule.Name, client, ext)
}

// RemoveArtifactFile deletes the on-disk artifact file. Errors are ignored when
// the file is already gone.
func RemoveArtifactFile(rulesDir string, rule *schema.RuleConfig, client, ext string) error {
	full, err := artifactFilePath(rulesDir, rule, client, ext)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func artifactFilePath(rulesDir string, rule *schema.RuleConfig, client, ext string) (string, error) {
	resolved := resolveExt(ext)
	if schema.IsGeositeRule(rule) {
		src := schema.PrimaryGeositeSource(rule)
		if src == nil {
			return "", fmt.Errorf("invalid geosite rule")
		}
		if err := schema.ValidateGeositeProvider(src.Provider); err != nil {
			return "", err
		}
		if err := util.EnsureSafeSegment(src.List, "geosite list"); err != nil {
			return "", err
		}
		if err := util.EnsureSafeSegment(schema.GeositeOutputName(src), "geosite output name"); err != nil {
			return "", err
		}
		if err := store.ValidateClientID(client); err != nil {
			return "", err
		}
		return filepath.Join(rulesDir, client, "geosite", src.Provider, schema.GeositeOutputName(src)+"."+resolved), nil
	}
	if err := util.EnsureSafeSegment(rule.Name, "rule name"); err != nil {
		return "", err
	}
	if err := store.ValidateClientID(client); err != nil {
		return "", err
	}
	return filepath.Join(rulesDir, client, rule.Name+"."+resolved), nil
}
