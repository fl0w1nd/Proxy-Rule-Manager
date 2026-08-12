// Package templates provides the embedded default template YAML files.
package templates

import "embed"

//go:embed *.yaml
var FS embed.FS
