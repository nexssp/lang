package core

import (
	"testing"

	"github.com/nexssp/kernel/xerr"
)

// TestAliasUnknownTag ensures unknown tags return ok=false rather than panicking.
func TestAliasUnknownTag(t *testing.T) {
	for _, tag := range []string{"", "cobol", "pascal", "PY", "Python"} {
		if _, _, ok := resolveAlias(tag); ok {
			t.Errorf("resolveAlias(%q) returned ok=true for unknown tag", tag)
		}
	}
}

// TestAliasDerivedFromRunners asserts the alias table is derived from the
// runner files, not hand-maintained. After LoadRunners, every alias
// resolves to its runner's (ID, Version).
func TestAliasDerivedFromRunners(t *testing.T) {
	for k := range registry {
		delete(registry, k)
	}
	aliases = nil
	if err := LoadRunners(); err != nil {
		t.Fatalf("LoadRunners: %v", err)
	}

	// Every alias in the derived table must point at a registered language.
	for _, a := range aliases {
		if _, err := Lookup(a.ID, a.Version); err != nil {
			t.Errorf("alias %q -> %s@%q not registered: %v", a.Tag, a.ID, a.Version, err)
		}
	}
}

// TestAliasDuplicateReturnsConflict ensures duplicate aliases are reported
// without panicking or replacing the original mapping.
func TestAliasDuplicateReturnsConflict(t *testing.T) {
	if err := registerAlias("py", "rust", ""); xerr.KindFrom(err) != xerr.KindConflict {
		t.Fatalf("duplicate alias error = %v; want Conflict", err)
	}
	if id, _, ok := ResolveAlias("py"); !ok || id != "python" {
		t.Fatalf("duplicate registration changed @py to %q, %t; want python, true", id, ok)
	}
}

// TestAllAliasesSorted asserts allAliases returns sorted-by-tag output.
// Stable output is needed for `nflow explain lang` and deterministic tests.
func TestAllAliasesSorted(t *testing.T) {
	got := allAliases()
	for i := 1; i < len(got); i++ {
		if got[i-1].Tag > got[i].Tag {
			t.Errorf("allAliases not sorted: %q before %q", got[i-1].Tag, got[i].Tag)
		}
	}
}
