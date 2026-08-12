package ir

// KindDelta groups added/removed entry display strings for one kind.
type KindDelta struct {
	Kind    Kind     `json:"kind"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// EntryDiff is a structured entry-level diff between two entry sets. One diff
// exists per rule change (the pipeline is client-agnostic before rendering).
type EntryDiff struct {
	AddedCount   int         `json:"addedCount"`
	RemovedCount int         `json:"removedCount"`
	Groups       []KindDelta `json:"groups,omitempty"`
}

// Empty reports whether the diff carries no changes.
func (d EntryDiff) Empty() bool { return d.AddedCount == 0 && d.RemovedCount == 0 }

// Diff computes the entry-level delta from old to new. Entries are matched by
// canonical key; ordering changes alone produce an empty diff.
func Diff(oldEntries, newEntries []Entry) EntryDiff {
	oldKeys := make(map[string]bool, len(oldEntries))
	for _, e := range oldEntries {
		oldKeys[e.Key()] = true
	}
	newKeys := make(map[string]bool, len(newEntries))
	for _, e := range newEntries {
		newKeys[e.Key()] = true
	}

	byKind := map[Kind]*KindDelta{}
	order := []Kind{}
	group := func(k Kind) *KindDelta {
		if g, ok := byKind[k]; ok {
			return g
		}
		g := &KindDelta{Kind: k}
		byKind[k] = g
		order = append(order, k)
		return g
	}

	var diff EntryDiff
	seen := map[string]bool{}
	for _, e := range newEntries {
		k := e.Key()
		if seen[k] {
			continue
		}
		seen[k] = true
		if !oldKeys[k] {
			g := group(e.Kind)
			g.Added = append(g.Added, e.Display())
			diff.AddedCount++
		}
	}
	seen = map[string]bool{}
	for _, e := range oldEntries {
		k := e.Key()
		if seen[k] {
			continue
		}
		seen[k] = true
		if !newKeys[k] {
			g := group(e.Kind)
			g.Removed = append(g.Removed, e.Display())
			diff.RemovedCount++
		}
	}

	for _, k := range order {
		g := byKind[k]
		if len(g.Added) > 0 || len(g.Removed) > 0 {
			diff.Groups = append(diff.Groups, *g)
		}
	}
	return diff
}
