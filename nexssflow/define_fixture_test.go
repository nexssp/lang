package nexssflow

import (
	"context"
	"testing"

	"github.com/nexssp/flow/compiler"

	core "github.com/nexssp/lang/core"
)

func TestDefineDirectiveRegistersRunner(t *testing.T) {
	const id = "adapter-inline-define-test"
	lines := []string{
		`@lang.define "adapter-inline-define-test" {`,
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
	t.Cleanup(func() { delete(core.Registry, id) })
	language, err := core.Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	spec, ok := language.Spec()
	if !ok || spec.FileTemplate != "{}" {
		t.Fatalf("runner profile = %+v, %t", spec, ok)
	}
}
