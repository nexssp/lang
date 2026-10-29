package nexssflow

import (
	"context"
	"fmt"

	"github.com/nexssp/flow/extensions/external"
	"github.com/nexssp/kernel/xerr"

	core "github.com/nexssp/lang/core"
)

// externalExecAdapter delegates interpreter execution to Flow's canonical
// subprocess action; it is deliberately kept out of the reusable core.
type externalExecAdapter struct{}

func (externalExecAdapter) Run(ctx context.Context, cmd string, args []string, stdin []byte) (stdoutText, stderrText string, runErr error) {
	input := map[string]any{"cmd": cmd, "args": args}
	if len(stdin) > 0 {
		input["input"] = stdin
	}
	result, err := external.Exec.Do(ctx, input)
	return result.Stdout, result.Stderr, err
}

func runLangBlock(ctx context.Context, block *LangBlock, input any) (any, error) {
	language, err := core.Lookup(block.ID, block.Version)
	if err != nil {
		return nil, xerr.Internal(fmt.Sprintf("lang.%s: registry lost language after parse", block.ID), err)
	}
	if language.Encode == nil || language.Decode == nil {
		return nil, xerr.Internal(fmt.Sprintf("lang.%s: runner has no stream codec", block.ID))
	}
	stdin, err := language.Encode(input)
	if err != nil {
		return nil, xerr.Validation(fmt.Sprintf("lang.%s encode input", block.ID), err)
	}
	program := core.Program{ID: block.ID, Version: block.Version, Source: block.Source, Tag: block.Tag, SourceFile: block.SourceFile}
	stdout, _, err := core.Execute(ctx, program, stdin, externalExecAdapter{})
	if err != nil {
		return nil, err
	}
	output, err := language.Decode(stdout)
	if err != nil {
		return nil, xerr.Validation(fmt.Sprintf("lang.%s decode output (stdout=%s)", block.ID, truncate(stdout, 200)), err)
	}
	return output, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
