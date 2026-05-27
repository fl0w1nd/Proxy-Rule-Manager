package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/diff"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

func (s *Server) registerRuleRoutes(r chi.Router) {
	r.Delete("/rules/{ruleName}", s.adminGuard(s.handleDeleteRule))
	r.Post("/rules/batch-delete", s.adminGuard(s.handleBatchDeleteRules))
	r.Post("/rules/{ruleName}/refresh", s.adminGuard(s.handleRefreshRule))
	r.Get("/rules/local-sources", s.adminGuard(s.handleListLocalSources))
	r.Put("/rules/{ruleName}/local-source", s.adminGuard(s.handleUpdateLocalSource))
	r.Put("/rules/{ruleName}", s.adminGuard(s.handleRenameRule))
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	ruleName, err := url.PathUnescape(chi.URLParam(r, "ruleName"))
	if err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	cfg, err := s.Store.GetConfig(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	idx := -1
	for i, rule := range cfg.Rules {
		if rule.Name == ruleName {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.Error(w, http.StatusNotFound, "Rule not found")
		return
	}

	var dependents []string
	for _, rule := range cfg.Rules {
		if rule.Name == ruleName {
			continue
		}
		for _, src := range rule.Sources {
			if src.SourceType() == "ref" && src.Ref == ruleName {
				dependents = append(dependents, rule.Name)
				break
			}
		}
	}
	if len(dependents) > 0 {
		s.ErrorWith(w, http.StatusBadRequest, map[string]any{
			"error":          "无法删除规则 \"" + ruleName + "\"，它被以下规则引用: " + strings.Join(dependents, ", "),
			"dependentRules": dependents,
		})
		return
	}

	rule := cfg.Rules[idx]
	trackActivity := !schema.IsGeositeRule(&rule)
	extByClient, err := s.loadClientExtMap(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Capture previous content BEFORE we touch the filesystem so the activity
	// diff is still correct even if a later cleanup step fails.
	type artifactSnapshot struct {
		client string
		prev   string
		meta   *schema.ArtifactMeta
	}
	snapshots := make([]artifactSnapshot, 0, len(rule.Output.Clients))
	for _, client := range rule.Output.Clients {
		snap := artifactSnapshot{client: client}
		if trackActivity {
			snap.prev, _ = syncengine.ReadForRule(s.Config.RulesDir, &rule, client, extByClient[client])
		}
		snap.meta, _ = s.Store.GetArtifactMeta(ctx, ruleName, client)
		snapshots = append(snapshots, snap)
	}

	// Persist the config change first. If this fails we have NOT touched
	// any on-disk artifact yet, so the rule remains fully usable.
	cfg.Rules = append(cfg.Rules[:idx], cfg.Rules[idx+1:]...)
	if _, err := s.Store.SaveConfig(ctx, cfg); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Config committed: now best-effort delete on-disk artifacts + DB meta.
	// Any failure here is reported back via `cleanupErrors` so the dashboard
	// can surface it without misrepresenting the overall delete result.
	var (
		changeRecords []store.ChangeRecordInput
		cleanupErrors []string
	)
	for _, snap := range snapshots {
		if err := syncengine.RemoveArtifactFile(s.Config.RulesDir, &rule, snap.client, extByClient[snap.client]); err != nil {
			cleanupErrors = append(cleanupErrors,
				fmt.Sprintf("remove artifact file (client=%s): %s", snap.client, err.Error()))
		}
		if err := s.Store.DeleteArtifactMeta(ctx, ruleName, snap.client); err != nil {
			cleanupErrors = append(cleanupErrors,
				fmt.Sprintf("delete artifact meta (client=%s): %s", snap.client, err.Error()))
		}
		if trackActivity && (snap.prev != "" || snap.meta != nil) {
			body := diff.CreateActivityDiff(diff.Deleted,
				transformer.NormalizeEffectiveRuleContent(transformer.StripManagedRuleHeader(snap.prev)), "", 3)
			var size *int64
			if snap.prev != "" {
				sz := int64(len(snap.prev))
				size = &sz
			}
			changeRecords = append(changeRecords, store.ChangeRecordInput{
				ID:         uuid.New().String(),
				Timestamp:  util.NowISO(),
				RuleName:   ruleName,
				Client:     snap.client,
				ChangeType: string(diff.Deleted),
				SizeBytes:  size,
				Diff:       body,
			})
		}
	}
	_ = s.Store.RecordRuleFileChanges(ctx, changeRecords)

	resp := map[string]any{
		"success":        true,
		"deletedRule":    ruleName,
		"deletedClients": rule.Output.Clients,
	}
	if len(cleanupErrors) > 0 {
		resp["cleanupErrors"] = cleanupErrors
	}
	s.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleBatchDeleteRules(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RuleNames []string `json:"ruleNames"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	seen := map[string]struct{}{}
	requested := body.RuleNames[:0]
	for _, name := range body.RuleNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		requested = append(requested, name)
	}
	if len(requested) == 0 {
		s.Error(w, http.StatusBadRequest, "ruleNames must be a non-empty array")
		return
	}
	ctx := r.Context()
	cfg, err := s.Store.GetConfig(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	ruleByName := map[string]int{}
	for i, rule := range cfg.Rules {
		ruleByName[rule.Name] = i
	}
	dependents := map[string][]string{}
	for _, rule := range cfg.Rules {
		for _, src := range rule.Sources {
			if src.SourceType() == "ref" && src.Ref != "" {
				dependents[src.Ref] = append(dependents[src.Ref], rule.Name)
			}
		}
	}
	type blocked struct {
		Name       string   `json:"name"`
		Dependents []string `json:"dependents"`
	}
	results := struct {
		Deleted  []string  `json:"deleted"`
		NotFound []string  `json:"notFound"`
		Blocked  []blocked `json:"blocked"`
	}{Deleted: []string{}, NotFound: []string{}, Blocked: []blocked{}}
	requestedSet := seen
	for _, name := range requested {
		if _, ok := ruleByName[name]; !ok {
			results.NotFound = append(results.NotFound, name)
			continue
		}
		var externalDeps []string
		for _, dep := range dependents[name] {
			if _, internal := requestedSet[dep]; internal {
				continue
			}
			externalDeps = append(externalDeps, dep)
		}
		if len(externalDeps) > 0 {
			results.Blocked = append(results.Blocked, blocked{Name: name, Dependents: externalDeps})
		}
	}
	if len(results.Blocked) > 0 {
		s.ErrorWith(w, http.StatusBadRequest, map[string]any{
			"error":    "rules cannot be deleted due to external dependencies",
			"deleted":  results.Deleted,
			"notFound": results.NotFound,
			"blocked":  results.Blocked,
		})
		return
	}

	extByClient, err := s.loadClientExtMap(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Snapshot per-(rule, client) state BEFORE any side-effects so the
	// activity diff is correct even if cleanup partially fails.
	type batchSnap struct {
		ruleName string
		rule     *schema.RuleConfig
		client   string
		prev     string
		meta     *schema.ArtifactMeta
	}
	var snapshots []batchSnap
	for _, name := range requested {
		idx, ok := ruleByName[name]
		if !ok {
			continue
		}
		rule := &cfg.Rules[idx]
		trackActivity := !schema.IsGeositeRule(rule)
		for _, client := range rule.Output.Clients {
			snap := batchSnap{ruleName: name, rule: rule, client: client}
			if trackActivity {
				snap.prev, _ = syncengine.ReadForRule(s.Config.RulesDir, rule, client, extByClient[client])
			}
			snap.meta, _ = s.Store.GetArtifactMeta(ctx, name, client)
			snapshots = append(snapshots, snap)
		}
		results.Deleted = append(results.Deleted, name)
	}

	// Persist the config change first so we never leave files orphaned with
	// a config that still references them.
	filtered := cfg.Rules[:0]
	for _, rule := range cfg.Rules {
		if _, ok := requestedSet[rule.Name]; ok {
			continue
		}
		filtered = append(filtered, rule)
	}
	cfg.Rules = filtered
	if _, err := s.Store.SaveConfig(ctx, cfg); err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Config committed: clean up artifacts on disk + in DB. Failures are
	// surfaced via cleanupErrors but do not flip the response status.
	var (
		changeRecords   []store.ChangeRecordInput
		artifactDeletes []store.ArtifactKey
		cleanupErrors   []string
	)
	for _, snap := range snapshots {
		trackActivity := !schema.IsGeositeRule(snap.rule)
		if err := syncengine.RemoveArtifactFile(s.Config.RulesDir, snap.rule, snap.client, extByClient[snap.client]); err != nil {
			cleanupErrors = append(cleanupErrors,
				fmt.Sprintf("remove artifact file (rule=%s client=%s): %s", snap.ruleName, snap.client, err.Error()))
		}
		artifactDeletes = append(artifactDeletes, store.ArtifactKey{RuleName: snap.ruleName, Client: snap.client})
		if trackActivity && (snap.prev != "" || snap.meta != nil) {
			body := diff.CreateActivityDiff(diff.Deleted,
				transformer.NormalizeEffectiveRuleContent(transformer.StripManagedRuleHeader(snap.prev)), "", 3)
			var size *int64
			if snap.prev != "" {
				sz := int64(len(snap.prev))
				size = &sz
			}
			changeRecords = append(changeRecords, store.ChangeRecordInput{
				ID:         uuid.New().String(),
				Timestamp:  util.NowISO(),
				RuleName:   snap.ruleName,
				Client:     snap.client,
				ChangeType: string(diff.Deleted),
				SizeBytes:  size,
				Diff:       body,
			})
		}
	}
	if err := s.Store.DeleteArtifactMetas(ctx, artifactDeletes); err != nil {
		cleanupErrors = append(cleanupErrors, "delete artifact metas: "+err.Error())
	}
	_ = s.Store.RecordRuleFileChanges(ctx, changeRecords)

	resp := map[string]any{
		"success":  true,
		"deleted":  results.Deleted,
		"notFound": results.NotFound,
		"blocked":  results.Blocked,
	}
	if len(cleanupErrors) > 0 {
		resp["cleanupErrors"] = cleanupErrors
	}
	s.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleRefreshRule(w http.ResponseWriter, r *http.Request) {
	ruleName, _ := url.PathUnescape(chi.URLParam(r, "ruleName"))
	result, err := s.Engine.ExecutePartialSync(r.Context(), ruleName)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, normalizeSyncResult(result))
}

type localSourceItem struct {
	SourceIndex int    `json:"sourceIndex"`
	Name        string `json:"name,omitempty"`
	ContentRef  string `json:"contentRef,omitempty"`
}

func (s *Server) handleListLocalSources(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	type entry struct {
		RuleName string            `json:"ruleName"`
		Sources  []localSourceItem `json:"sources"`
	}
	rules := []entry{}
	for _, rule := range cfg.Rules {
		var items []localSourceItem
		for i, src := range rule.Sources {
			if src.SourceType() == "local" {
				items = append(items, localSourceItem{
					SourceIndex: i,
					Name:        src.Name,
					ContentRef:  src.ContentRef,
				})
			}
		}
		if len(items) > 0 {
			rules = append(rules, entry{RuleName: rule.Name, Sources: items})
		}
	}
	s.JSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Server) handleUpdateLocalSource(w http.ResponseWriter, r *http.Request) {
	ruleName, _ := url.PathUnescape(chi.URLParam(r, "ruleName"))
	var body struct {
		SourceIndex int     `json:"sourceIndex"`
		Content     *string `json:"content"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.SourceIndex < 0 {
		s.Error(w, http.StatusBadRequest, "sourceIndex must be a non-negative integer")
		return
	}
	if body.Content == nil {
		s.Error(w, http.StatusBadRequest, "content must be a string")
		return
	}
	ctx := r.Context()
	cfg, err := s.Store.GetConfig(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *schema.RuleConfig
	for i := range cfg.Rules {
		if cfg.Rules[i].Name == ruleName {
			target = &cfg.Rules[i]
			break
		}
	}
	if target == nil {
		s.Error(w, http.StatusNotFound, "Rule not found")
		return
	}
	if body.SourceIndex >= len(target.Sources) {
		s.Error(w, http.StatusNotFound, "sourceIndex out of range")
		return
	}
	src := &target.Sources[body.SourceIndex]
	if src.SourceType() != "local" {
		s.Error(w, http.StatusNotFound, "Source is not a local source")
		return
	}
	oldRef := src.ContentRef
	ref, err := s.Store.WriteLocalSource(ctx, src.ContentRef, *body.Content)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	src.ContentRef = ref
	// Only persist the config when the ref actually changed (i.e. a new
	// ref was minted because oldRef was empty / invalid). The common path
	// — updating an existing local source — must skip SaveConfig: the
	// in-memory cfg was hydrated by GetConfig, so src.Content currently
	// holds the *previous* DB value. Re-saving would let saveConfig's
	// "src.Content != nil ⇒ externalize" branch write that stale value
	// back to local_sources, undoing the WriteLocalSource we just did.
	if ref != oldRef {
		// Drop the hydrated stale content before saving so saveConfig
		// keeps the freshly written DB row instead of overwriting it.
		src.Content = nil
		if _, err := s.Store.SaveConfig(ctx, cfg); err != nil {
			s.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	syncRes, err := s.Engine.ExecutePartialSync(ctx, ruleName)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"ruleName":    ruleName,
		"sourceIndex": body.SourceIndex,
		"contentRef":  ref,
		"sync":        syncRes,
	})
}

func (s *Server) handleRenameRule(w http.ResponseWriter, r *http.Request) {
	oldName, _ := url.PathUnescape(chi.URLParam(r, "ruleName"))
	var body struct {
		NewName string `json:"newName"`
	}
	if err := s.DecodeJSON(r, &body); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.NewName) == "" {
		s.Error(w, http.StatusBadRequest, "newName is required")
		return
	}
	ctx := r.Context()
	cfg, err := s.Store.GetConfig(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *schema.RuleConfig
	for i := range cfg.Rules {
		if cfg.Rules[i].Name == oldName {
			target = &cfg.Rules[i]
			break
		}
	}
	if target == nil {
		s.Error(w, http.StatusNotFound, "Rule not found")
		return
	}
	if schema.IsGeositeRule(target) {
		s.Error(w, http.StatusBadRequest, "Rule \""+oldName+"\" is system-managed and cannot be renamed")
		return
	}
	for _, rule := range cfg.Rules {
		if rule.Name == body.NewName {
			// TODO: switch to typed error once store exposes a sentinel.
			s.ErrorWith(w, http.StatusBadRequest, map[string]any{
				"error": "Rule name already exists",
				"code":  "VALIDATION_ERROR",
			})
			return
		}
	}
	extByClient, err := s.loadClientExtMap(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Plan all artifact renames first. We do NOT touch the filesystem until
	// every target path has been validated as free, so a single conflict
	// cannot leave us half-renamed.
	plan, planErr := planRuleArtifactRenames(s.Config.RulesDir, target, body.NewName, extByClient)
	if planErr != nil {
		s.ErrorWith(w, http.StatusConflict, map[string]any{
			"error": planErr.Error(),
			"code":  "RENAME_CONFLICT",
		})
		return
	}

	// Execute moves. On any failure, roll back already-moved files.
	moved, moveErr := executeRuleArtifactRenames(plan)
	if moveErr != nil {
		rollbackRuleArtifactRenames(moved)
		s.Error(w, http.StatusInternalServerError, moveErr.Error())
		return
	}

	target.Name = body.NewName
	for i := range cfg.Rules {
		for j := range cfg.Rules[i].Sources {
			src := &cfg.Rules[i].Sources[j]
			if src.SourceType() == "ref" && src.Ref == oldName {
				src.Ref = body.NewName
			}
		}
	}
	if _, err := s.Store.SaveConfig(ctx, cfg); err != nil {
		// DB save failed → undo the filesystem changes so the on-disk
		// artifacts still match the (unchanged) config.
		rollbackRuleArtifactRenames(moved)
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.Store.RenameRuleArtifacts(ctx, oldName, body.NewName); err != nil {
		// Artifact meta cascade failed: files + config are renamed but the
		// artifacts table still references the old name. We don't try to
		// undo files-and-config (that would be even more destabilizing);
		// instead surface the error via cleanupErrors so the operator can
		// re-run the cascade or investigate.
		s.JSON(w, http.StatusOK, map[string]any{
			"success":       true,
			"oldName":       oldName,
			"newName":       body.NewName,
			"renamedFiles":  collectRenamedURLs(moved),
			"cleanupErrors": []string{"rename artifact meta: " + err.Error()},
		})
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"oldName":      oldName,
		"newName":      body.NewName,
		"renamedFiles": collectRenamedURLs(moved),
	})
}

type ruleArtifactRename struct {
	client  string
	oldPath string
	newPath string
	url     string
	exists  bool // old path existed before the move
}

// planRuleArtifactRenames builds the per-client move plan and validates that
// every target path is free. Returns an error if any target already exists or
// any path cannot be derived.
func planRuleArtifactRenames(rulesDir string, rule *schema.RuleConfig, newName string, extByClient map[string]string) ([]ruleArtifactRename, error) {
	if schema.IsGeositeRule(rule) {
		return nil, nil
	}
	plans := make([]ruleArtifactRename, 0, len(rule.Output.Clients))
	for _, client := range rule.Output.Clients {
		ext := extByClient[client]
		oldArt, err := syncengine.RuleArtifactPath(rulesDir, rule.Name, client, ext)
		if err != nil {
			return nil, fmt.Errorf("derive old path (client=%s): %w", client, err)
		}
		newArt, err := syncengine.RuleArtifactPath(rulesDir, newName, client, ext)
		if err != nil {
			return nil, fmt.Errorf("derive new path (client=%s): %w", client, err)
		}
		p := ruleArtifactRename{
			client:  client,
			oldPath: oldArt.FilePath,
			newPath: newArt.FilePath,
			url:     newArt.URL,
		}
		if _, err := os.Stat(oldArt.FilePath); err == nil {
			p.exists = true
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat old path (client=%s): %w", client, err)
		}
		if p.exists && p.oldPath != p.newPath {
			if _, err := os.Stat(newArt.FilePath); err == nil {
				return nil, fmt.Errorf(`target artifact already exists for client %q: %s`, client, newArt.FilePath)
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat new path (client=%s): %w", client, err)
			}
		}
		plans = append(plans, p)
	}
	return plans, nil
}

func executeRuleArtifactRenames(plans []ruleArtifactRename) ([]ruleArtifactRename, error) {
	moved := make([]ruleArtifactRename, 0, len(plans))
	for _, p := range plans {
		if !p.exists || p.oldPath == p.newPath {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p.newPath), 0o755); err != nil {
			return moved, fmt.Errorf("mkdir for new path (client=%s): %w", p.client, err)
		}
		if err := os.Rename(p.oldPath, p.newPath); err != nil {
			return moved, fmt.Errorf("rename %s -> %s: %w", p.oldPath, p.newPath, err)
		}
		moved = append(moved, p)
	}
	return moved, nil
}

func rollbackRuleArtifactRenames(moved []ruleArtifactRename) {
	for i := len(moved) - 1; i >= 0; i-- {
		p := moved[i]
		if err := os.Rename(p.newPath, p.oldPath); err != nil {
			// Best-effort: log and continue so we revert as much as possible.
			fmt.Printf("[rule rename] WARNING: rollback %s -> %s failed: %v\n", p.newPath, p.oldPath, err)
		}
	}
}

func collectRenamedURLs(moved []ruleArtifactRename) []string {
	urls := make([]string, 0, len(moved))
	for _, p := range moved {
		urls = append(urls, p.url)
	}
	return urls
}
