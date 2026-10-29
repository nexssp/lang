package svelte

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nexssp/lang/languages/javascript"
)

var (
	sfcScriptRegex    = regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`)
	sfcComponentRegex = regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*)([^>]*)>`)
	sfcAttrRegex      = regexp.MustCompile(`\b([a-zA-Z0-9_-]+)(?:=|\s|/|$)`)
)

// parseSingleFileComponent extracts <script> declarations (via the
// TypeScript/JS parser) and template component tags. Vue and Svelte
// single-file components share this structure exactly.
func parseSingleFileComponent(src []byte, exportedOnly bool) (snippets []string, funcCount int) {
	content := string(src)

	for _, m := range sfcScriptRegex.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		attrPart := content[m[2]:m[3]]
		scriptBody := content[m[4]:m[5]]

		ext := "file.js"
		if sfcHasTSLang(attrPart) {
			ext = "file.ts"
		}

		tsSnippets, count, parseErr := javascript.ExtractSignatures([]byte(scriptBody), ext, exportedOnly)
		if parseErr != nil {
			// Inner <script lang="ts"> failed to parse; skip this block
			// but keep the component tags. Failing the whole component
			// because one embedded script is malformed is worse than
			// losing that script's signatures.
			continue
		}
		snippets = append(snippets, tsSnippets...)
		funcCount += count
	}

	templateOnly := sfcScriptRegex.ReplaceAllString(content, "")
	if tags := extractComponentTags(templateOnly); len(tags) > 0 {
		snippets = append(snippets, "// --- UI COMPONENTS ---")
		snippets = append(snippets, tags...)
	}

	return snippets, funcCount
}

func sfcHasTSLang(attrPart string) bool {
	for _, needle := range []string{
		`lang="ts"`, `lang='ts'`,
		`lang="typescript"`, `lang='typescript'`,
	} {
		if strings.Contains(attrPart, needle) {
			return true
		}
	}
	return false
}

func extractComponentTags(template string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, match := range sfcComponentRegex.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		attrStr := ""
		if len(match) >= 3 {
			attrStr = match[2]
		}
		tag := buildComponentTag(match[1], attrStr)
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}

func buildComponentTag(name, attrStr string) string {
	attrStr = strings.TrimSuffix(strings.TrimSpace(attrStr), "/")
	matches := sfcAttrRegex.FindAllStringSubmatch(attrStr, -1)
	names := make([]string, 0, len(matches))
	for _, am := range matches {
		if len(am) >= 2 && am[1] != "" && am[1] != "/" {
			names = append(names, strings.TrimSpace(am[1]))
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("<%s />", name)
	}
	return fmt.Sprintf("<%s %s />", name, strings.Join(names, " "))
}
