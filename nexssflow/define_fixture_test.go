package nexssflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
)

func TestDefineDirectiveRegistersRunner(t *testing.T) {
	id := uniqueTestLanguageID(t)
	lines := []string{
		fmt.Sprintf("@lang.define %q {", id),
		`  ext: ".txt"`,
		`  find: ["cat"]`,
		`  file_template: "{}"`,
		`}`,
	}
	result, err := handleDefineDirective(context.Background(), compiler.DirectiveReq{I: 0, File: "test.nflow", Lines: lines})
	if err != nil {
		t.Fatalf("handleDefineDirective: %v", err)
	}
	if result.Next != len(lines) {
		t.Fatalf("Next = %d; want %d", result.Next, len(lines))
	}
	language, err := core.Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	spec, ok := language.Spec()
	if !ok || spec.FileTemplate != "{}" {
		t.Fatalf("runner profile = %+v, %t", spec, ok)
	}
}
