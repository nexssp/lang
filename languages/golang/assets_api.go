// Package golang owns the Go runner profile, starter template, and source-signature extractor.
package golang

import (
	"io/fs"

	"github.com/nexssp/lang/languages/golang/assets"
)

// AssetFiles returns this language's embedded profile and template files,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
