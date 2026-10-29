// Package assets embeds the Go runner profile and source template.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the Go language package.
//
//go:embed profiles templates
var Files embed.FS
