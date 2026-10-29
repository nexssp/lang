// Package vue extracts signatures and component tags from Vue files.
package vue

// parseVue extracts component script declarations and custom component tags.
func parseVue(src []byte, exportedOnly bool) (snippets []string, funcCount int) {
	return parseSingleFileComponent(src, exportedOnly)
}

// ExtractSignatures extracts declarations and custom component tags.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	snippets, count := parseVue(src, exportedOnly)
	return snippets, count, nil
}
