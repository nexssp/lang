// Package lang provides reusable language runner discovery, registration,
// installation, and byte-oriented execution primitives. Flow-specific syntax
// and stream conversion live in the nexssflow subpackage.
package core

// Program describes source to execute with a registered language runner.
// It contains no parser- or compiler-specific data.
type Program struct {
	ID         string
	Version    string
	Source     string
	Tag        string
	SourceFile string
}

// UnknownTagError reports an alias which is not registered by a runner.
type UnknownTagError struct{ Tag string }

func (e *UnknownTagError) Error() string { return "unknown language keyword " + e.Tag }

// ProgramForTag resolves a runner alias and constructs a generic program.
// Interpreter discovery is intentionally deferred to Resolve or Execute.
func ProgramForTag(tag, source string) (Program, error) {
	id, version, ok := ResolveAlias(tag)
	if !ok {
		return Program{}, &UnknownTagError{Tag: tag}
	}
	if _, err := Lookup(id, version); err != nil {
		return Program{}, err
	}
	return Program{ID: id, Version: version, Source: source, Tag: tag}, nil
}

// FileProgramForTag resolves a runner alias and constructs a program backed by
// an existing source file. The caller resolves the path relative to its own
// source file and supplies contents for preflight/dependency-marker inspection.
func FileProgramForTag(tag, sourceFile, source string) (Program, error) {
	program, err := ProgramForTag(tag, source)
	if err != nil {
		return Program{}, err
	}
	program.SourceFile = sourceFile
	return program, nil
}
