package config

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// Position records a YAML source location.
type Position struct {
	Line   int
	Column int
}

// PositionIndex maps config paths (e.g. "rules[0].sources[1].geosite") to
// their YAML source positions. Built from the raw yaml.Node tree so that
// validation errors can reference exact line numbers.
type PositionIndex struct {
	entries map[string]Position
}

// Lookup returns the position for a config path, or a zero Position if unknown.
func (pi *PositionIndex) Lookup(path string) Position {
	if pi == nil {
		return Position{}
	}
	return pi.entries[path]
}

// Has reports whether a config path was explicitly present in the YAML file.
func (pi *PositionIndex) Has(path string) bool {
	if pi == nil {
		return false
	}
	_, ok := pi.entries[path]
	return ok
}

// BuildPositionIndex walks a yaml.Node tree and records positions for every
// addressable config path.
func BuildPositionIndex(doc *yaml.Node) *PositionIndex {
	pi := &PositionIndex{entries: make(map[string]Position)}
	if doc == nil {
		return pi
	}
	root := doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		pi.walkMapping(root, "")
	}
	return pi
}

func (pi *PositionIndex) walkMapping(node *yaml.Node, prefix string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		path := prefix + keyNode.Value
		pi.entries[path] = Position{Line: valNode.Line, Column: valNode.Column}

		switch valNode.Kind {
		case yaml.MappingNode:
			pi.walkMapping(valNode, path+".")
		case yaml.SequenceNode:
			pi.walkSequence(valNode, path)
		}
	}
}

func (pi *PositionIndex) walkSequence(node *yaml.Node, prefix string) {
	for i, item := range node.Content {
		path := prefix + "[" + strconv.Itoa(i) + "]"
		pi.entries[path] = Position{Line: item.Line, Column: item.Column}

		switch item.Kind {
		case yaml.MappingNode:
			pi.walkMapping(item, path+".")
		case yaml.SequenceNode:
			pi.walkSequence(item, path)
		}
	}
}
