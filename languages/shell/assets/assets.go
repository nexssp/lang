// Package assets embeds the POSIX shell runner profile.
package assets

import "embed"

// Files contains profiles/ owned by the POSIX shell package.
//
//go:embed profiles
var Files embed.FS
