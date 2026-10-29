package zig

import (
	"strings"
	"testing"
)

func TestParseZigSnippets(t *testing.T) {
	t.Parallel()

	zigCode := []byte(`
pub fn public_add(a: i32, b: i32) i32 {
    return a + b;
}

fn private_sub(a: i32, b: i32) i32 {
    return a - b;
}

pub const MyStruct = struct {
    x: i32,
    y: i32,

    pub fn init(x: i32, y: i32) MyStruct {
        return MyStruct{ .x = x, .y = y };
    }

    fn internal(self: MyStruct) i32 {
        return self.x + self.y;
    }
};

const PrivateStruct = struct {
    z: i32,
};

pub const EnumKind = enum {
    A,
    B,
    C,
};
`)

	t.Run("Exported Only (pub declarations)", func(t *testing.T) {
		t.Parallel()

		snippets, funcCount, err := parseZig(zigCode, true)
		if err != nil {
			t.Fatalf("parseZig failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")

		// Public function
		if !strings.Contains(joined, "pub fn public_add(a: i32, b: i32) i32 {}") {
			t.Errorf("expected public_add signature with body stripped, got:\n%s", joined)
		}
		// Private function should be excluded
		if strings.Contains(joined, "private_sub") {
			t.Errorf("did not expect private function private_sub when exportedOnly=true")
		}

		// Public struct
		if !strings.Contains(joined, "pub const MyStruct = struct {") {
			t.Errorf("expected MyStruct in output")
		}
		// Public method inside struct (init)
		if !strings.Contains(joined, "pub fn init(x: i32, y: i32) MyStruct {}") {
			t.Errorf("expected init method signature, got:\n%s", joined)
		}
		// Private method inside struct should be excluded
		if strings.Contains(joined, "fn internal(self: MyStruct) i32") {
			t.Errorf("did not expect internal method when exportedOnly=true")
		}

		// Private struct should be excluded
		if strings.Contains(joined, "PrivateStruct") {
			t.Errorf("did not expect PrivateStruct when exportedOnly=true")
		}

		// Public enum
		if !strings.Contains(joined, "pub const EnumKind = enum {") {
			t.Errorf("expected EnumKind in output")
		}

		// Function count: public_add + init (2)
		if funcCount != 2 {
			t.Errorf("expected 2 public functions, got %d", funcCount)
		}
	})

	t.Run("Include Internal (exportedOnly=false)", func(t *testing.T) {
		t.Parallel()

		snippets, funcCount, err := parseZig(zigCode, false)
		if err != nil {
			t.Fatalf("parseZig failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")

		if !strings.Contains(joined, "fn private_sub(a: i32, b: i32) i32 {}") {
			t.Errorf("expected private_sub when exportedOnly=false")
		}
		if !strings.Contains(joined, "const PrivateStruct = struct {") {
			t.Errorf("expected PrivateStruct when exportedOnly=false")
		}
		if !strings.Contains(joined, "fn internal(self: MyStruct) i32 {}") {
			t.Errorf("expected internal method when exportedOnly=false")
		}
		// Function count: public_add, private_sub, init, internal = 4
		if funcCount != 4 {
			t.Errorf("expected 4 functions, got %d", funcCount)
		}
	})
}

func TestParseZigNoBody(t *testing.T) {
	t.Parallel()

	zigCode := []byte(`
pub fn declared_only() void;
`)

	snippets, funcCount, err := parseZig(zigCode, true)
	if err != nil {
		t.Fatalf("parseZig failed: %v", err)
	}
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}
	if !strings.Contains(snippets[0], "pub fn declared_only() void;") {
		t.Errorf("expected exact declaration, got %s", snippets[0])
	}
	if funcCount != 1 {
		t.Errorf("expected 1 function, got %d", funcCount)
	}
}

func TestParseZigEmptyContainer(t *testing.T) {
	t.Parallel()

	zigCode := []byte(`
pub const EmptyStruct = struct {};
`)

	snippets, _, err := parseZig(zigCode, true)
	if err != nil {
		t.Fatalf("parseZig failed: %v", err)
	}
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}
	if !strings.Contains(snippets[0], "pub const EmptyStruct = struct {};") {
		t.Errorf("expected struct with empty body, got %s", snippets[0])
	}
}
