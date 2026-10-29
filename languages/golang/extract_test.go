package golang

import (
	"strings"
	"testing"
)

func TestParseGoSnippets(t *testing.T) {
	goCode := []byte(`package sample

// PublicStruct is exported
type PublicStruct struct {
	ID   string
	Name string
}

type privateStruct struct {
	secret string
}

// ExportedFunc does something
func ExportedFunc(a int, b string) (bool, error) {
	if a > 0 {
		return true, nil
	}
	return false, nil
}

func unexportedFunc() string {
	return "internal"
}
`)

	t.Run("Exported Only (AST Signatures)", func(t *testing.T) {
		snippets, funcCount, err := parseGo("sample.go", goCode, true)
		if err != nil {
			t.Fatalf("parseGo failed: %v", err)
		}

		if funcCount != 1 {
			t.Errorf("expected 1 exported function, got %d", funcCount)
		}

		joined := strings.Join(snippets, "\n\n")

		if !strings.Contains(joined, "type PublicStruct struct") {
			t.Errorf("expected PublicStruct in output:\n%s", joined)
		}
		if strings.Contains(joined, "privateStruct") {
			t.Errorf("did not expect privateStruct when exportedOnly=true")
		}
		if !strings.Contains(joined, "func ExportedFunc(a int, b string) (bool, error) {}") {
			t.Errorf("expected ExportedFunc signature stripped of body, got:\n%s", joined)
		}
		if strings.Contains(joined, "unexportedFunc") {
			t.Errorf("did not expect unexportedFunc when exportedOnly=true")
		}
	})

	t.Run("Include Internal (exportedOnly=false)", func(t *testing.T) {
		snippets, funcCount, err := parseGo("sample.go", goCode, false)
		if err != nil {
			t.Fatalf("parseGo failed: %v", err)
		}

		if funcCount != 2 {
			t.Errorf("expected 2 functions, got %d", funcCount)
		}

		joined := strings.Join(snippets, "\n\n")
		if !strings.Contains(joined, "privateStruct") {
			t.Errorf("expected privateStruct when exportedOnly=false")
		}
		if !strings.Contains(joined, "func unexportedFunc() string {}") {
			t.Errorf("expected unexportedFunc signature when exportedOnly=false")
		}
	})
}

func TestParseGoGenericsAndConstraints(t *testing.T) {
	t.Parallel()

	goCode := []byte(`package generic

type Number interface {
	~int | ~int64 | ~float64
}

type Stack[T any] struct {
	elements []T
}

func (s *Stack[T]) Push(item T) {
	s.elements = append(s.elements, item)
}

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	return zero, false
}

func Sum[T Number](vals []T) T {
	var sum T
	for _, v := range vals {
		sum += v
	}
	return sum
}

func unexportedGeneric[T any](val T) {}
`)

	snippets, funcCount, err := parseGo("generic.go", goCode, true)
	if err != nil {
		t.Fatalf("parseGo failed: %v", err)
	}

	joined := strings.Join(snippets, "\n\n")

	if !strings.Contains(joined, "type Number interface {") {
		t.Errorf("expected Number interface constraint")
	}
	if !strings.Contains(joined, "type Stack[T any] struct") {
		t.Errorf("expected generic Stack[T] struct")
	}
	if !strings.Contains(joined, "func (s *Stack[T]) Push(item T) {}") {
		t.Errorf("expected generic method Push signature")
	}
	if !strings.Contains(joined, "func Sum[T Number](vals []T) T {}") {
		t.Errorf("expected generic function Sum signature")
	}
	if strings.Contains(joined, "unexportedGeneric") {
		t.Errorf("did not expect unexportedGeneric when exportedOnly=true")
	}
	if funcCount != 3 { // Push, Pop, Sum
		t.Errorf("expected 3 exported functions, got %d", funcCount)
	}
}
