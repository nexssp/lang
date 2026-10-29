package nexssflow

import (
	"context"
	"reflect"
	"strings"

	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
)

// RunnerDirective registers a data-driven runner declared in a Flow source.
var RunnerDirective = compiler.Directive{
	Name: "lang.runner",
	Example: `@lang.runner "python" {
  ext: ".py"
  aliases: ["py", "python"]
  find: ["python3", "python"]
}`,
	Handler: handleRunnerDirective,
}

// DefineDirective registers an interpreter available to the current process.
var DefineDirective = compiler.Directive{
	Name: "lang.define",
	Example: `@lang.define "gleam" {
  ext: ".gleam"
  aliases: ["gleam"]
  find: ["gleam run"]
}`,
	Handler: handleDefineDirective,
}

func handleRunnerDirective(_ context.Context, req compiler.DirectiveReq) (compiler.DirectiveRes, error) {
	pos := compiler.Position{File: req.File, Line: req.I + 1}
	raw, next, err := collectDirectiveBody(req, pos)
	if err != nil {
		return compiler.DirectiveRes{}, err
	}
	spec, err := core.ParseRunner(raw)
	if err != nil {
		return compiler.DirectiveRes{}, compiler.SourceError(pos, "@lang.runner: %w", err)
	}
	if err := validateRunnerSymbols(req, spec); err != nil {
		return compiler.DirectiveRes{}, compiler.SourceError(pos, "@lang.runner: %w", err)
	}
	if err := registerRunnerProfile(spec); err != nil {
		return compiler.DirectiveRes{}, compiler.SourceError(pos, "@lang.runner: %w", err)
	}
	return compiler.DirectiveRes{Next: next}, nil
}

func handleDefineDirective(_ context.Context, req compiler.DirectiveReq) (compiler.DirectiveRes, error) {
	pos := compiler.Position{File: req.File, Line: req.I + 1}
	raw, next, err := collectDirectiveBody(req, pos)
	if err != nil {
		return compiler.DirectiveRes{}, err
	}
	spec, err := core.ParseRunner(raw)
	if err != nil {
		return compiler.DirectiveRes{}, compiler.SourceError(pos, "@lang.define: %w", err)
	}
	definition := core.Define{
		ID: spec.ID, Version: spec.Version, Ext: spec.Ext,
		Aliases: spec.Aliases, Find: spec.Find, Stdin: spec.Stdin,
		Stdout: spec.Stdout, FileTemplate: spec.FileTemplate,
		Install: spec.Install, Deps: spec.Deps, DepsMarker: spec.DepsMarker,
	}
	if err := core.RegisterDefine(definition); err != nil {
		return compiler.DirectiveRes{}, compiler.SourceError(pos, "@lang.define: %w", err)
	}
	return compiler.DirectiveRes{Next: next}, nil
}

func validateRunnerSymbols(req compiler.DirectiveReq, spec *core.RunnerSpec) error {
	namespaces, _ := req.Out["symbols"].(map[string]compiler.SymbolNamespace)
	namespace, ok := namespaces["runner"]
	if !ok {
		return nil
	}
	if id, exists := namespace.Symbols["id"]; exists && id != spec.ID {
		return compiler.SourceError(compiler.Position{File: req.File, Line: req.I + 1},
			"@symbols runner.id %q does not match @lang.runner ID %q", id, spec.ID)
	}
	if version, exists := namespace.Symbols["version"]; exists && version != spec.Version {
		return compiler.SourceError(compiler.Position{File: req.File, Line: req.I + 1},
			"@symbols runner.version %q does not match @lang.runner version %q", version, spec.Version)
	}
	return nil
}

func registerRunnerProfile(spec *core.RunnerSpec) error {
	existing, err := core.Lookup(spec.ID, spec.Version)
	if err == nil {
		if current, ok := existing.Spec(); ok && reflect.DeepEqual(current, *spec) {
			return nil
		}
	}
	return core.RegisterRunnerSpec(spec)
}

// collectDirectiveBody reconstructs a directive header and its field block,
// stopping at the matching closing brace. Braces inside quoted values are
// ignored, including escaped quotes and backslashes.
func collectDirectiveBody(req compiler.DirectiveReq, pos compiler.Position) (body string, next int, err error) {
	if req.I >= len(req.Lines) {
		return "", 0, compiler.SourceError(pos, "@lang.runner: no body")
	}
	var b strings.Builder
	depth := 0
	started := false
	inString := false
	escaped := false
	for lineIndex := req.I; lineIndex < len(req.Lines); lineIndex++ {
		line := req.Lines[lineIndex]
		b.WriteString(line)
		b.WriteByte('\n')
		for i := range len(line) {
			ch := line[i]
			if !started {
				if ch == '{' {
					started = true
					depth = 1
				}
				continue
			}
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return b.String(), lineIndex + 1, nil
				}
			}
		}
	}
	return "", 0, compiler.SourceError(pos, "@lang.runner: unterminated body (missing '}')")
}
