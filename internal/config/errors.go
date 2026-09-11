package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigError is a single validation issue with an optional source position.
type ConfigError struct {
	Path    string // config path, e.g. "rules[0].sources[1].geosite"
	Line    int    // YAML line number (1-based), 0 if unknown
	Message string
}

func (e ConfigError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s: %s", e.Line, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ConfigErrors aggregates multiple ConfigError values into one error.
type ConfigErrors []ConfigError

func (es ConfigErrors) Error() string {
	msgs := make([]string, len(es))
	for i, e := range es {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "\n")
}

// ErrorAt creates a ConfigError for the given path, automatically resolving
// the YAML line number from the config's position index.
func (c *Config) ErrorAt(path, message string) ConfigError {
	p := Position{}
	if c.positions != nil {
		p = c.positions.Lookup(path)
	}
	return ConfigError{Path: path, Line: p.Line, Message: message}
}

var yamlErrorLine = regexp.MustCompile(`line ([0-9]+):`)

// documentError converts YAML decoder diagnostics into editor positions.
func documentError(err error, doc *yaml.Node) error {
	messages := []string{err.Error()}
	var typeErr *yaml.TypeError
	if errors.As(err, &typeErr) {
		messages = typeErr.Errors
	}
	issues := make(ConfigErrors, 0, len(messages))
	for _, message := range messages {
		line := 1
		if match := yamlErrorLine.FindStringSubmatch(message); len(match) > 1 {
			line, _ = strconv.Atoi(match[1])
		}
		path := "config"
		// Prefer the deepest field on the reported line, including unknown keys.
		var walk func(*yaml.Node, string)
		walk = func(node *yaml.Node, prefix string) {
			if node == nil {
				return
			}
			switch node.Kind {
			case yaml.DocumentNode:
				for _, child := range node.Content {
					walk(child, prefix)
				}
			case yaml.MappingNode:
				for i := 0; i+1 < len(node.Content); i += 2 {
					key, value := node.Content[i], node.Content[i+1]
					field := key.Value
					if prefix != "" {
						field = prefix + "." + field
					}
					if key.Line == line || value.Line == line {
						path = field
					}
					walk(value, field)
				}
			case yaml.SequenceNode:
				for i, child := range node.Content {
					walk(child, fmt.Sprintf("%s[%d]", prefix, i))
				}
			}
		}
		walk(doc, "")
		issues = append(issues, ConfigError{Path: path, Line: line, Message: message})
	}
	return issues
}
