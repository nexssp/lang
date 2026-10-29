package extract

import (
	"strings"
	"testing"
)

func TestParseDispatchesSrcpackLanguages(t *testing.T) {
	cases := []struct {
		name string
		path string
		lang string
		src  string
		want string
	}{
		{"Go", "service.go", "go", "package service\nfunc Public() int { return 1 }", "Public"},
		{"PHP", "service.php", "php", "<?php function publicCall(): int { return 1; }", "publicCall"},
		{"Python", "service.py", "python", "def public_call(value):\n    return value\n", "public_call"},
		{"Rust", "service.rs", "rust", "pub fn public_call() -> i32 { 1 }", "public_call"},
		{"JavaScript", "service.js", "javascript", "export function publicCall() { return 1; }", "publicCall"},
		{"TypeScript", "service.ts", "typescript", "export function publicCall(): number { return 1; }", "publicCall"},
		{"JSX", "component.jsx", "jsx", "export function Public() { return <div />; }", "Public"},
		{"TSX", "component.tsx", "tsx", "export function Public() { return <div />; }", "Public"},
		{"Svelte", "component.svelte", "svelte", "<script>export function publicCall() { return 1; }</script><Button on:click={publicCall} />", "publicCall"},
		{"Vue", "component.vue", "vue", "<script lang=\"ts\">export function publicCall(): number { return 1; }</script><Button @click=\"publicCall\" />", "publicCall"},
		{"Zig", "service.zig", "zig", "pub fn publicCall() i32 { return 1; }", "publicCall"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(tc.path, []byte(tc.src), tc.lang, Options{ExportedOnly: true})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			joined := strings.Join(result.Snippets, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("snippets %q do not contain %q", joined, tc.want)
			}
			if result.FuncCount == 0 {
				t.Fatalf("FuncCount = 0 for extracted function %q", tc.want)
			}
			if result.Content != "" {
				t.Fatalf("Content = %q; want empty when snippets were extracted", result.Content)
			}
		})
	}
}

func TestParsePassthroughAndUnsupportedLanguage(t *testing.T) {
	source := "@pipeline\nvalue"
	for _, language := range []string{"nflow", "flow", "unknown"} {
		result, err := Parse("file."+language, []byte(source), language, Options{})
		if err != nil {
			t.Fatalf("Parse(%q): %v", language, err)
		}
		if result.Content != source {
			t.Errorf("Parse(%q).Content = %q; want %q", language, result.Content, source)
		}
	}
}
