package extract

import (
	"bytes"
	"strings"
	"sync"

	golanglang "github.com/nexssp/lang/languages/golang"
	javascriptlang "github.com/nexssp/lang/languages/javascript"
	phplang "github.com/nexssp/lang/languages/php"
	pythonlang "github.com/nexssp/lang/languages/python"
	rustlang "github.com/nexssp/lang/languages/rust"
	sveltelang "github.com/nexssp/lang/languages/svelte"
	typescriptlang "github.com/nexssp/lang/languages/typescript"
	vuelang "github.com/nexssp/lang/languages/vue"
	ziglang "github.com/nexssp/lang/languages/zig"
)

// Options controls signature filtering and content cleanup for Parse.
type Options struct {
	ExportedOnly   bool
	StripComments  bool
	TrimIndent     bool
	TrimBlankLines bool
	MaxLines       int
	MaxLinesHead   int
	MaxLinesTail   int
	RemoveBodyFunc bool
}

// Result contains extracted declaration snippets, the function count, or
// processed full content when no signature snippets are available.
type Result struct {
	Snippets  []string
	FuncCount int
	Content   string
}

var cleanBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 16<<10))
	},
}

func isWholeDocumentLang(lang string) bool {
	return lang == "nflow" || lang == "flow"
}

// Parse dispatches source to the language-specific extractor selected by lang.
// The result shape is stable for callers that package or summarize source files.
func Parse(path string, src []byte, lang string, opts Options) (Result, error) {
	if isWholeDocumentLang(lang) {
		return Result{Content: string(src)}, nil
	}

	snippets, funcCount, err := parseSnippets(path, src, lang, opts.ExportedOnly, opts.StripComments)
	if err != nil {
		return Result{}, err
	}

	if len(snippets) > 0 {
		return Result{
			Snippets:  cleanSnippets(snippets, opts),
			FuncCount: funcCount,
		}, nil
	}

	if opts.RemoveBodyFunc {
		return Result{Snippets: nil, FuncCount: 0, Content: ""}, nil
	}

	lineCount := bytes.Count(src, []byte{'\n'}) + 1
	if opts.MaxLines > 0 && lineCount > opts.MaxLines {
		return Result{
			Content:   Truncate(src, opts, snippets),
			FuncCount: funcCount,
		}, nil
	}

	return Result{
		Content:   ProcessContent(string(src), opts),
		FuncCount: funcCount,
	}, nil
}

// parseSnippets dispatches to the per-language extractor. Languages
// with no dedicated extractor fall through to the raw-content path in
// the caller.
func parseSnippets(path string, src []byte, lang string, exportedOnly, stripComments bool) (snippets []string, funcCount int, err error) {
	switch lang {
	case "go":
		return golanglang.ExtractSignatures(path, src, exportedOnly, stripComments)
	case "rust":
		return rustlang.ExtractSignatures(src, exportedOnly)
	case "javascript", "jsx":
		return javascriptlang.ExtractSignatures(src, path, exportedOnly)
	case "typescript", "tsx":
		return typescriptlang.ExtractSignatures(src, path, exportedOnly)
	case "php":
		return phplang.ExtractSignatures(src, exportedOnly)
	case "python":
		return pythonlang.ExtractSignatures(src, exportedOnly)
	case "svelte":
		return sveltelang.ExtractSignatures(src, exportedOnly)
	case "vue":
		return vuelang.ExtractSignatures(src, exportedOnly)
	case "zig":
		return ziglang.ExtractSignatures(src, exportedOnly)
	default:
		return nil, 0, nil
	}
}

// cleanSnippets trims each snippet and applies CleanSource when the
// options require it. Empty results are dropped so the caller's
// "has snippets?" check reflects reality.
func cleanSnippets(snippets []string, opts Options) []string {
	needClean := opts.StripComments || opts.TrimBlankLines || opts.TrimIndent
	out := make([]string, 0, len(snippets))

	if !needClean {
		for _, s := range snippets {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}

	buf, ok := cleanBufferPool.Get().(*bytes.Buffer)
	if !ok {
		buf = bytes.NewBuffer(make([]byte, 0, 16<<10))
	}
	defer cleanBufferPool.Put(buf)

	for _, s := range snippets {
		CleanSource([]byte(s), opts, buf)
		if cleaned := strings.TrimSpace(buf.String()); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

// ProcessContent applies the requested comment, indentation, and blank-line cleanup.
func ProcessContent(s string, opts Options) string {
	buf, ok := cleanBufferPool.Get().(*bytes.Buffer)
	if !ok {
		buf = bytes.NewBuffer(make([]byte, 0, 16<<10))
	}
	defer cleanBufferPool.Put(buf)

	CleanSource([]byte(s), opts, buf)
	return strings.Clone(buf.String())
}

// StripComments removes line and block comments while preserving string literals.
func StripComments(content string) string {
	return ProcessContent(content, Options{StripComments: true})
}

// TrimIndent removes leading indentation from every line.
func TrimIndent(s string) string {
	return ProcessContent(s, Options{TrimIndent: true})
}

// TrimBlankLines collapses runs of blank lines.
func TrimBlankLines(s string) string {
	return ProcessContent(s, Options{TrimBlankLines: true})
}

// Truncate keeps configured head and tail lines and marks omitted middle content.
func Truncate(src []byte, opts Options, snippets []string) string {
	var cleaned bytes.Buffer
	CleanSource(src, opts, &cleaned)

	lines := bytes.Split(cleaned.Bytes(), []byte{'\n'})
	total := len(lines)

	head := 20
	tail := 10

	if opts.MaxLines > 0 {
		switch {
		case opts.MaxLinesHead > 0 && opts.MaxLinesTail > 0:
			head = opts.MaxLinesHead
			tail = opts.MaxLinesTail
		case opts.MaxLinesHead > 0:
			head = opts.MaxLinesHead
			tail = max(0, opts.MaxLines-head)
		case opts.MaxLinesTail > 0:
			tail = opts.MaxLinesTail
			head = max(0, opts.MaxLines-tail)
		default:
			head = max(1, opts.MaxLines/2)
			tail = max(1, opts.MaxLines-head)
		}
	} else {
		if opts.MaxLinesHead > 0 {
			head = opts.MaxLinesHead
		}
		if opts.MaxLinesTail > 0 {
			tail = opts.MaxLinesTail
		}
	}

	if head > total {
		head = total
	}
	if tail > total-head {
		tail = total - head
	}

	var buf bytes.Buffer
	for i := range head {
		buf.Write(lines[i])
		buf.WriteByte('\n')
	}

	if len(snippets) > 0 {
		buf.WriteString("\n// --- SIGNATURES ---\n")
		for _, s := range snippets {
			buf.WriteString(s)
			buf.WriteByte('\n')
		}
	}

	if head < total-tail {
		buf.WriteString("\n// --- TRUNCATED MIDDLE ---\n")
	}

	for i := total - tail; i < total; i++ {
		buf.Write(lines[i])
		buf.WriteByte('\n')
	}

	return buf.String()
}
