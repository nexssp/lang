package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramForTagResolvesAliasWithoutCheckingInterpreter(t *testing.T) {
	program, err := ProgramForTag("py3", "print(1)")
	if err != nil {
		t.Fatalf("ProgramForTag: %v", err)
	}
	if program.ID != "python" || program.Version != "3" || program.Source != "print(1)" {
		t.Fatalf("ProgramForTag returned %+v", program)
	}
	if program.Tag != "py3" {
		t.Errorf("Tag = %q; want py3", program.Tag)
	}
}

func TestProgramForTagUnknown(t *testing.T) {
	_, err := ProgramForTag("cobol", "IDENTIFICATION DIVISION.")
	var unknown *UnknownTagError
	if !errors.As(err, &unknown) || unknown.Tag != "cobol" {
		t.Fatalf("ProgramForTag error = %T %v; want UnknownTagError(cobol)", err, err)
	}
}

func TestFileProgramForTag(t *testing.T) {
	program, err := FileProgramForTag("py", "/tmp/program.py", "print(1)")
	if err != nil {
		t.Fatalf("FileProgramForTag: %v", err)
	}
	if program.SourceFile != "/tmp/program.py" || program.ID != "python" {
		t.Fatalf("FileProgramForTag returned %+v", program)
	}
}

func TestExecuteUsesInjectedExecutorAndReturnsRawOutput(t *testing.T) {
	const id = "core-execute-test"
	Register(Language{
		ID: id, Ext: ".txt",
		Select: func(string) (string, []string, error) { return "fake-runner", []string{"--mode"}, nil },
	})
	t.Cleanup(func() { delete(Registry, id) })

	called := false
	executor := ExecutorFunc(func(_ context.Context, cmd string, args []string, stdin []byte) (string, string, error) {
		called = true
		if cmd != "fake-runner" || len(args) != 2 || args[0] != "--mode" {
			t.Errorf("executor received %q %#v", cmd, args)
		}
		if len(stdin) != 3 || string(stdin) != "in\n" {
			t.Errorf("stdin = %q; want in newline", stdin)
		}
		if len(args) == 2 {
			contents, readErr := os.ReadFile(args[1])
			if readErr != nil || string(contents) != "source" {
				t.Errorf("materialized source = %q, %v", contents, readErr)
			}
		}
		return "raw output", "", nil
	})
	stdout, _, err := Execute(context.Background(), Program{ID: id, Source: "source"}, []byte("in\n"), executor)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called || stdout != "raw output" {
		t.Fatalf("called=%t stdout=%q", called, stdout)
	}
}

func TestExecuteWrapsSubprocessFailure(t *testing.T) {
	const id = "core-execute-failure-test"
	Register(Language{ID: id, Ext: ".txt", Select: func(string) (string, []string, error) { return "runner", nil, nil }})
	t.Cleanup(func() { delete(Registry, id) })
	_, stderr, err := Execute(context.Background(), Program{ID: id, Source: "x"}, nil,
		ExecutorFunc(func(context.Context, string, []string, []byte) (string, string, error) {
			return "partial", "boom", errors.New("exit 1")
		}))
	var execErr *ExecError
	if !errors.As(err, &execErr) || execErr.Stderr != "boom" || stderr != "boom" {
		t.Fatalf("Execute error = %#v, stderr=%q", err, stderr)
	}
}

func TestRegisterDefineAndRunnerSpecCopy(t *testing.T) {
	const id = "core-define-test"
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(path)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := RegisterDefine(Define{ID: id, Ext: ".txt", Find: []string{filepath.Base(path)}, FileTemplate: "starter", Aliases: []string{"coredefine"}}); err != nil {
		t.Fatalf("RegisterDefine: %v", err)
	}
	t.Cleanup(func() {
		delete(Registry, id)
		aliases = removeAlias(aliases, "coredefine")
	})
	language, err := Lookup(id, "")
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := language.Spec()
	if !ok || spec.FileTemplate != "starter" {
		t.Fatalf("Spec() = %+v, %t", spec, ok)
	}
	spec.Aliases[0] = "mutated"
	copyAgain, _ := language.Spec()
	if copyAgain.Aliases[0] != "coredefine" {
		t.Fatalf("Spec returned a shared alias slice: %#v", copyAgain.Aliases)
	}
}

func removeAlias(source []Alias, tag string) []Alias {
	out := source[:0]
	for _, alias := range source {
		if alias.Tag != tag {
			out = append(out, alias)
		}
	}
	return out
}

func TestRegisterDefineRejectsInvalidAndDuplicateAlias(t *testing.T) {
	if err := RegisterDefine(Define{Ext: ".x", Find: []string{"x"}}); err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("missing ID error = %v", err)
	}
	if err := RegisterDefine(Define{ID: "alias-collision-test", Ext: ".x", Aliases: []string{"py"}, Find: []string{"x"}}); err == nil {
		t.Fatal("RegisterDefine accepted an already registered alias")
	}
	if _, err := Lookup("alias-collision-test", ""); err == nil {
		t.Fatal("failed registration mutated language registry")
	}
}
