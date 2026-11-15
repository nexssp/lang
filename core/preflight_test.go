package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPreflightGroupsAndDeduplicatesMissingInterpreters(t *testing.T) {
	const id = "preflight-fake-lang"
	registerTestLanguage(t, Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound }})
	t.Cleanup(func() { delete(registry, id) })
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
	registerTestLanguage(t, Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound }})
	t.Cleanup(func() { delete(registry, id) })
	installCalls := 0
	preflight := Preflight{Installer: EnsureFunc(func(ctx context.Context, language Language, _ string) (string, []string, error) {
		if ctx == nil {
			t.Fatal("Run supplied a nil installer context")
		}
		installCalls++
		id := language.ID
		language.ID = "mutated-installer-copy"
		return "/installed/" + id, nil, nil
	})}
	report := preflight.Run([]Program{{ID: id}})
	if !report.OK() || installCalls != 1 || report.Checked[0].Cmd != "/installed/"+id {
		t.Fatalf("report=%+v installCalls=%d", report, installCalls)
	}
	stored, err := Lookup(id, "")
	if err != nil || stored.ID != id {
		t.Fatalf("installer mutated the registered language: lang=%+v err=%v", stored, err)
	}
}

func TestPreflightRunContextCancellationSkipsInstaller(t *testing.T) {
	const id = "preflight-canceled-lang"
	registerTestLanguage(t, Language{ID: id, Ext: ".fake", SelectContext: func(ctx context.Context, _ string) (string, []string, error) {
		return "", nil, ctx.Err()
	}})
	t.Cleanup(func() { delete(registry, id) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	installCalls := 0
	preflight := Preflight{Installer: EnsureFunc(func(context.Context, Language, string) (string, []string, error) {
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

func TestPreflightRunContextCancelsInFlightInstaller(t *testing.T) {
	const id = "preflight-inflight-cancel-test"
	registerTestLanguage(t, Language{ID: id, Ext: ".fake", SelectContext: func(context.Context, string) (string, []string, error) {
		return "", nil, ErrNotFound
	}})
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		delete(registry, id)
	})

	started := make(chan struct{})
	preflight := Preflight{Installer: EnsureFunc(func(ctx context.Context, _ Language, _ string) (string, []string, error) {
		close(started)
		<-ctx.Done()
		return "", nil, ctx.Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan Report, 1)
	go func() {
		result <- preflight.RunContext(ctx, []Program{{ID: id}})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("preflight installer did not start")
	}
	cancel()
	select {
	case report := <-result:
		if report.OK() || len(report.Missing) != 1 || !errors.Is(report.Missing[0].Err, context.Canceled) {
			t.Fatalf("report = %+v; want a canceled preflight result", report)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("preflight did not return after cancellation")
	}
}

func TestPreflightRunContextCancelsInFlightSelector(t *testing.T) {
	const id = "preflight-inflight-selector-cancel-test"
	started := make(chan struct{})
	registerTestLanguage(t, Language{ID: id, Ext: ".fake", SelectContext: func(ctx context.Context, _ string) (string, []string, error) {
		close(started)
		<-ctx.Done()
		return "", nil, ctx.Err()
	}})
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		delete(registry, id)
	})

	installCalls := 0
	preflight := Preflight{Installer: EnsureFunc(func(context.Context, Language, string) (string, []string, error) {
		installCalls++
		return "/installed", nil, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan Report, 1)
	go func() {
		result <- preflight.RunContext(ctx, []Program{{ID: id}})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("context-aware selector did not start")
	}
	cancel()
	select {
	case report := <-result:
		if report.OK() || len(report.Missing) != 1 || !errors.Is(report.Missing[0].Err, context.Canceled) || installCalls != 0 {
			t.Fatalf("report=%+v installCalls=%d; want canceled selector without install", report, installCalls)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunContext did not return after selector cancellation")
	}
}
