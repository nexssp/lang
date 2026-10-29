// Package assets embeds the WSL runner profile.
package assets

import "embed"

// Files contains profiles/ owned by the WSL package.
//
//go:embed profiles
var Files embed.FS
