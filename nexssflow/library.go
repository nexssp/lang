// Package nexssflow adapts github.com/nexssp/lang to Nexss Flow's compiler,
// AST, bundle registry, subprocess action, and @require mechanism.
package nexssflow

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/nexssp/flow/compiler"
	"github.com/nexssp/kernel/action"

	core "github.com/nexssp/lang/core"
	pythonlang "github.com/nexssp/lang/languages/python"
)

// ID is the stable @require bundle name.
const ID = "lang"

//go:embed nflows
var fixturesFS embed.FS

func init() { compiler.Register(ID, Bundle) }

// Bundle wires language directives, keyword primaries, preflight validation,
// runner source modules, and existing conformance fixtures into Flow.
func Bundle(_ map[string]string) compiler.Bundle {
	return compiler.Bundle{
		ID:              ID,
		Libraries:       []action.Library{{Name: ID}},
		Directives:      []compiler.Directive{RunnerDirective, DefineDirective},
		Fixtures:        bundleFixtureFiles(),
		SourceModules:   core.RunnerFiles(),
		ExportedModules: runnerExports(),
		OnPreprocess: func(_ map[string]any) compiler.PreprocessContributions {
			return compiler.PreprocessContributions{
				Primaries: keywordPrimaries(),
				CompileOpts: []compiler.CompileOption{
					compiler.WithASTContextValidators(preflightValidator),
				},
			}
		},
	}
}

func bundleFixtureFiles() fs.FS {
	files := fstest.MapFS{}
	for _, source := range []fs.FS{fixturesFS, pythonlang.NFlowFiles()} {
		if err := fs.WalkDir(source, "nflows", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(source, name)
			if err != nil {
				return err
			}
			files[name] = &fstest.MapFile{Data: data}
			return nil
		}); err != nil {
			panic("lang: map embedded conformance fixtures: " + err.Error())
		}
	}
	return files
}

func runnerExports() []string {
	entries, err := fs.ReadDir(core.RunnerFiles(), "runners")
	if err != nil {
		panic("lang: read embedded runner profiles: " + err.Error())
	}
	modules := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".nflow") {
			modules = append(modules, path.Join("runners", entry.Name()))
		}
	}
	sort.Strings(modules)
	return modules
}
