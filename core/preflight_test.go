package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPreflightGroupsAndDeduplicatesMissingInterpreters(t *testing.T) {
	const id = "preflight-fake-lang"
	Register(Language{ID: id, Ext: ".fake", Select: func(string) (string, []string, error) { return "", nil, ErrNotFound }})
	t.Cleanup(func() { delete(Registry, id) })
	report := (Preflight{}).Run([]Program{{ID: id}, {ID: id}, {ID: id, Version: "v2"}})
	if report.OK() || len(report.Missing) != 2 {
		t.Fatalf("report = %+v; want two deduplicated misses", report)
	}
}

func TestPreflightReportsUnknownLanguage(t *testing.T) {
	report := (Preflight{}).Run([]Program{{ID: "totally-unknown-lang"}})
	if report.OK() || len(report.Missing) != 1 || report.Missing[0].ID != "totally-unknown-lang" {
		t.Fatalf("report = %+v", report)
	}
}

func TestPreflightUsesInjectedInstaller(t *testing.T) {
	const id = "preflight-autoinstall-lang"
	Register(Language{ID: id, Ext: ".fake", Select: func(string) (string, []string, error) { return "", nil, ErrNotFound }})
	t.Cleanup(func() { delete(Registry, id) })
	installCalls := 0
	preflight := Preflight{Installer: EnsureFunc(func(language *Language, _ string) (string, []string, error) {
		installCalls++
		return "/installed/" + language.ID, nil, nil
	})}
	report := preflight.Run([]Program{{ID: id}})
	if !report.OK() || installCalls != 1 || report.Checked[0].Cmd != "/installed/"+id {
		t.Fatalf("report=%+v installCalls=%d", report, installCalls)
	}
}

func TestPreflightRunContextCancellationSkipsInstaller(t *testing.T) {
	const id = "preflight-canceled-lang"
	Register(Language{ID: id, Ext: ".fake", SelectContext: func(ctx context.Context, _ string) (string, []string, error) {
		return "", nil, ctx.Err()
	}})
	t.Cleanup(func() { delete(Registry, id) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	installCalls := 0
	preflight := Preflight{Installer: EnsureFunc(func(*Language, string) (string, []string, error) {
		installCalls++
		return "/installed", nil, nil
	})}
	report := preflight.RunContext(ctx, []Program{{ID: id}})
	if report.OK() || installCalls != 0 || len(report.Missing) != 1 || !errors.Is(report.Missing[0].Err, context.Canceled) {
		t.Fatalf("report=%+v installCalls=%d; want canceled probe without install", report, installCalls)
	}
}

func TestPreflightReportFormattingAndSorting(t *testing.T) {
	report := Report{Missing: []Check{
		{ID: "python", Version: "3", Tag: "py3", Err: &NotInstalledError{ID: "python", Version: "3", Cause: errors.New("tool unavailable")}},
		{ID: "rust", Tag: "rs", Err: ErrNotFound},
	}}
	text := report.Error()
	for _, fragment := range []string{"lang.python@3", "keyword py3", "tool unavailable", "lang.rust", "keyword rs"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("Error() = %q; missing %q", text, fragment)
		}
	}
	if (Report{}).Error() != "" {
		t.Fatal("empty report should format as empty string")
	}
	if reason := report.SelfTestSkipReason(); reason == "" {
		t.Fatal("missing-interpreter report should return a self-test skip reason")
	}
	unknown := Report{Missing: []Check{{ID: "unknown", Err: &UnknownLanguageError{ID: "unknown"}}}}
	if unknown.SelfTestSkipReason() != "" {
		t.Fatal("unknown language must not be hidden as a self-test skip")
	}
}

func TestPreflightResultsAreSorted(t *testing.T) {
	report := (Preflight{}).Run([]Program{{ID: "go"}, {ID: "python", Version: "3"}, {ID: "python"}, {ID: "rust"}})
	for i := 1; i < len(report.Checked); i++ {
		previous, current := report.Checked[i-1], report.Checked[i]
		if previous.ID > current.ID || previous.ID == current.ID && previous.Version > current.Version {
			t.Fatalf("results are not sorted: %+v before %+v", previous, current)
		}
	}
}
