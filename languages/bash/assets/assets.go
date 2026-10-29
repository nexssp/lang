// Package assets embeds the Bash runner profile.
package assets

import "embed"

// Files contains profiles/ owned by the Bash package.
//
//go:embed profiles
var Files embed.FS
