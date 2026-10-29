package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/nexssp/kernel/xerr"
)

// Executor runs one resolved program and returns stdout and stderr as strings.
// Implementations must honor context cancellation.
type Executor interface {
	Run(ctx context.Context, cmd string, args []string, stdin []byte) (stdout, stderr string, err error)
}

// ExecutorFunc adapts a function to the Executor interface.
type ExecutorFunc func(context.Context, string, []string, []byte) (string, string, error)

// Run implements Executor.
func (f ExecutorFunc) Run(ctx context.Context, cmd string, args []string, stdin []byte) (stdout, stderr string, err error) {
	return f(ctx, cmd, args, stdin)
}

// DefaultExecutor runs the selected interpreter with os/exec. Hosts can inject
// a different Executor into Execute to centralize subprocess policy.
type DefaultExecutor struct{}

// Run implements Executor.
func (DefaultExecutor) Run(ctx context.Context, cmd string, args []string, stdin []byte) (stdoutText, stderrText string, runErr error) {
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, cmd, args...)
	command.Stdin = bytes.NewReader(stdin)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// Execute runs a Program using raw stdin bytes and returns raw stdout bytes as
// a string. Value serialization/deserialization is intentionally left to the
// host adapter. Inline source is materialized in a private temporary directory;
// file-backed programs use their original path and contents.
func Execute(ctx context.Context, program Program, stdin []byte, executor Executor) (stdout, stderr string, err error) {
	if ctx == nil {
		return "", "", xerr.Validation("lang: execution context is required")
	}
	if executor == nil {
		executor = DefaultExecutor{}
	}
	language, err := Lookup(program.ID, program.Version)
	if err != nil {
		return "", "", xerr.Internal(fmt.Sprintf("lang.%s: registry lost language after parse", program.ID), err)
	}

	filePath := program.SourceFile
	runProgram := program
	if filePath == "" {
		dir, materializedPath, materializeErr := materialize(ctx, language, program.Source)
		if materializeErr != nil {
			return "", "", materializeErr
		}
		defer cleanupMaterialized(dir)
		filePath = materializedPath
	} else {
		source, readErr := os.ReadFile(filePath)
		if readErr != nil {
			message := fmt.Sprintf("lang.%s: read source file %q", language.ID, filePath)
			switch {
			case errors.Is(readErr, os.ErrNotExist):
				return "", "", xerr.NotFound(message, readErr)
			case errors.Is(readErr, os.ErrPermission):
				return "", "", xerr.Forbidden(message, readErr)
			default:
				return "", "", xerr.BadRequest(message, readErr)
			}
		}
		runProgram.Source = string(source)
	}

	cmd, args, err := resolveRunCmd(ctx, language, &runProgram, filePath)
	if err != nil {
		return "", "", err
	}
	stdout, stderr, err = executor.Run(ctx, cmd, args, stdin)
	if err != nil {
		return stdout, stderr, &ExecError{ID: language.ID, Cmd: cmd, Cause: err, Stderr: stderr}
	}
	return stdout, stderr, nil
}

// ExecError is returned when an interpreter process exits unsuccessfully.
type ExecError struct {
	ID     string
	Cmd    string
	Cause  error
	Stderr string
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("lang.%s: %s failed: %v\nstderr:\n%s", e.ID, e.Cmd, e.Cause, e.Stderr)
}

func (e *ExecError) Unwrap() error { return e.Cause }
