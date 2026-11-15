package core

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexssp/kernel/xerr"
)

// Ensure is the context-aware auto-install seam. Implementations must not start
// work for an already-canceled context and must honor cancellation while work
// is running. Language is passed by value so an installer cannot mutate the
// registry's stored language.
//
// Pure Go: no nexssp/ensure, no Node.js. Recipes call winget/brew/apt/npm
// directly. See install.go.
type Ensure interface {
	Ensure(ctx context.Context, lang Language, version string) (cmd string, args []string, err error)
}

// EnsureFunc adapts a context-aware function to the Ensure interface.
type EnsureFunc func(context.Context, Language, string) (string, []string, error)

// Ensure implements Ensure.
func (f EnsureFunc) Ensure(ctx context.Context, language Language, version string) (cmd string, args []string, err error) {
	return f(ctx, language, version)
}

func ensureWithContext(ctx context.Context, installer Ensure, lang Language, version string) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("ensure context is required")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	cmd, args, err = installer.Ensure(ctx, lang, version)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	return cmd, args, err
}

// ensureHook is the package-level seam. Production automatically installs
// through runner recipes; tests can replace it with a deterministic stub.
var ensureHook Ensure = installEnsure{}

// DefaultEnsure returns the production context-aware installer, which executes
// the profile's platform recipe and re-resolves the interpreter.
func DefaultEnsure() Ensure { return installEnsure{} }

type noEnsure struct{}

func (noEnsure) Ensure(ctx context.Context, lang Language, version string) (cmd string, args []string, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
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

// Resolve is a non-cancelable convenience wrapper using context.Background.
// Use ResolveContext when the operation must be canceled by its caller.
func Resolve(lang Language, version string) (cmd string, args []string, err error) {
	return ResolveContext(context.Background(), lang, version)
}

// ResolveContext selects an interpreter and, on a miss, installs it using ctx.
// Selector and installer implementations on this path receive the same context.
func ResolveContext(ctx context.Context, lang Language, version string) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("resolve context is required")
	}
	return resolveContext(ctx, lang, version)
}

func resolveContext(ctx context.Context, lang Language, version string) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("resolve context is required")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	if lang.SelectContext == nil {
		return "", nil, &NotInstalledError{ID: lang.ID, Version: version, Cause: xerr.Internal("runner has no command selector")}
	}
	cmd, args, err = lang.SelectContext(ctx, version)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	if err == nil {
		return cmd, args, nil
	}
	if autoInstallDisabled(ctx) {
		return "", nil, &NotInstalledError{ID: lang.ID, Version: version, Cause: ErrNotFound}
	}
	return ensureWithContext(ctx, ensureHook, lang, version)
}
