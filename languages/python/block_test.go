package python

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nexssp/flow/compiler"

	"github.com/nexssp/lang/contract"
)

func TestReadBlockBodyKeepsBracesInsideCommentsAndStrings(t *testing.T) {
	source := `{
# comment brace must not close the block: }
record = {"literal": "}", "triple": """a brace } inside text"""}
message = f"value is {record['literal']}"
print(message)
}`
	parser := compiler.NewParserWithFile(context.Background(), compiler.NewOperatorTable(), nil, source, "analysis.py")
	body, err := ReadBlockBody(parser)
	if err != nil {
		t.Fatalf("ReadBlockBody: %v", err)
	}
	for _, fragment := range []string{"# comment brace must not close the block: }", `"""a brace } inside text"""`, `f"value is {record['literal']}"`} {
		if !strings.Contains(body, fragment) {
			t.Errorf("Python block body %q does not contain %q", body, fragment)
		}
	}
}

func TestReadBlockBodyReportsMissingCloser(t *testing.T) {
	parser := compiler.NewParserWithFile(context.Background(), compiler.NewOperatorTable(), nil, "{\nprint('unfinished')\n", "analysis.py")
	if _, err := ReadBlockBody(parser); err == nil || !strings.Contains(err.Error(), "unterminated block") {
		t.Fatalf("ReadBlockBody error = %v; want unterminated-block error", err)
	}
}

func TestInspectDataclassAndTypedDict(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for AST inspection tests")
	}
	source := strings.Join([]string{
		"from dataclasses import dataclass, field",
		"from typing import NotRequired, Required, TypedDict",
		"",
		"@dataclass",
		"class Person:",
		"    \"\"\"A person request.\"\"\"",
		"    name: str",
		"    age: int | None = None",
		"    tags: list[str] = field(default_factory=list)",
		"    coordinates: tuple[int, str] = ()",
		"    total: int = field(init=False, default=0)",
		"",
		"class Patch(TypedDict, total=False):",
		"    display_name: str",
		"    id: Required[int]",
		"    note: NotRequired[str]",
	}, "\n")
	got, err := Inspect(context.Background(), source, "", InspectOptions{})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Inspect returned %d contracts; want 2", len(got))
	}
	person := got[0]
	if person.Name != "Person" || person.Description != "A person request." {
		t.Fatalf("Person contract = %+v", person)
	}
	if len(person.Fields) != 4 || !person.Fields[0].Required || person.Fields[1].Required || person.Fields[2].Required || person.Fields[3].Required {
		t.Fatalf("Person fields = %+v; required flags do not match dataclass defaults", person.Fields)
	}
	if person.Fields[0].Type.Kind != contract.String || person.Fields[1].Type.Kind != contract.Integer || person.Fields[2].Type.Kind != contract.Array || person.Fields[2].Type.Element.Kind != contract.String || person.Fields[3].Type.Kind != contract.Array || person.Fields[3].Type.Element.Kind != contract.Any {
		t.Fatalf("Person field types = %+v", person.Fields)
	}
	patch := got[1]
	if patch.Name != "Patch" || patch.Fields[0].Required || !patch.Fields[1].Required || patch.Fields[2].Required {
		t.Fatalf("Patch fields = %+v; Required/NotRequired flags are wrong", patch.Fields)
	}
}

func TestInspectSelectsClassAndReportsMissingClass(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for AST inspection tests")
	}
	source := `from typing import TypedDict
class A(TypedDict):
 value: str
class B(TypedDict):
 count: int
`
	got, err := Inspect(context.Background(), source, "B", InspectOptions{})
	if err != nil {
		t.Fatalf("Inspect selected class: %v", err)
	}
	if len(got) != 1 || got[0].Name != "B" {
		t.Fatalf("selected contracts = %+v; want only B", got)
	}
	if _, err := Inspect(context.Background(), source, "Missing", InspectOptions{}); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("missing-class error = %v", err)
	}
}

func TestInspectDoesNotExecutePythonSource(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for AST inspection tests")
	}
	marker := filepath.Join(t.TempDir(), "source-executed")
	source := "from dataclasses import dataclass\nopen(" + strconv.Quote(marker) + ", 'w').write('executed')\n@dataclass\nclass Model:\n    value: str\n"
	if _, err := Inspect(context.Background(), source, "Model", InspectOptions{}); err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("inspection executed the input module; marker stat error = %v", err)
	}
}

func TestInspectRejectsInvalidPythonSyntax(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for AST inspection tests")
	}
	_, err := Inspect(context.Background(), "class Broken(:\n", "", InspectOptions{})
	if err == nil || !strings.Contains(err.Error(), "Python syntax error") {
		t.Fatalf("Inspect error = %v; want Python syntax diagnostic", err)
	}
}

func TestMapAnnotation(t *testing.T) {
	cases := []struct {
		annotation string
		kind       contract.TypeKind
		item       contract.TypeKind
	}{
		{"str", contract.String, ""},
		{"typing.Optional[int]", contract.Integer, ""},
		{"str | None", contract.String, ""},
		{"list[dict[str, int]]", contract.Array, contract.Object},
		{"tuple[int, str]", contract.Array, contract.Any},
		{"dict[str, object]", contract.Object, ""},
		{"list", contract.Array, contract.Any},
		{"dict", contract.Object, ""},
		{"int | str", contract.Any, ""},
		{"CustomModel", contract.Object, ""},
	}
	for _, tc := range cases {
		t.Run(tc.annotation, func(t *testing.T) {
			got := mapAnnotation(tc.annotation)
			if got.Kind != tc.kind {
				t.Fatalf("mapAnnotation(%q) kind = %q; want %q", tc.annotation, got.Kind, tc.kind)
			}
			if tc.item != "" && (got.Element == nil || got.Element.Kind != tc.item) {
				t.Fatalf("mapAnnotation(%q) element = %+v; want %q", tc.annotation, got.Element, tc.item)
			}
		})
	}
}
