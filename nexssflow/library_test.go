package nexssflow

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/nexssp/flow/compiler"
	"github.com/nexssp/flow/extensions/description"
	"github.com/nexssp/flow/extensions/require"
	"github.com/nexssp/flow/extensions/symbols"

	core "github.com/nexssp/lang/core"
)

func TestBundleContractAndRunnerExports(t *testing.T) {
	bundle := Bundle(nil)
	if err := compiler.ValidateBundle(bundle); err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}
	if len(bundle.ExportedModules) == 0 {
		t.Fatal("bundle exports no runner profiles")
	}
	for _, name := range bundle.ExportedModules {
		if _, err := bundle.SourceModules.Open(name); err != nil {
			t.Errorf("exported runner %q is unavailable: %v", name, err)
		}
	}
	for _, name := range []string{"nflows/python_block.nflow", "nflows/go.nflow", "nflows/python_file--python_file.py"} {
		if _, err := bundle.Fixtures.Open(name); err != nil {
			t.Errorf("conformance fixture %q is unavailable: %v", name, err)
		}
	}
	if len(bundle.Directives) != 2 || len(keywordPrimaries()) == 0 {
		t.Fatalf("bundle directives=%d primaries=%d", len(bundle.Directives), len(keywordPrimaries()))
	}
}

func TestAdapterIsRegisteredForRequire(t *testing.T) {
	if _, ok := compiler.Lookup(ID); !ok {
		t.Fatal("nexssflow adapter is not registered with the Flow compiler")
	}
}

func TestAdapterResolvesThroughExistingRequirePath(t *testing.T) {
	bundles, err := require.ResolveBundles([]require.Requirement{{Import: "github.com/nexssp/lang/nexssflow"}})
	if err != nil {
		t.Fatalf("ResolveBundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].ID != ID {
		t.Fatalf("resolved bundles = %+v; want one %q bundle", bundles, ID)
	}
}

func TestExportedRunnerProfilesUseNativeFlowMetadata(t *testing.T) {
	adapter := Bundle(nil)
	directives := append([]compiler.Directive{}, description.Bundle(nil).Directives...)
	directives = append(directives, symbols.Bundle(nil).Directives...)
	directives = append(directives, adapter.Directives...)
	dt := compiler.NewDirectiveTable(directives...)

	for _, name := range adapter.ExportedModules {
		t.Run(name, func(t *testing.T) {
			file, err := adapter.SourceModules.Open(name)
			if err != nil {
				t.Fatalf("open profile: %v", err)
			}
			source, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				t.Fatalf("read profile: %v", err)
			}
			clean, meta, err := compiler.Preprocess(context.Background(), dt, string(source), name)
			if err != nil {
				t.Fatalf("Flow preprocess: %v", err)
			}
			if strings.TrimSpace(clean) != "" {
				t.Errorf("metadata-only profile left executable source: %q", clean)
			}
			if meta["description"] == nil {
				t.Fatal("profile did not expose @description metadata")
			}
			namespaces, ok := meta["symbols"].(map[string]compiler.SymbolNamespace)
			if !ok {
				t.Fatal("profile did not expose @symbols metadata")
			}
			runnerSymbols, ok := namespaces["runner"]
			if !ok {
				t.Fatal("profile did not declare the runner symbol namespace")
			}
			id, ok := runnerSymbols.Symbols["id"].(string)
			if !ok || id == "" {
				t.Fatalf("runner.id symbol = %#v; want a non-empty string", runnerSymbols.Symbols["id"])
			}
			spec, err := core.ParseRunner(string(source))
			if err != nil {
				t.Fatalf("parse runner profile: %v", err)
			}
			if id != spec.ID {
				t.Errorf("runner.id symbol %q does not match profile ID %q", id, spec.ID)
			}
		})
	}
}

func TestRunnerSymbolMismatchRejected(t *testing.T) {
	adapter := Bundle(nil)
	directives := append([]compiler.Directive{}, description.Bundle(nil).Directives...)
	directives = append(directives, symbols.Bundle(nil).Directives...)
	directives = append(directives, adapter.Directives...)
	dt := compiler.NewDirectiveTable(directives...)
	profile := `@symbols runner {
  id: "different-id"
}
@lang.runner "runner-symbol-mismatch-test" {
  ext: ".test"
  find: ["test-runner"]
}`
	if _, _, err := compiler.Preprocess(context.Background(), dt, profile, "mismatch.nflow"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched @symbols identity error = %v; want ID mismatch diagnostic", err)
	}
}
