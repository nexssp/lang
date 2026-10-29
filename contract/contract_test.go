package contract

import (
	"strings"
	"testing"
)

func TestRenderNFlow(t *testing.T) {
	contracts := []Contract{{
		Name:        "CreateUser",
		Description: "Input shape for a user.\nGenerated from annotations.",
		Fields: []Field{
			{Name: "full_name", JSONName: "name", Type: ValueType{Kind: String}, Required: true},
			{Name: "age", Type: ValueType{Kind: Integer}, Required: false},
			{Name: "tags", Type: ValueType{Kind: Array, Element: &ValueType{Kind: String}}, Required: false},
			{Name: "metadata", Type: ValueType{Kind: Object}, Required: false},
		},
	}}

	got, err := RenderNFlow(contracts)
	if err != nil {
		t.Fatalf("RenderNFlow: %v", err)
	}
	for _, want := range []string{
		"@schema CreateUser {",
		"Source-required fields remain marked in inspect JSON",
		"full_name string `json:\"name\"`",
		"age int\n",
		"tags []string\n",
		"metadata map[string]any\n",
		"#   :request-schema=CreateUser",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered NFlow does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "full_name string `validate:\"required\"`") {
		t.Fatal("renderer must not emit a required tag that rejects valid zero values")
	}
	if strings.Contains(got, "\n@pipeline") {
		t.Fatal("starter pipeline must stay commented until implemented")
	}
}

func TestValidateRejectsInvalidAndDuplicateFields(t *testing.T) {
	cases := []struct {
		name string
		item Contract
		want string
	}{
		{"invalid schema name", Contract{Name: "bad-name", Fields: []Field{{Name: "x", Type: ValueType{Kind: String}}}}, "schema identifier"},
		{"duplicate field", Contract{Name: "Input", Fields: []Field{{Name: "x", Type: ValueType{Kind: String}}, {Name: "x", Type: ValueType{Kind: Integer}}}}, "repeats field"},
		{"duplicate JSON name", Contract{Name: "Input", Fields: []Field{{Name: "x", JSONName: "same", Type: ValueType{Kind: String}}, {Name: "y", JSONName: "same", Type: ValueType{Kind: String}}}}, "JSON field name"},
		{"missing array element is allowed", Contract{Name: "Input", Fields: []Field{{Name: "x", Type: ValueType{Kind: Array}}}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.item.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error = %v; want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateAllRejectsDuplicateSchemas(t *testing.T) {
	item := Contract{Name: "Input", Fields: []Field{{Name: "value", Type: ValueType{Kind: Any}}}}
	if err := ValidateAll([]Contract{item, item}); err == nil || !strings.Contains(err.Error(), "schema name") {
		t.Fatalf("ValidateAll error = %v; want duplicate schema error", err)
	}
}
