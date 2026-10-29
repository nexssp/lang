// Package assets embeds JavaScript runner files.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the JavaScript package.
//
//go:embed profiles templates
var Files embed.FS
