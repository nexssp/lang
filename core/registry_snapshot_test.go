package core

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/xerr"
)

func TestRegisterStoresLanguageValueCopy(t *testing.T) {
	const id = "register-language-value-copy-test"
	original := Language{
		ID:  id,
		Ext: ".original",
		SelectContext: func(context.Context, string) (string, []string, error) {
			return "tool", nil, nil
		},
	}
	registerTestLanguage(t, original)
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		delete(registry, registryKey(id, ""))
	})

	original.ID = "changed"
	original.Ext = ".changed"
	original.SelectContext = nil

	registered, err := Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if registered.ID != id || registered.Ext != ".original" || registered.SelectContext == nil {
		t.Fatalf("registered value changed after caller mutation: %+v", registered)
	}
}

func TestLookupAndRegisteredLanguagesReturnDeepCopies(t *testing.T) {
	const (
		id    = "lookup-deep-copy-test"
		alias = "lookup-copy-alias-test"
	)
	input := &RunnerSpec{
		ID:      id,
		Ext:     ".copy",
		Find:    []string{"original-tool"},
		Aliases: []string{alias},
		Install: OSRecipe{
			Linux: []string{"installer", "original-arg"},
		},
	}
	if err := RegisterRunnerSpec(input); err != nil {
		t.Fatalf("RegisterRunnerSpec: %v", err)
	}
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		delete(registry, registryKey(id, ""))
		filtered := aliases[:0]
		for _, registered := range aliases {
			if registered.ID != id {
				filtered = append(filtered, registered)
			}
		}
		aliases = filtered
	})

	// Mutating the registration input after return cannot alter stored slices.
	input.ID = "changed-input"
	input.Find[0] = "changed-input-tool"
	input.Aliases[0] = "changed-input-alias"
	input.Install.Linux[1] = "changed-input-arg"

	language, err := Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	language.ID = "changed-lookup"
	language.Ext = ".changed"
	language.spec.Find[0] = "changed-private-lookup-tool"
	language.install.Linux[1] = "changed-private-lookup-arg"
	profile, ok := language.Spec()
	if !ok {
		t.Fatal("Lookup value lost its runner profile")
	}
	profile.Find[0] = "changed-profile-tool"
	profile.Aliases[0] = "changed-profile-alias"
	profile.Install.Linux[1] = "changed-profile-arg"

	snapshots := RegisteredLanguages()
	foundSnapshot := false
	for i := range snapshots {
		if snapshots[i].ID != id {
			continue
		}
		foundSnapshot = true
		snapshots[i].ID = "changed-list-snapshot"
		snapshots[i].spec.Find[0] = "changed-private-list-tool"
		snapshots[i].install.Linux[1] = "changed-private-list-arg"
		break
	}
	if !foundSnapshot {
		t.Fatal("RegisteredLanguages omitted the registered language")
	}

	fresh, err := Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup after mutations: %v", err)
	}
	freshProfile, ok := fresh.Spec()
	if !ok {
		t.Fatal("fresh Lookup value lost its runner profile")
	}
	if fresh.ID != id || fresh.Ext != ".copy" || freshProfile.Find[0] != "original-tool" || freshProfile.Install.Linux[1] != "original-arg" {
		t.Fatalf("registry changed through a returned snapshot: lang=%+v profile=%+v", fresh, freshProfile)
	}
	if freshProfile.Aliases[0] != alias {
		t.Fatalf("stored aliases changed: got %v, want %q", freshProfile.Aliases, alias)
	}
	if resolvedID, _, ok := ResolveAlias(alias); !ok || resolvedID != id {
		t.Fatalf("original alias resolves to %q, %t; want %q, true", resolvedID, ok, id)
	}
	if _, _, ok := ResolveAlias("changed-input-alias"); ok {
		t.Fatal("mutated registration input changed the alias registry")
	}
}

func TestUnregisterRemovesLanguageAndAliasesWithoutInvalidatingSnapshots(t *testing.T) {
	const (
		id    = "unregister-snapshot-test"
		alias = "unregister-snapshot-alias-test"
	)
	spec := &RunnerSpec{ID: id, Ext: ".unreg", Find: []string{"tool"}, Aliases: []string{alias}}
	if err := RegisterRunnerSpec(spec); err != nil {
		t.Fatalf("RegisterRunnerSpec: %v", err)
	}
	t.Cleanup(func() { Unregister(id, "") })

	snapshot, err := Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup before unregister: %v", err)
	}
	if !Unregister(id, "") {
		t.Fatal("Unregister returned false for a registered language")
	}
	if Unregister(id, "") {
		t.Fatal("Unregister returned true for an absent language")
	}
	if _, err := Lookup(id, ""); err == nil {
		t.Fatal("Lookup found an unregistered language")
	}
	if _, _, ok := ResolveAlias(alias); ok {
		t.Fatal("alias remained registered after Unregister")
	}
	if snapshot.ID != id {
		t.Fatalf("previous snapshot was invalidated: %+v", snapshot)
	}
	if _, ok := snapshot.Spec(); !ok {
		t.Fatal("previous snapshot lost its runner profile")
	}
	if err := RegisterRunnerSpec(spec); err != nil {
		t.Fatalf("re-register after Unregister: %v", err)
	}
}

func TestRegisterReturnsValidationForEmptyID(t *testing.T) {
	if err := Register(Language{}); xerr.KindFrom(err) != xerr.KindValidation {
		t.Fatalf("Register(empty ID) error = %v; want Validation", err)
	}
}

func TestRegisterReturnsConflictForDuplicateID(t *testing.T) {
	const id = "register-duplicate-id-test"
	if err := Register(Language{ID: id, Ext: ".first"}); err != nil {
		t.Fatalf("Register(first): %v", err)
	}
	t.Cleanup(func() { Unregister(id, "") })
	if err := Register(Language{ID: id, Ext: ".second"}); xerr.KindFrom(err) != xerr.KindConflict {
		t.Fatalf("Register(duplicate ID) error = %v; want Conflict", err)
	}
	registered, err := Lookup(id, "")
	if err != nil {
		t.Fatalf("Lookup after duplicate registration: %v", err)
	}
	if registered.Ext != ".first" {
		t.Fatalf("duplicate registration replaced the original language: %+v", registered)
	}
}

func registerTestLanguage(t *testing.T, language Language) {
	t.Helper()
	if err := Register(language); err != nil {
		t.Fatalf("Register(%q): %v", language.ID, err)
	}
}
