// Package typescript owns the TypeScript runner profile, starter template, and source-signature extractor.
package typescript

import (
	"io/fs"

	"github.com/nexssp/lang/languages/typescript/assets"
)

// AssetFiles returns this language's embedded profile and template files,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
