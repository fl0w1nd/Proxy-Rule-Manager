package config

import (
	"fmt"
	"strings"
)

// InvalidDocumentError reports malformed YAML or fields outside the config
// schema before semantic validation can run.
type InvalidDocumentError struct {
	Err error
}

func (e *InvalidDocumentError) Error() string { return e.Err.Error() }
func (e *InvalidDocumentError) Unwrap() error { return e.Err }

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
