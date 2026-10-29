// Package assets embeds Python runner profiles and source templates.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the Python package.
//
//go:embed profiles templates
var Files embed.FS
