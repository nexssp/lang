// Package bash owns the Bash runner profile.
package bash

import (
	"io/fs"

	"github.com/nexssp/lang/languages/bash/assets"
)

// AssetFiles returns this language's embedded files, rooted at profiles/.
func AssetFiles() fs.FS { return assets.Files }
