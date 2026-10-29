package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexssp/kernel/xerr"
)

// TestParseRunnerVersioned asserts the @lang.runner "python" "3" header
// form is parsed correctly.
func TestParseRunnerVersioned(t *testing.T) {
	raw := `@lang.runner "python" "3" {
  ext: ".py"
  aliases: ["py3", "python3"]
  find: ["python3", "py -3"]
}`
	spec, err := ParseRunner(raw)
	if err != nil {
		t.Fatalf("ParseRunner: %v", err)
	}
	if spec.ID != "python" || spec.Version != "3" {
		t.Errorf("ID/Version = %q/%q; want python/3", spec.ID, spec.Version)
	}
}

// TestParseRunnerErrors exercises the validation paths.
func TestParseRunnerErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"no runner block", `# just a comment`, "runner profile"},
		{"unterminated", `@lang.runner "x" {
  ext: ".py"`, "unterminated block"},
		{"missing ext", `@lang.runner "x" {
  find: ["go"]
}`, "ext"},
		{"missing find", `@lang.runner "x" {
  ext: ".x"
}`, "find"},
		{"missing id", `@lang.runner "" {
  ext: ".x"
  find: ["go"]
}`, "cannot be empty"},
		{"file template requires JSON codecs", `@lang.runner "x" {
  ext: ".x"
  find: ["x"]
  stdin: "text"
  file_template: "starter"
}`, "requires stdin and stdout codecs to be json"},
		{"JSON runner block is not supported", `{"id":"x","ext":".py","find":["go"]}`, "runner profile must contain an @lang.runner or @lang.define block"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRunner(c.src)
			if err == nil {
				t.Fatal("ParseRunner returned nil error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q; want substring %q", err, c.want)
			}
		})
	}
}

// TestLoadRunners loads the embedded runners/*.nflow and asserts every
// expected language and alias is registered. This is the integration test
// for the data-driven model — if a runner file is malformed or an alias
// drifts, this fails.
func TestLoadRunners(t *testing.T) {
	// Reset registry + aliases; LoadRunners appends to both.
	for k := range Registry {
		delete(Registry, k)
	}
	aliases = nil
	if err := LoadRunners(); err != nil {
		t.Fatalf("LoadRunners: %v", err)
	}

	// Every shipped language must be registered.
	for _, id := range []string{"python", "rust", "go", "typescript", "javascript", "shell", "bash", "wsl", "powershell"} {
		if _, ok := Registry[id]; !ok {
			t.Errorf("LoadRunners did not register %q", id)
		}
	}
	for _, key := range []string{"python", "python@2", "python@3", "rust", "go", "typescript", "javascript", "powershell"} {
		language := Registry[key]
		if language == nil || language.spec == nil || language.spec.FileTemplate == "" {
			t.Errorf("runner %q does not declare a file_template", key)
			continue
		}
		if language.spec.Stdin != "" && language.spec.Stdin != "json" || language.spec.Stdout != "" && language.spec.Stdout != "json" {
			t.Errorf("runner %q file_template must use JSON codecs", key)
		}
	}
}

// TestLoadRunnersAliases asserts every documented alias resolves.
func TestLoadRunnersAliases(t *testing.T) {
	for k := range Registry {
		delete(Registry, k)
	}
	aliases = nil
	if err := LoadRunners(); err != nil {
		t.Fatalf("LoadRunners: %v", err)
	}

	cases := []struct {
		tag         string
		wantID      string
		wantVersion string
	}{
		{"py", "python", ""},
		{"python", "python", ""},
		{"py2", "python", "2"},
		{"python2", "python", "2"},
		{"py3", "python", "3"},
		{"python3", "python", "3"},
		{"rs", "rust", ""},
		{"rust", "rust", ""},
		{"go", "go", ""},
		{"golang", "go", ""},
		{"ts", "typescript", ""},
		{"typescript", "typescript", ""},
		{"js", "javascript", ""},
		{"node", "javascript", ""},
		{"sh", "shell", ""},
		{"bash", "bash", ""},
		{"wsl", "wsl", ""},
		{"ps", "powershell", ""},
		{"pwsh", "powershell", ""},
		{"powershell", "powershell", ""},
	}
	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			id, ver, ok := resolveAlias(c.tag)
			if !ok {
				t.Fatalf("resolveAlias(%q) returned ok=false; want true", c.tag)
			}
			if id != c.wantID || ver != c.wantVersion {
				t.Errorf("resolveAlias(%q) = (%q, %q); want (%q, %q)",
					c.tag, id, ver, c.wantID, c.wantVersion)
			}
		})
	}
}

// TestRunnerSpecFindCmd asserts the find: list walks correctly and the
// first on-PATH candidate wins.
func TestRunnerSpecFindCmd(t *testing.T) {
	// Use a spec we know resolves on this machine (python3 or python).
	spec := &RunnerSpec{Find: []string{"python3", "python", "py -3"}}
	cmd, args, err := spec.FindCmd()
	if err != nil {
		t.Skipf("no python on PATH: %v", err)
	}
	if cmd == "" {
		t.Fatal("FindCmd returned empty cmd with nil err")
	}
	// If "py -3" was the winner, args should be ["-3"].
	if strings.HasSuffix(cmd, "py") && (len(args) == 0 || args[0] != "-3") {
		t.Errorf("FindCmd on 'py -3' returned args=%v; want [-3]", args)
	}
}

func TestRunnerSpecFindCmdPythonLauncherAvailability(t *testing.T) {
	const launcher = `C:\WINDOWS\py.exe`
	const interpreter = `C:\Python312\python.exe`
	lookup := func(name string) (string, error) {
		switch name {
		case "py":
			return launcher, nil
		case "python3":
			return interpreter, nil
		default:
			return "", ErrNotFound
		}
	}

	t.Run("unavailable launcher is skipped", func(t *testing.T) {
		spec := RunnerSpec{Find: []string{"py -3", "python3"}}
		cmd, args, err := spec.findCmd(lookup, func(cmd string, args []string) bool {
			if cmd != launcher {
				return true
			}
			if len(args) != 1 || args[0] != "-3" {
				t.Fatalf("launcher probe got %q %#v; want py.exe [-3]", cmd, args)
			}
			return false
		})
		if err != nil {
			t.Fatalf("findCmd: %v", err)
		}
		if cmd != interpreter || len(args) != 0 {
			t.Fatalf("findCmd = %q %#v; want usable interpreter %q", cmd, args, interpreter)
		}
	})

	t.Run("usable launcher is preserved", func(t *testing.T) {
		spec := RunnerSpec{Find: []string{"py -3"}}
		cmd, args, err := spec.findCmd(lookup, func(cmd string, args []string) bool {
			return cmd == launcher && len(args) == 1 && args[0] == "-3"
		})
		if err != nil {
			t.Fatalf("findCmd: %v", err)
		}
		if cmd != launcher || len(args) != 1 || args[0] != "-3" {
			t.Fatalf("findCmd = %q %#v; want usable launcher %q [-3]", cmd, args, launcher)
		}
	})

	t.Run("unavailable launcher with no fallback is missing", func(t *testing.T) {
		spec := RunnerSpec{Find: []string{"py -3"}}
		_, _, err := spec.findCmd(lookup, func(string, []string) bool { return false })
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("findCmd error = %v; want ErrNotFound", err)
		}
	})
}

func TestIsWindowsPythonLauncher(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{`C:\WINDOWS\py.exe`, true},
		{`C:\Tools\PY.EXE`, true},
		{`C:\Python312\python.exe`, false},
		{`/usr/bin/py`, false},
	} {
		if got := isWindowsPythonLauncher(tc.cmd); got != tc.want {
			t.Errorf("isWindowsPythonLauncher(%q) = %t; want %t", tc.cmd, got, tc.want)
		}
	}
}

// TestRunnerSpecFindCmdMiss asserts ErrNotFound when no candidate resolves.
func TestRunnerSpecFindCmdMiss(t *testing.T) {
	spec := &RunnerSpec{Find: []string{"totally-fake-binary-xyz"}}
	_, _, err := spec.FindCmd()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindCmd miss returned err=%v; want ErrNotFound", err)
	}
}

// TestRunnerSpecCodecs asserts the codec resolution from stdin/stdout fields.
func TestRunnerSpecCodecs(t *testing.T) {
	t.Run("json defaults", func(t *testing.T) {
		s := RunnerSpec{ID: "x", Stdin: "", Stdout: ""}
		enc, err := s.Encode()
		if err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		if enc == nil {
			t.Error("Encode() returned nil func; want non-nil")
		}
		dec, err := s.Decode()
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}
		if dec == nil {
			t.Error("Decode() returned nil func; want non-nil")
		}
	})
	t.Run("text", func(t *testing.T) {
		s := RunnerSpec{ID: "x", Stdin: "text", Stdout: "raw"}
		enc, _ := s.Encode()
		dec, _ := s.Decode()
		if enc == nil || dec == nil {
			t.Fatal("text codecs returned nil")
		}
		// encodeText on a string returns the bytes verbatim
		b, _ := enc("hello")
		if string(b) != "hello" {
			t.Errorf("encodeText = %q; want hello", b)
		}
		// decodeText on bytes returns the string
		v, _ := dec("world")
		if v != "world" {
			t.Errorf("decodeText = %v; want world", v)
		}
	})
	t.Run("bad codec", func(t *testing.T) {
		s := RunnerSpec{ID: "x", Stdin: "yaml"}
		if _, err := s.Encode(); err == nil {
			t.Error("Encode() with bad codec returned nil error")
		}
	})
}

func TestRunnerErrorsUseKernelKinds(t *testing.T) {
	if _, err := Lookup("runner-kind-missing-language-test", ""); xerr.KindFrom(err) != xerr.KindNotFound {
		t.Fatalf("unknown language error kind = %q; want NotFound", xerr.KindFrom(err))
	}
	if xerr.KindFrom(&NotInstalledError{ID: "python"}) != xerr.KindNotFound {
		t.Fatal("NotInstalledError does not expose Kernel NotFound classification")
	}
	if _, err := (RunnerSpec{ID: "bad", Stdin: "yaml"}).Encode(); xerr.KindFrom(err) != xerr.KindValidation {
		t.Fatalf("Encode error kind = %q; want Validation", xerr.KindFrom(err))
	}
	if _, err := ParseRunner(`@lang.runner "bad" {
  find: ["tool"]
}`); xerr.KindFrom(err) != xerr.KindValidation {
		t.Fatalf("ParseRunner error kind = %q; want Validation", xerr.KindFrom(err))
	}
	if _, err := ParseRunner(`@lang.runner "bad" {
  ext: ".x"
  find: ["tool"]
  file_template_file: "../python.py"
}`); xerr.KindFrom(err) != xerr.KindValidation {
		t.Fatalf("template path error kind = %q; want Validation", xerr.KindFrom(err))
	}
	if _, err := ParseRunner(`@lang.runner "bad" {
  ext: ".sh"
  find: ["wsl bash"]
  file_path_mode: "powershell"
}`); xerr.KindFrom(err) != xerr.KindValidation {
		t.Fatalf("file path mode error kind = %q; want Validation", xerr.KindFrom(err))
	}

	const id = "runner-conflict-error-kind-test"
	spec := &RunnerSpec{ID: id, Ext: ".test", Find: []string{"test-runner"}}
	if err := RegisterRunnerSpec(spec); err != nil {
		t.Fatalf("RegisterRunnerSpec: %v", err)
	}
	t.Cleanup(func() { delete(Registry, id) })
	if err := RegisterRunnerSpec(spec); xerr.KindFrom(err) != xerr.KindConflict {
		t.Fatalf("duplicate registration error kind = %q; want Conflict", xerr.KindFrom(err))
	}
}

func TestWindowsShellProfilesAreExplicit(t *testing.T) {
	for id, want := range map[string][]string{
		"bash":       {"bash"},
		"wsl":        {"wsl bash"},
		"powershell": {"pwsh -File", "powershell -File"},
	} {
		language, err := Lookup(id, "")
		if err != nil {
			t.Fatalf("Lookup(%q): %v", id, err)
		}
		spec, ok := language.Spec()
		if !ok || strings.Join(spec.Find, "|") != strings.Join(want, "|") {
			t.Errorf("%s candidates = %v; want %v", id, spec.Find, want)
		}
	}
	wsl, err := Lookup("wsl", "")
	if err != nil {
		t.Fatal(err)
	}
	wslSpec, ok := wsl.Spec()
	if !ok || wslSpec.FilePathMode != "wsl" {
		t.Fatalf("WSL file path mode = %q; want wsl", wslSpec.FilePathMode)
	}
}
