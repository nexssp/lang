package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/nexssp/kernel/xerr"
)

// installEnsure is the production Ensure impl: run the per-OS recipe, then
// re-resolve via the language's find-only selector (no recursion).
//
// Pure Go: no nexssp/ensure, no Node.js. Recipes call winget/brew/apt/npm
// directly via os/exec. The recipe lives in the runner file's install:
// block; this file is the generic runner.
//
// KISS: one recipe per (language, OS). No version matrices — that's
// flow_version's territory.
type installEnsure struct{}

// installer runs package-manager commands only when installation is explicitly
// requested. Runtime language programs use the separate external.Exec adapter.
type installer interface {
	Run(ctx context.Context, cmd string, args []string) error
}

type defaultInstaller struct{}

func (defaultInstaller) Run(ctx context.Context, cmd string, args []string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	// CommandContext kills the directly launched process on cancellation. It
	// does not guarantee termination of descendants spawned by that process.
	return c.Run()
}

var installerHook installer = defaultInstaller{}

// Ensure runs the recipe and re-resolves the interpreter using ctx.
//
// Installation failures return NotInstalledError so preflight can group them;
// cancellation returns ctx.Err directly. The default installer terminates the
// launched process when canceled; descendants are not guaranteed to terminate.
//
// Failure modes:
//   - no recipe for this OS  -> NotInstalledError with "no auto-install"
//   - recipe command missing -> NotInstalledError with the underlying error
//   - recipe exits non-zero  -> NotInstalledError with stderr
//   - recipe runs, LookPath still fails -> NotInstalledError (PATH not
//     refreshed in this process — common on Windows after winget)
func (installEnsure) Ensure(ctx context.Context, lang Language, version string) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("install context is required")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	recipe := lang.install.RecipeFor()
	if len(recipe) == 0 {
		return "", nil, &NotInstalledError{
			ID: lang.ID, Version: version,
			Cause: xerr.NotFound(fmt.Sprintf("no auto-install recipe for %s on %s; install manually", lang.ID, runtime.GOOS)),
		}
	}
	if runErr := installerHook.Run(ctx, recipe[0], recipe[1:]); runErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", nil, ctxErr
		}
		return "", nil, &NotInstalledError{
			ID: lang.ID, Version: version,
			Cause: xerr.Unavailable(fmt.Sprintf("install recipe %v failed", recipe), runErr),
		}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	// Re-resolve via the find-only selector. No recursion into Ensure.
	if lang.SelectContext == nil {
		err = xerr.Internal("runner has no command selector")
	} else {
		selectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd, args, err = lang.SelectContext(selectCtx, version)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	if err == nil {
		return cmd, args, nil
	}
	return "", nil, &NotInstalledError{
		ID: lang.ID, Version: version,
		Cause: xerr.NotFound("installed but not on PATH (may need shell restart)"),
	}
}

// Install is a non-cancelable convenience wrapper using context.Background.
// Use InstallContext when the operation must be canceled by its caller.
func Install(id, version string) (cmd string, args []string, err error) {
	return InstallContext(context.Background(), id, version)
}

// InstallContext installs the registered language with caller cancellation.
func InstallContext(ctx context.Context, id, version string) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("install context is required")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	lang, lookupErr := Lookup(id, version)
	if lookupErr != nil {
		return "", nil, lookupErr
	}
	return installEnsure{}.Ensure(ctx, lang, version)
}
