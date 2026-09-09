package config

import (
	"fmt"
	"iter"
)

// WalkSources visits source groups and their members with YAML-relative paths.
func WalkSources(sources []SourceConfig) iter.Seq2[string, SourceConfig] {
	return func(yield func(string, SourceConfig) bool) {
		var walk func([]SourceConfig, string, string) bool
		walk = func(items []SourceConfig, prefix string, key string) bool {
			for i, source := range items {
				path := fmt.Sprintf("%s%s[%d]", prefix, key, i)
				if !yield(path, source) || !walk(source.Group, path+".", "group") {
					return false
				}
			}
			return true
		}
		walk(sources, "", "sources")
	}
}

func copySources(sources []SourceConfig) []SourceConfig {
	if sources == nil {
		return nil
	}
	out := make([]SourceConfig, len(sources))
	for i, source := range sources {
		out[i] = source
		out[i].Attrs = append([]string(nil), source.Attrs...)
		out[i].Ops = copyOps(source.Ops)
		out[i].Group = copySources(source.Group)
		if source.Preprocess != nil {
			script := *source.Preprocess
			out[i].Preprocess = &script
		}
	}
	return out
}
