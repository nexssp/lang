// Package javascript owns the JavaScript runner profile, starter template, and source-signature extractor.
package javascript

import (
	"io/fs"

	"github.com/nexssp/lang/languages/javascript/assets"
)

// AssetFiles returns this language's embedded profile and template files,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
