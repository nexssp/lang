// Package svelte extracts signatures and component tags from Svelte files.
package svelte

// parseSvelte extracts component script declarations and custom component tags.
func parseSvelte(src []byte, exportedOnly bool) (snippets []string, funcCount int) {
	return parseSingleFileComponent(src, exportedOnly)
}

// ExtractSignatures extracts declarations and custom component tags.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	snippets, count := parseSvelte(src, exportedOnly)
	return snippets, count, nil
}
