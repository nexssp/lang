// Package shell owns the POSIX sh runner profile.
package shell

import (
	"io/fs"

	"github.com/nexssp/lang/languages/shell/assets"
)

// AssetFiles returns this language's embedded files, rooted at profiles/.
func AssetFiles() fs.FS { return assets.Files }
