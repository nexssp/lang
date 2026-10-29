package nexssflow

import (
	"errors"
	"testing"

	core "github.com/nexssp/lang/core"
)

func TestPreflightValidatorIgnoresTreesWithoutLangBlocks(t *testing.T) {
	if err := preflightValidator(t.Context(), nil, nil); err != nil {
		t.Fatalf("preflightValidator(nil): %v", err)
	}
}

func TestPreflightValidatorChecksLanguageBlocks(t *testing.T) {
	const id = "preflight-validator-present-test"
	selectCalls := 0
	core.Register(core.Language{ID: id, Ext: ".fake", Select: func(string) (string, []string, error) {
		selectCalls++
		return "/test/interpreter", nil, nil
	}})
	t.Cleanup(func() { delete(core.Registry, id) })
	ast := &langBlockExpr{block: &LangBlock{ID: id, Source: "source", Tag: "test"}}
	if err := preflightValidator(core.WithAutoInstallDisabled(t.Context()), ast, nil); err != nil {
		t.Fatalf("preflightValidator: %v", err)
	}
	if selectCalls != 1 {
		t.Fatalf("Select calls = %d; want 1", selectCalls)
	}
}

func TestPreflightValidatorReturnsMissingReportWithoutInstalling(t *testing.T) {
	const id = "preflight-validator-missing-test"
	core.Register(core.Language{ID: id, Ext: ".fake", Select: func(string) (string, []string, error) { return "", nil, core.ErrNotFound }})
	t.Cleanup(func() { delete(core.Registry, id) })
	ast := &langBlockExpr{block: &LangBlock{ID: id}}
	err := preflightValidator(core.WithAutoInstallDisabled(t.Context()), ast, nil)
	var report core.Report
	if !errors.As(err, &report) || report.OK() {
		t.Fatalf("preflight error = %T %v; want a missing-language Report", err, err)
	}
	if report.SelfTestSkipReason() == "" {
		t.Fatal("missing interpreter should be classified as a safe self-test skip")
	}
}

func TestPreflightValidatorIsDisabledForSyntaxOnlyCompilation(t *testing.T) {
	const id = "preflight-validator-syntax-only-test"
	selectCalls := 0
	core.Register(core.Language{ID: id, Ext: ".fake", Select: func(string) (string, []string, error) {
		selectCalls++
		return "", nil, core.ErrNotFound
	}})
	t.Cleanup(func() { delete(core.Registry, id) })
	ast := &langBlockExpr{block: &LangBlock{ID: id}}
	if err := preflightValidator(WithPreflightDisabled(t.Context()), ast, nil); err != nil {
		t.Fatalf("syntax-only validator: %v", err)
	}
	if selectCalls != 0 {
		t.Fatalf("syntax-only preflight called Select %d times", selectCalls)
	}
}
