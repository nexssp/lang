package nexssflow

import (
	"context"

	"github.com/nexssp/flow/compiler"
	"github.com/nexssp/kernel/xctx"

	core "github.com/nexssp/lang/core"
)

var preflightDisabledKey = xctx.NewKey[bool]("nexssflow.lang.preflight-disabled")

// WithPreflightDisabled marks a compile-only context that must not probe
// external interpreters. It is intended for syntax-only compilation, not run.
func WithPreflightDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return preflightDisabledKey.With(ctx, true)
}

func preflightDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, _ := preflightDisabledKey.From(ctx)
	return disabled
}

// preflightValidator walks the Flow AST, gathers language blocks, and invokes
// the reusable core availability check before any action is built or run.
func preflightValidator(ctx context.Context, ast compiler.Expr, _ map[string]any) error {
	if preflightDisabled(ctx) {
		return nil
	}
	var blocks []*LangBlock
	collectLangBlocks(ast, &blocks)
	if len(blocks) == 0 {
		return nil
	}
	programs := make([]core.Program, 0, len(blocks))
	for _, block := range blocks {
		programs = append(programs, core.Program{
			ID: block.ID, Version: block.Version, Source: block.Source, Tag: block.Tag,
		})
	}
	var installer core.Ensure
	if !core.AutoInstallDisabled(ctx) {
		installer = core.DefaultEnsure()
	}
	report := (core.Preflight{Installer: installer}).RunContext(ctx, programs)
	if report.OK() {
		return nil
	}
	return report
}

func collectLangBlocks(expr compiler.Expr, out *[]*LangBlock) {
	if expr == nil {
		return
	}
	if blockExpr, ok := expr.(*langBlockExpr); ok {
		*out = append(*out, blockExpr.block)
	}
	for _, child := range expr.Children() {
		collectLangBlocks(child, out)
	}
}
