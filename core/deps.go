package core

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/nexssp/kernel/xerr"
)

// resolveRunCmd selects the ordinary interpreter unless the source opts into
// a runner's dependency tool by containing its configured marker. The file
// path remains a single argument even when it contains spaces.
func resolveRunCmd(ctx context.Context, language *Language, program *Program, filePath string) (cmd string, args []string, err error) {
	if depCmd, depArgs, handled, depErr := resolveDepsCmd(language, program.Source, filePath); handled {
		return depCmd, depArgs, depErr
	}

	cmd, args, err = resolveContext(ctx, language, program.Version)
	if err != nil {
		return "", nil, err
	}
	return cmd, sourcePathArgs(args, language.spec, filePath, runtime.GOOS), nil
}

func sourcePathArgs(args []string, spec *RunnerSpec, filePath, goos string) []string {
	if spec != nil && spec.FilePathMode == "wsl" && goos == windowsGOOS {
		return append(args, "-c", `exec bash "$(wslpath -u "$1")"`, "nexssp-lang", filePath)
	}
	return append(args, filePath)
}

// resolveDepsCmd reports handled=true only when the source uses a configured
// dependency marker. It deliberately does not cache arguments: each execution
// gets its own materialized file path.
func resolveDepsCmd(language *Language, source, filePath string) (cmd string, args []string, handled bool, err error) {
	if language == nil || language.spec == nil || language.spec.Deps == "" || language.spec.DepsMarker == "" || !strings.Contains(source, language.spec.DepsMarker) {
		return "", nil, false, nil
	}

	parts := strings.Fields(language.spec.Deps)
	if len(parts) < 2 || !strings.Contains(strings.Join(parts[1:], " "), "{file}") {
		return "", nil, true, xerr.Validation(fmt.Sprintf("lang.%s: invalid dependency command template", language.ID))
	}
	cmd, err = lookPathCmd(parts[0])
	if err != nil {
		return "", nil, true, &NotInstalledError{
			ID:      language.ID,
			Version: language.Version(),
			Cause:   xerr.NotFound(fmt.Sprintf("dependency runner %q not found on PATH; install it to run scripts with %q", parts[0], language.spec.DepsMarker), err),
		}
	}

	args = make([]string, len(parts)-1)
	for i, part := range parts[1:] {
		args[i] = strings.ReplaceAll(part, "{file}", filePath)
	}
	return cmd, args, true, nil
}
