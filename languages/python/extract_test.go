package python

import (
	"strings"
	"testing"
)

func TestParsePythonSnippets(t *testing.T) {
	t.Parallel()

	pyCode := []byte(`
def public_function(a, b):
    return a + b

def _private_helper():
    print("internal")

class PublicClass:
    def __init__(self, name):
        self.name = name

    def public_method(self, value):
        return value * 2

    def _internal_method(self):
        pass

class _PrivateClass:
    def method(self):
        pass
`)

	t.Run("Exported Only (public top-level functions and public class methods)", func(t *testing.T) {
		t.Parallel()

		snippets, funcCount, err := parsePython(pyCode, true)
		if err != nil {
			t.Fatalf("parsePython failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")

		// Top-level public function
		if !strings.Contains(joined, "def public_function(a, b): {}") {
			t.Errorf("expected public_function signature with stripped body, got:\n%s", joined)
		}
		// Private function should be excluded
		if strings.Contains(joined, "_private_helper") {
			t.Errorf("did not expect private function _private_helper when exportedOnly=true")
		}

		// Public class should be present
		if !strings.Contains(joined, "class PublicClass:") {
			t.Errorf("expected PublicClass in output")
		}
		// Public method should be present
		if !strings.Contains(joined, "def public_method(self, value): {}") {
			t.Errorf("expected public_method signature with body stripped, got:\n%s", joined)
		}
		// Private method should be excluded
		if strings.Contains(joined, "_internal_method") {
			t.Errorf("did not expect private method _internal_method when exportedOnly=true")
		}
		// Private class should be excluded
		if strings.Contains(joined, "_PrivateClass") {
			t.Errorf("did not expect private class _PrivateClass when exportedOnly=true")
		}

		// Function count should include public_function and public_method (2)
		if funcCount != 2 {
			t.Errorf("expected 2 public functions, got %d", funcCount)
		}
	})

	t.Run("Include Internal (exportedOnly=false)", func(t *testing.T) {
		t.Parallel()

		snippets, funcCount, err := parsePython(pyCode, false)
		if err != nil {
			t.Fatalf("parsePython failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")

		if !strings.Contains(joined, "def _private_helper(): {}") {
			t.Errorf("expected _private_helper when exportedOnly=false")
		}
		if !strings.Contains(joined, "_PrivateClass") {
			t.Errorf("expected _PrivateClass when exportedOnly=false")
		}
		if !strings.Contains(joined, "def _internal_method(self): {}") {
			t.Errorf("expected _internal_method when exportedOnly=false")
		}
		if funcCount != 6 { // public_function, _private_helper, public_method, _internal_method
			t.Errorf("expected 6 functions, got %d", funcCount)
		}
	})
}

func TestParsePythonNestedFunctionsAndDecorators(t *testing.T) {
	t.Parallel()

	pyCode := []byte(`
@decorator
def decorated_func(x):
    return x * x

def outer():
    def inner():
        pass
    return inner
`)

	snippets, funcCount, err := parsePython(pyCode, false)
	if err != nil {
		t.Fatalf("parsePython failed: %v", err)
	}

	joined := strings.Join(snippets, "\n")
	// Top-level decorated function
	if !strings.Contains(joined, "def decorated_func(x): {}") {
		t.Errorf("expected decorated_func signature, got:\n%s", joined)
	}
	// Nested function inside outer should not appear as a top-level snippet
	if strings.Contains(joined, "def inner():") {
		t.Errorf("nested function inner should not be included as a separate snippet")
	}
	// Outer function itself should be included
	if !strings.Contains(joined, "def outer():") {
		t.Errorf("expected outer function")
	}
	// Function count: decorated_func and outer (2), inner is not counted separately
	if funcCount != 2 {
		t.Errorf("expected 2 functions, got %d", funcCount)
	}
}

func TestParsePythonClassWithoutMethods(t *testing.T) {
	t.Parallel()

	pyCode := []byte(`
class Empty:
    pass
`)

	snippets, funcCount, err := parsePython(pyCode, false)
	if err != nil {
		t.Fatalf("parsePython failed: %v", err)
	}
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet for class, got %d", len(snippets))
	}
	if !strings.Contains(snippets[0], "class Empty:") {
		t.Errorf("expected class Empty, got %s", snippets[0])
	}
	if funcCount != 0 {
		t.Errorf("expected 0 functions, got %d", funcCount)
	}
}
