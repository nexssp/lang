package nexssflow

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexssp/flow/compiler"
	"github.com/nexssp/flow/extensions/assert"
	dataext "github.com/nexssp/flow/extensions/data"
	"github.com/nexssp/flow/extensions/description"
	"github.com/nexssp/flow/extensions/pipeline"
	projectionext "github.com/nexssp/flow/extensions/projection"
	requireext "github.com/nexssp/flow/extensions/require"
	runtimeext "github.com/nexssp/flow/extensions/runtime"
	"github.com/nexssp/flow/extensions/syntax"
	"github.com/nexssp/flow/runner"
	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xerr"
)

func TestExtractSourceActionIsRegistered(t *testing.T) {
	bundle := Bundle(nil)
	if err := compiler.ValidateBundle(bundle); err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}

	found := false
	for _, library := range bundle.Libraries {
		for _, candidate := range library.Actions {
			if candidate.Describe().Name == "lang.extract" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("bundle does not expose lang.extract")
	}

	if _, err := bundle.Fixtures.Open("nflows/source_extract.nflow"); err != nil {
		t.Fatalf("source extraction fixture is unavailable: %v", err)
	}
}

func TestExtractSourceFlowFixture(t *testing.T) {
	const fixtureName = "nflows/source_extract.nflow"
	source, err := fs.ReadFile(Bundle(nil).Fixtures, fixtureName)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	baseBundles := []compiler.Bundle{
		assert.Bundle(nil),
		dataext.Bundle(nil),
		description.Bundle(nil),
		pipeline.Bundle(nil),
		projectionext.Bundle(nil),
		requireext.Bundle(nil),
		runtimeext.Bundle(nil),
		syntax.Bundle(nil),
	}
	var directives []compiler.Directive
	for _, bundle := range baseBundles {
		directives = append(directives, bundle.Directives...)
	}
	filename := filepath.Join(t.TempDir(), "source_extract.nflow")
	_, meta, err := compiler.Preprocess(t.Context(), compiler.NewDirectiveTable(directives...), string(source), filename)
	if err != nil {
		t.Fatalf("preprocess fixture: %v", err)
	}
	requirements, err := requireext.SourceRequirements(meta)
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	requiredBundles, err := requireext.ResolveBundles(requirements)
	if err != nil {
		t.Fatalf("resolve requirements: %v", err)
	}
	cfg, err := runner.BuildConfig(append(baseBundles, requiredBundles...))
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	if _, err := runner.Execute(t.Context(), cfg, string(source), filename, nil); err != nil {
		t.Fatalf("execute fixture: %v", err)
	}
}

func TestExtractSourceActionAcceptsFlowPayload(t *testing.T) {
	payload := map[string]any{
		"path":     "sample.go",
		"language": "go",
		"source":   "package sample\n\nfunc Exported() {}\nfunc hidden() {}\n",
		"options":  map[string]any{"exported_only": true},
	}
	result, err := action.InvokeAny(context.Background(), extractSourceAction, payload)
	if err != nil {
		t.Fatalf("invoke lang.extract: %v", err)
	}
	got, ok := result.(ExtractSourceResult)
	if !ok {
		t.Fatalf("result type = %T; want ExtractSourceResult", result)
	}
	if got.FuncCount != 1 {
		t.Fatalf("FuncCount = %d; want 1 exported function", got.FuncCount)
	}
	if len(got.Snippets) != 1 || !strings.Contains(got.Snippets[0], "Exported") {
		t.Fatalf("Snippets = %#v; want only the exported declaration", got.Snippets)
	}
	if strings.Contains(strings.Join(got.Snippets, "\n"), "hidden") {
		t.Errorf("hidden declaration unexpectedly included: %#v", got.Snippets)
	}
	if got.Content != "" {
		t.Errorf("Content = %q; want empty when snippets were extracted", got.Content)
	}
}

func TestExtractSourceActionClassifiesInvalidSource(t *testing.T) {
	_, err := extractSource(context.Background(), ExtractSourceRequest{
		Path:     "broken.go",
		Language: "go",
		Source:   "package sample\nfunc Broken( {\n",
	})
	if err == nil {
		t.Fatal("extractSource accepted malformed Go source")
	}
	if kind := xerr.KindFrom(err); kind != xerr.KindValidation {
		t.Fatalf("error kind = %q; want %q (%v)", kind, xerr.KindValidation, err)
	}
}

func TestExtractSourceActionHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := extractSource(ctx, ExtractSourceRequest{Path: "sample.go", Language: "go"})
	if err == nil {
		t.Fatal("extractSource accepted a canceled context")
	}
	if kind := xerr.KindFrom(err); kind != xerr.KindCanceled {
		t.Fatalf("error kind = %q; want %q (%v)", kind, xerr.KindCanceled, err)
	}
}
