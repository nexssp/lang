package python

import (
	"embed"
	"io/fs"
)

//go:embed nflows
var nflowAssets embed.FS

// NFlowFiles exposes Python-specific examples rooted at "nflows/".
func NFlowFiles() fs.FS { return nflowAssets }
