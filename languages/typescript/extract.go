package typescript

import "github.com/nexssp/lang/languages/javascript"

// ExtractSignatures parses TypeScript, TSX, or JavaScript-compatible source.
// filename selects TSX/JSX grammar when its extension requires it.
func ExtractSignatures(src []byte, filename string, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return javascript.ExtractSignatures(src, filename, exportedOnly)
}
