// Package assets embeds TypeScript runner files.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the TypeScript package.
//
//go:embed profiles templates
var Files embed.FS
