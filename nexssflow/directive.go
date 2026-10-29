package nexssflow

import (
	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
)

// LangBlock is the Flow AST payload for one language keyword. It belongs in
// this adapter because compiler positions and syntax are Flow-specific.
//

type LangBlock struct {
	ID         string
	Version    string
	Source     string
	Tag        string
	SourceFile string
	Pos        compiler.Position
}

// ParseTag resolves a Flow language keyword to a canonical language block.
// Interpreter availability remains the responsibility of preflight.
func ParseTag(tag, source string, pos compiler.Position) (*LangBlock, error) {
	program, err := core.ProgramForTag(tag, source)
	if err != nil {
		return nil, compiler.SourceError(pos, "%s: %w", tag, err)
	}
	return &LangBlock{ID: program.ID, Version: program.Version, Source: program.Source, Tag: tag, Pos: pos}, nil
}

// ParseFileTag creates a block backed by a source file. The caller resolves the
// path relative to the containing .nflow source and provides file contents.
func ParseFileTag(tag, sourceFile, source string, pos compiler.Position) (*LangBlock, error) {
	block, err := ParseTag(tag, source, pos)
	if err != nil {
		return nil, err
	}
	block.Tag = tag + ".file"
	block.SourceFile = sourceFile
	return block, nil
}
