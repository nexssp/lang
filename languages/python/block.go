// Package python contains Python-specific source inspection, signature extraction, and NFlow assets.
package python

import (
	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
)

// IsTag reports whether tag resolves to the built-in Python language.
func IsTag(tag string) bool {
	for _, alias := range core.Aliases() {
		if alias.Tag == tag {
			return alias.ID == "python"
		}
	}
	return false
}

// ReadBlockBody scans Python source directly, rather than relying on Flow's
// host lexer for comment and string boundaries. Braces in comments, ordinary
// strings, triple-quoted strings, and f-strings do not close the outer block.
func ReadBlockBody(c *compiler.Parser) (string, error) {
	startTok := c.Current()
	if startTok.Type != compiler.TokLBrace {
		return "", c.Fail(startTok.Line, "expected '{' to open block")
	}
	source := c.Src()
	bodyStart := startTok.Offset + 1
	depth := 1
	for offset := bodyStart; offset < len(source); {
		switch source[offset] {
		case '#':
			for offset < len(source) && source[offset] != '\n' {
				offset++
			}
		case '\'', '"':
			offset = skipPythonString(source, offset)
		case '{':
			depth++
			offset++
		case '}':
			depth--
			if depth == 0 {
				for c.Current().Type != compiler.TokEOF && c.Current().Offset < offset {
					c.Advance()
				}
				if c.Current().Type != compiler.TokRBrace || c.Current().Offset != offset {
					return "", c.Fail(startTok.Line, "could not align Python block closing brace")
				}
				body := source[bodyStart:offset]
				c.Advance()
				return body, nil
			}
			offset++
		default:
			offset++
		}
	}
	return "", c.Fail(startTok.Line, "unterminated block (missing '}')")
}

func skipPythonString(source string, start int) int {
	quote := source[start]
	triple := start+2 < len(source) && source[start+1] == quote && source[start+2] == quote
	if triple {
		start += 3
	} else {
		start++
	}
	for start < len(source) {
		if source[start] == '\\' {
			if start+1 < len(source) {
				start += 2
			} else {
				start++
			}
			continue
		}
		if source[start] == quote {
			if !triple {
				return start + 1
			}
			if start+2 < len(source) && source[start+1] == quote && source[start+2] == quote {
				return start + 3
			}
		}
		start++
	}
	return len(source)
}
