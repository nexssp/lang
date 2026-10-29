// Package powershell owns the PowerShell runner profile and starter template.
package powershell

import (
	"io/fs"

	"github.com/nexssp/lang/languages/powershell/assets"
)

// AssetFiles returns this language's embedded profile and template files,
// rooted at profiles/ and templates/.
func AssetFiles() fs.FS { return assets.Files }
