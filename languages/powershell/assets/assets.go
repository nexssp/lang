// Package assets embeds PowerShell runner files.
package assets

import "embed"

// Files contains profiles/ and templates/ owned by the PowerShell package.
//
//go:embed profiles templates
var Files embed.FS
