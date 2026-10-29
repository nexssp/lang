package nexssflow

import (
	"io/fs"
	"testing"

	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
	pythonlang "github.com/nexssp/lang/languages/python"
)

func TestPreflightFixtureIsParsedByAdapter(t *testing.T) {
	fixture := "nflows/testdata/py3_preflight_autoinstall.nflow"
	source, err := fs.ReadFile(pythonlang.NFlowFiles(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	primaries := compiler.NewPrimaryExtensionTable(keywordPrimaries()...)
	parser := compiler.NewParserWithFile(core.WithAutoInstallDisabled(t.Context()), compiler.NewOperatorTable(), primaries, string(source), fixture)
	expr, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var blocks []*LangBlock
	collectLangBlocks(expr, &blocks)
	if len(blocks) != 1 || blocks[0].ID != "python" || blocks[0].Version != "3" {
		t.Fatalf("collected blocks = %+v", blocks)
	}
}
