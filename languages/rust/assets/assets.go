// Package assets embeds Rust runner files.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the Rust package.
//
//go:embed profiles templates
var Files embed.FS
