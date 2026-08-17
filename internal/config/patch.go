package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// PatchOp is one finite, source-preserving configuration operation.
type PatchOp struct {
	Type      string
	ID        string
	RuleID    string
	OutputID  string
	RuleIDs   []string
	OutputIDs []string
	Order     []string
	Value     *yaml.Node
}

// PatchError reports an invalid operation in a patch transaction.
type PatchError struct {
	OpIndex int
	Path    string
	Message string
}

func (e *PatchError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("ops[%d]: %s", e.OpIndex, e.Message)
	}
	return fmt.Sprintf("ops[%d].%s: %s", e.OpIndex, e.Path, e.Message)
}

// Prepare applies operations to a private YAML tree and validates the complete
// effective configuration without changing memory or disk.
func (m *Manager) Prepare(version int64, ops []PatchOp) (*Candidate, error) {
	m.mu.RLock()
	if version != m.version {
		current := m.version
		m.mu.RUnlock()
		return nil, &VersionConflictError{CurrentVersion: current}
	}
	doc := cloneYAMLNode(m.doc)
	baseRaw := append([]byte(nil), m.raw...)
	baseDigest := m.digest
	baseVersion := m.version
	path := m.path
	cfg := m.cfg.DeepCopy()
	m.mu.RUnlock()

	if path == "" {
		return nil, ErrPersistenceUnavailable
	}
	if dirty, err := fileDigestDiffers(path, baseDigest); err != nil {
		return nil, err
	} else if dirty {
		return nil, &DirtyConfigError{}
	}

	changed, err := applyPatchOperations(doc, ops)
	if err != nil {
		return nil, err
	}
	if !changed {
		return &Candidate{
			baseVersion: baseVersion, baseDigest: baseDigest, raw: baseRaw,
			doc: doc, cfg: cfg, changed: false,
		}, nil
	}
	raw, err := encodeDocument(doc)
	if err != nil {
		return nil, err
	}
	effective, parsed, err := decodeDocument(raw, m.dataDir)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		baseVersion: baseVersion, baseDigest: baseDigest, raw: raw,
		doc: parsed, cfg: effective, changed: true,
	}, nil
}

// ParsePatchValue decodes one JSON-compatible value into a YAML node.
func ParsePatchValue(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode patch value: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("patch value is empty")
	}
	value := cloneYAMLNode(doc.Content[0])
	normalizePatchStyle(value)
	return value, nil
}

func normalizePatchStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	node.Style = 0
	for _, child := range node.Content {
		normalizePatchStyle(child)
	}
}

func applyPatchOperations(doc *yaml.Node, ops []PatchOp) (bool, error) {
	changed := false
	for i, op := range ops {
		opChanged, path, err := applyPatchOperation(doc, op)
		if err != nil {
			return false, &PatchError{OpIndex: i, Path: path, Message: err.Error()}
		}
		changed = changed || opChanged
	}
	return changed, nil
}

func applyPatchOperation(doc *yaml.Node, op PatchOp) (bool, string, error) {
	switch op.Type {
	case "add_client":
		return addObject(doc, "clients", op.Value)
	case "update_client":
		return updateObject(doc, "clients", op.ID, op.Value)
	case "remove_client":
		return removeObject(doc, "clients", op.ID)
	case "add_rule":
		return addObject(doc, "rules", op.Value)
	case "update_rule":
		return updateObject(doc, "rules", op.ID, op.Value)
	case "remove_rule":
		return removeObject(doc, "rules", op.ID)
	case "add_output":
		return changeOutput(doc, op.RuleID, op.OutputID, true)
	case "remove_output":
		return changeOutput(doc, op.RuleID, op.OutputID, false)
	case "batch_add_output", "batch_remove_output":
		if len(op.RuleIDs) == 0 {
			return false, "rule_ids", fmt.Errorf("at least one rule ID is required")
		}
		if len(op.OutputIDs) == 0 {
			return false, "output_ids", fmt.Errorf("at least one output ID is required")
		}
		add := op.Type == "batch_add_output"
		changed := false
		for _, ruleID := range op.RuleIDs {
			for _, outputID := range op.OutputIDs {
				itemChanged, path, err := changeOutput(doc, ruleID, outputID, add)
				if err != nil {
					return false, path, err
				}
				changed = changed || itemChanged
			}
		}
		return changed, "", nil
	case "reorder_rules":
		return reorderObjects(doc, "rules", op.Order)
	case "update_schedule":
		return updateSetting(doc, "schedule", op.Value)
	case "update_fetch":
		return updateSetting(doc, "fetch", op.Value)
	case "update_preprocess":
		return updateSetting(doc, "preprocess", op.Value)
	case "update_history":
		return updateHistory(doc, op.Value)
	case "update_geosite":
		return updateGeosite(doc, op.Value)
	default:
		return false, "op", fmt.Errorf("unknown operation %q", op.Type)
	}
}

func addObject(doc *yaml.Node, section string, value *yaml.Node) (bool, string, error) {
	value, id, err := objectValue(value)
	if err != nil {
		return false, "value", err
	}
	sequence, err := ensureTopSequence(doc, section)
	if err != nil {
		return false, "value", err
	}
	if _, _, ok := findObject(sequence, id); ok {
		return false, "value.id", fmt.Errorf("%s %q already exists", singular(section), id)
	}
	sequence.Content = append(sequence.Content, cloneYAMLNode(value))
	return true, "", nil
}

func updateObject(doc *yaml.Node, section, id string, value *yaml.Node) (bool, string, error) {
	if id == "" {
		return false, "id", fmt.Errorf("required")
	}
	value, valueID, err := objectValue(value)
	if err != nil {
		return false, "value", err
	}
	if valueID != id {
		return false, "value.id", fmt.Errorf("must match id %q", id)
	}
	sequence, err := topSequence(doc, section)
	if err != nil {
		return false, "id", err
	}
	old, index, ok := findObject(sequence, id)
	if !ok {
		return false, "id", fmt.Errorf("%s %q does not exist", singular(section), id)
	}
	replacement := cloneYAMLNode(value)
	replacement.HeadComment = old.HeadComment
	replacement.LineComment = old.LineComment
	replacement.FootComment = old.FootComment
	sequence.Content[index] = replacement
	return true, "", nil
}

func removeObject(doc *yaml.Node, section, id string) (bool, string, error) {
	if id == "" {
		return false, "id", fmt.Errorf("required")
	}
	sequence, err := topSequence(doc, section)
	if err != nil {
		return false, "id", err
	}
	_, index, ok := findObject(sequence, id)
	if !ok {
		return false, "id", fmt.Errorf("%s %q does not exist", singular(section), id)
	}
	sequence.Content = append(sequence.Content[:index], sequence.Content[index+1:]...)
	return true, "", nil
}

func changeOutput(doc *yaml.Node, ruleID, outputID string, add bool) (bool, string, error) {
	if ruleID == "" {
		return false, "rule_id", fmt.Errorf("required")
	}
	if outputID == "" {
		return false, "output_id", fmt.Errorf("required")
	}
	rules, err := topSequence(doc, "rules")
	if err != nil {
		return false, "rule_id", err
	}
	rule, _, ok := findObject(rules, ruleID)
	if !ok {
		return false, "rule_id", fmt.Errorf("rule %q does not exist", ruleID)
	}
	outputs, _, exists := mappingValue(rule, "outputs")
	if !exists {
		if !add {
			return false, "", nil
		}
		outputs = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		setMappingValue(rule, "outputs", outputs)
	}
	if outputs.Kind != yaml.SequenceNode {
		return false, "output_id", fmt.Errorf("rule outputs must be a sequence")
	}
	for i, item := range outputs.Content {
		if item.Value != outputID {
			continue
		}
		if add {
			return false, "", nil
		}
		outputs.Content = append(outputs.Content[:i], outputs.Content[i+1:]...)
		return true, "", nil
	}
	if !add {
		return false, "", nil
	}
	outputs.Content = append(outputs.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: outputID})
	return true, "", nil
}

func reorderObjects(doc *yaml.Node, section string, order []string) (bool, string, error) {
	sequence, err := topSequence(doc, section)
	if err != nil {
		return false, "order", err
	}
	if len(order) != len(sequence.Content) {
		return false, "order", fmt.Errorf("must contain every %s ID exactly once", singular(section))
	}
	byID := make(map[string]*yaml.Node, len(sequence.Content))
	current := make([]string, 0, len(sequence.Content))
	for _, item := range sequence.Content {
		id, ok := mappingScalar(item, "id")
		if !ok {
			return false, "order", fmt.Errorf("%s entry is missing id", singular(section))
		}
		byID[id] = item
		current = append(current, id)
	}
	seen := make(map[string]bool, len(order))
	next := make([]*yaml.Node, 0, len(order))
	for _, id := range order {
		item, ok := byID[id]
		if !ok || seen[id] {
			return false, "order", fmt.Errorf("must contain every %s ID exactly once", singular(section))
		}
		seen[id] = true
		next = append(next, item)
	}
	changed := false
	for i := range current {
		changed = changed || current[i] != order[i]
	}
	if changed {
		sequence.Content = next
	}
	return changed, "", nil
}

func updateSetting(doc *yaml.Node, key string, value *yaml.Node) (bool, string, error) {
	value = normalizeNode(value)
	if value == nil || value.Kind != yaml.MappingNode {
		return false, "value", fmt.Errorf("must be an object")
	}
	root, err := rootMapping(doc)
	if err != nil {
		return false, "value", err
	}
	update := ensureMappingValue(root, "update")
	setMappingValue(update, key, cloneYAMLNode(value))
	return true, "", nil
}

func updateHistory(doc *yaml.Node, value *yaml.Node) (bool, string, error) {
	value = normalizeNode(value)
	if value == nil || value.Kind != yaml.MappingNode {
		return false, "value", fmt.Errorf("must be an object")
	}
	allowed := map[string]bool{"history_retention": true, "history_limit": true}
	seen := map[string]bool{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if !allowed[key] {
			return false, "value." + key, fmt.Errorf("unknown field")
		}
		seen[key] = true
	}
	for key := range allowed {
		if !seen[key] {
			return false, "value." + key, fmt.Errorf("required")
		}
	}
	root, err := rootMapping(doc)
	if err != nil {
		return false, "value", err
	}
	update := ensureMappingValue(root, "update")
	for key := range allowed {
		node, _, _ := mappingValue(value, key)
		setMappingValue(update, key, cloneYAMLNode(node))
	}
	return true, "", nil
}

func updateGeosite(doc *yaml.Node, value *yaml.Node) (bool, string, error) {
	value = normalizeNode(value)
	if value == nil {
		return false, "value", fmt.Errorf("required")
	}
	root, err := rootMapping(doc)
	if err != nil {
		return false, "value", err
	}
	if value.Tag == "!!null" {
		return deleteMappingValue(root, "geosite"), "", nil
	}
	if value.Kind != yaml.MappingNode {
		return false, "value", fmt.Errorf("must be an object or null")
	}
	setMappingValue(root, "geosite", cloneYAMLNode(value))
	return true, "", nil
}

func objectValue(value *yaml.Node) (*yaml.Node, string, error) {
	value = normalizeNode(value)
	if value == nil || value.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("must be an object")
	}
	id, ok := mappingScalar(value, "id")
	if !ok || id == "" {
		return nil, "", fmt.Errorf("id is required")
	}
	return value, id, nil
}

func normalizeNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func rootMapping(doc *yaml.Node) (*yaml.Node, error) {
	root := normalizeNode(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be an object")
	}
	return root, nil
}

func topSequence(doc *yaml.Node, key string) (*yaml.Node, error) {
	root, err := rootMapping(doc)
	if err != nil {
		return nil, err
	}
	value, _, ok := mappingValue(root, key)
	if !ok || value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("config %s must be a sequence", key)
	}
	return value, nil
}

func ensureTopSequence(doc *yaml.Node, key string) (*yaml.Node, error) {
	root, err := rootMapping(doc)
	if err != nil {
		return nil, err
	}
	value, _, ok := mappingValue(root, key)
	if ok {
		if value.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("config %s must be a sequence", key)
		}
		return value, nil
	}
	value = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	setMappingValue(root, key, value)
	return value, nil
}

func findObject(sequence *yaml.Node, id string) (*yaml.Node, int, bool) {
	for i, item := range sequence.Content {
		if itemID, ok := mappingScalar(item, "id"); ok && itemID == id {
			return item, i, true
		}
	}
	return nil, -1, false
}

func mappingScalar(mapping *yaml.Node, key string) (string, bool) {
	value, _, ok := mappingValue(mapping, key)
	if !ok || value.Kind != yaml.ScalarNode {
		return "", false
	}
	return value.Value, true
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, int, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, -1, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], i, true
		}
	}
	return nil, -1, false
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	if _, index, ok := mappingValue(mapping, key); ok {
		mapping.Content[index+1] = value
		return
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value,
	)
}

func deleteMappingValue(mapping *yaml.Node, key string) bool {
	_, index, ok := mappingValue(mapping, key)
	if !ok {
		return false
	}
	mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
	return true
}

func ensureMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if value, _, ok := mappingValue(mapping, key); ok && value.Kind == yaml.MappingNode {
		return value
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setMappingValue(mapping, key, value)
	return value
}

func singular(section string) string {
	if section == "clients" {
		return "client"
	}
	return "rule"
}
