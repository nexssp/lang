package python

import (
	"io/fs"

	"github.com/nexssp/lang/languages/python/assets"
)

// AssetFiles returns Python's embedded runner profiles and source templates,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
