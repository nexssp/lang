// Package rust owns the Rust runner profile, starter template, and source-signature extractor.
package rust

import (
	"io/fs"

	"github.com/nexssp/lang/languages/rust/assets"
)

// AssetFiles returns this language's embedded profile and template files,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
