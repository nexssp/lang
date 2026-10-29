package extract

import "bytes"

// CleanSource applies comment stripping, indent trimming, and blank line collapsing
// in a single pass without intermediate string allocations.
//
//nolint:gocyclo // byte-level state machine; splitting on transitions adds per-byte dispatch.
func CleanSource(src []byte, opts Options, out *bytes.Buffer) {
	out.Reset()
	out.Grow(len(src))

	atLineStart := true
	consecutiveNL := 0
	inBlockComment := false
	inLineComment := false
	inString := false
	stringChar := byte(0)
	escape := false
	lineHadContent := false

	n := len(src)
	for i := 0; i < n; i++ {
		b := src[i]

		// Handle comments
		if inLineComment {
			if b == '\n' {
				inLineComment = false
				atLineStart = true
				// Only emit newline if the line had actual code content before the comment.
				// Standalone whole-line comments are stripped without leaving orphan blank lines.
				if lineHadContent && out.Len() > 0 {
					consecutiveNL++
					if !opts.TrimBlankLines || consecutiveNL <= 2 {
						out.WriteByte('\n')
					}
				}
				lineHadContent = false
			}
			continue
		}

		if inBlockComment {
			if b == '*' && i+1 < n && src[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		// Handle string literals
		if inString {
			out.WriteByte(b)
			switch {
			case b == stringChar && !escape:
				inString = false
			case b == '\\' && stringChar != '`':
				escape = !escape
			default:
				escape = false
			}
			atLineStart = false
			lineHadContent = true
			continue
		}

		// Check string literal entrance
		if b == '"' || b == '`' || b == '\'' {
			inString = true
			stringChar = b
			escape = false
			atLineStart = false
			lineHadContent = true
			out.WriteByte(b)
			continue
		}

		// Check comment entrance
		if opts.StripComments && b == '/' && i+1 < n {
			if src[i+1] == '/' {
				inLineComment = true
				i++
				continue
			}
			if src[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		// Indent trimming at line start
		if opts.TrimIndent && atLineStart && (b == ' ' || b == '\t') {
			continue
		}

		// Blank line handling
		if b == '\r' {
			continue
		}

		if b == '\n' {
			atLineStart = true
			lineHadContent = false
			consecutiveNL++
			// Never emit leading blank lines at the start of the buffer
			if out.Len() == 0 {
				continue
			}
			// Collapse excess blank lines: never allow more than 1 empty line (2 consecutive newlines)
			if (opts.TrimBlankLines || opts.StripComments) && consecutiveNL > 2 {
				continue
			}
			out.WriteByte('\n')
			continue
		}

		consecutiveNL = 0
		atLineStart = false
		if b != ' ' && b != '\t' {
			lineHadContent = true
		}
		out.WriteByte(b)
	}
}
