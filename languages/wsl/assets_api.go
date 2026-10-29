// Package wsl owns the WSL Bash runner profile.
package wsl

import (
	"io/fs"

	"github.com/nexssp/lang/languages/wsl/assets"
)

// AssetFiles returns this language's embedded files, rooted at profiles/.
func AssetFiles() fs.FS { return assets.Files }
