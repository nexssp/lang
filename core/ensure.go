package core

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexssp/kernel/xerr"
)

// Ensure is the auto-install seam.
//
// Resolve calls Ensure when a language's Select (find-only) fails. The
// default installEnsure runs the runner's per-OS recipe and re-resolves the
// interpreter; tests or hosts can replace the hook when needed.
//
// Pure Go: no nexssp/ensure, no Node.js. Recipes call winget/brew/apt/npm
// directly. See install.go.
type Ensure interface {
	Ensure(lang *Language, version string) (cmd string, args []string, err error)
}

// EnsureFunc adapts a function to the Ensure interface.
type EnsureFunc func(*Language, string) (string, []string, error)

// Ensure implements Ensure.
func (f EnsureFunc) Ensure(language *Language, version string) (cmd string, args []string, err error) {
	return f(language, version)
}

// ensureHook is the package-level seam. Production automatically installs
// through runner recipes; tests can replace it with a deterministic stub.
var ensureHook Ensure = installEnsure{}

// DefaultEnsure returns the production installer, which executes the current
// platform's recipe declared by the runner profile and then resolves again.
func DefaultEnsure() Ensure { return installEnsure{} }

type noEnsure struct{}

func (noEnsure) Ensure(lang *Language, version string) (cmd string, args []string, err error) {
	return "", nil, &NotInstalledError{ID: lang.ID, Version: version}
}

// NotInstalledError is the unified "interpreter missing" error. Preflight
// groups these into a single Report; the CLI prints the report and aborts.
type NotInstalledError struct {
	ID, Version string
	Cause       error // optional: why an auto-install attempt failed
}

func (e *NotInstalledError) Error() string {
	prefix := "lang." + e.ID
	if e.Version != "" {
		prefix += "@" + e.Version
	}
	if e.Cause != nil {
		return prefix + ": " + e.Cause.Error()
	}
	return prefix + ": interpreter not found on PATH"
}

func (e *NotInstalledError) Unwrap() error {
	if e.Cause == nil {
		return xerr.NotFound(e.Error(), ErrNotFound)
	}
	if xerr.KindFrom(e.Cause) != "" {
		return e.Cause
	}
	return xerr.NotFound(e.Error(), e.Cause)
}

// ErrNotFound is the sentinel from lookPathCmd. Tests assert against this
// rather than a string match.
var ErrNotFound = xerr.NotFound("interpreter not found")

// lookPathCmd finds name on PATH, with Windows .cmd/.exe/.bat fallbacks.
//
// exec.LookPath on Windows tries PATHEXT extensions, but its behavior
// across Go versions is inconsistent for shims installed by npm/pnpm
// (bun.cmd, deno.cmd, tsx.cmd). The explicit suffix tries are insurance.
func lookPathCmd(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil && !isWindowsStorePythonAlias(p) {
		return p, nil
	}
	if runtime.GOOS == windowsGOOS {
		for _, ext := range []string{".cmd", ".exe", ".bat"} {
			if p, err := exec.LookPath(name + ext); err == nil && !isWindowsStorePythonAlias(p) {
				return p, nil
			}
		}
	}
	return "", ErrNotFound
}

func isWindowsStorePythonAlias(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
	if !strings.Contains(normalized, "/microsoft/windowsapps/") {
		return false
	}
	name := strings.TrimSuffix(filepath.Base(normalized), ".exe")
	return name == "python" || name == "python3"
}

// Resolve is the default runtime entry point for "give me a runnable command."
//
// It tries a find-only selector, preferring SelectContext when available. On
// miss, it calls ensureHook, which installs and re-resolves by default or
// returns a NotInstalledError if the runner has no recipe or installation
// fails. Context-scoped internal callers may disable installation without
// changing this production default. Preflight uses the find-only selector
// directly and never triggers installation.
func Resolve(lang *Language, version string) (cmd string, args []string, err error) {
	if lang == nil {
		return "", nil, &NotInstalledError{Version: version, Cause: xerr.Validation("nil language")}
	}
	return resolveContext(context.Background(), lang, version)
}

func resolveContext(ctx context.Context, lang *Language, version string) (cmd string, args []string, err error) {
	if lang == nil {
		return "", nil, &NotInstalledError{Version: version, Cause: xerr.Validation("nil language")}
	}
	if lang.Select == nil && lang.SelectContext == nil {
		return "", nil, &NotInstalledError{ID: lang.ID, Version: version, Cause: xerr.Internal("runner has no command selector")}
	}
	if lang.SelectContext != nil {
		cmd, args, err = lang.SelectContext(ctx, version)
	} else {
		cmd, args, err = lang.Select(version)
	}
	if err == nil {
		return cmd, args, nil
	}
	if autoInstallDisabled(ctx) {
		return "", nil, &NotInstalledError{ID: lang.ID, Version: version, Cause: ErrNotFound}
	}
	return ensureHook.Ensure(lang, version)
}
