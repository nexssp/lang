package nexssflow

import (
	"context"
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
	id := uniqueTestLanguageID(t)
	selectCalls := 0
	registerCoreTestLanguage(t, core.Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) {
		selectCalls++
		return "/test/interpreter", nil, nil
	}})
	ast := &langBlockExpr{block: &LangBlock{ID: id, Source: "source", Tag: "test"}}
	if err := preflightValidator(core.WithAutoInstallDisabled(t.Context()), ast, nil); err != nil {
		t.Fatalf("preflightValidator: %v", err)
	}
	if selectCalls != 1 {
		t.Fatalf("Select calls = %d; want 1", selectCalls)
	}
}

func TestPreflightValidatorReturnsMissingReportWithoutInstalling(t *testing.T) {
	id := uniqueTestLanguageID(t)
	registerCoreTestLanguage(t, core.Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, core.ErrNotFound }})
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
	id := uniqueTestLanguageID(t)
	selectCalls := 0
	registerCoreTestLanguage(t, core.Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) {
		selectCalls++
		return "", nil, core.ErrNotFound
	}})
	ast := &langBlockExpr{block: &LangBlock{ID: id}}
	if err := preflightValidator(WithPreflightDisabled(t.Context()), ast, nil); err != nil {
		t.Fatalf("syntax-only validator: %v", err)
	}
	if selectCalls != 0 {
		t.Fatalf("syntax-only preflight called Select %d times", selectCalls)
	}
}
