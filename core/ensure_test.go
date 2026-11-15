package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestResolveHit asserts Resolve returns the command when Select succeeds.
// No call to Ensure should happen.
func TestResolveHit(t *testing.T) {
	called := false
	lang := Language{
		ID: "test-resolve-hit",
		SelectContext: func(context.Context, string) (string, []string, error) {
			called = true
			return "/usr/bin/fake", nil, nil
		},
	}
	oldHook := ensureHook
	ensureHook = failingEnsure{t: t}
	defer func() { ensureHook = oldHook }()

	cmd, _, err := Resolve(lang, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !called {
		t.Error("Select was not called")
	}
	if cmd != "/usr/bin/fake" {
		t.Errorf("cmd = %q; want /usr/bin/fake", cmd)
	}
}

// TestResolveMissCallsEnsure asserts Resolve falls through to Ensure on miss.
func TestResolveMissCallsEnsure(t *testing.T) {
	lang := Language{
		ID: "test-resolve-miss",
		SelectContext: func(context.Context, string) (string, []string, error) {
			return "", nil, ErrNotFound
		},
	}
	ensureCalled := false
	oldHook := ensureHook
	ensureHook = stubEnsure{called: &ensureCalled, cmd: "/usr/bin/installed", args: nil}
	defer func() { ensureHook = oldHook }()

	cmd, _, err := Resolve(lang, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ensureCalled {
		t.Error("Ensure was not called on Select miss")
	}
	if cmd != "/usr/bin/installed" {
		t.Errorf("cmd = %q; want /usr/bin/installed", cmd)
	}
}

// TestResolveMissReturnsError asserts Resolve returns NotInstalledError
// when both Select and Ensure fail.
func TestResolveMissReturnsError(t *testing.T) {
	lang := Language{
		ID: "test-resolve-error",
		SelectContext: func(context.Context, string) (string, []string, error) {
			return "", nil, ErrNotFound
		},
	}
	oldHook := ensureHook
	ensureHook = noEnsure{}
	defer func() { ensureHook = oldHook }()

	_, _, err := Resolve(lang, "3")
	if err == nil {
		t.Fatal("Resolve returned nil error for missing interpreter")
	}
	var nie *NotInstalledError
	if !errors.As(err, &nie) {
		t.Fatalf("Resolve error type %T; want *NotInstalledError", err)
	}
	if nie.ID != "test-resolve-error" || nie.Version != "3" {
		t.Errorf("NotInstalledError = %+v; want ID=test-resolve-error Version=3", nie)
	}
}

// TestNotInstalledErrorMessage asserts the error message includes ID and
// version, and the optional Cause.
func TestNotInstalledErrorMessage(t *testing.T) {
	t.Run("no cause", func(t *testing.T) {
		e := &NotInstalledError{ID: "python", Version: "3"}
		got := e.Error()
		want := "lang.python@3: interpreter not found on PATH"
		if got != want {
			t.Errorf("Error() = %q; want %q", got, want)
		}
	})
	t.Run("with cause", func(t *testing.T) {
		e := &NotInstalledError{
			ID: "rust", Version: "",
			Cause: errors.New("recipe failed"),
		}
		got := e.Error()
		if want := "lang.rust: recipe failed"; got != want {
			t.Errorf("Error() = %q; want %q", got, want)
		}
	})
}

func TestIsWindowsStorePythonAlias(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Users\mapo\AppData\Local\Microsoft\WindowsApps\python3.exe`, true},
		{`C:\Users\mapo\AppData\Local\Microsoft\WindowsApps\python.exe`, true},
		{`C:\Users\mapo\AppData\Local\Microsoft\WindowsApps\go.exe`, false},
		{`C:\Python312\python.exe`, false},
	}
	for _, tc := range cases {
		if got := isWindowsStorePythonAlias(tc.path); got != tc.want {
			t.Errorf("isWindowsStorePythonAlias(%q) = %t; want %t", tc.path, got, tc.want)
		}
	}
}

// --- helpers -------------------------------------------------------------

type failingEnsure struct{ t *testing.T }

func (f failingEnsure) Ensure(context.Context, Language, string) (cmd string, args []string, err error) {
	f.t.Fatal("Ensure should not be called when Select succeeds")
	return "", nil, nil
}

type stubEnsure struct {
	called *bool
	cmd    string
	args   []string
}

func (s stubEnsure) Ensure(context.Context, Language, string) (cmd string, args []string, err error) {
	*s.called = true
	return s.cmd, s.args, nil
}

func TestResolveContextCanceledBeforeSelectionSkipsEnsure(t *testing.T) {
	lang := Language{
		ID:            "canceled-resolve-test",
		SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound },
	}
	ensureCalled := false
	oldHook := ensureHook
	ensureHook = stubEnsure{called: &ensureCalled}
	defer func() { ensureHook = oldHook }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := ResolveContext(ctx, lang, "")
	if !errors.Is(err, context.Canceled) || ensureCalled {
		t.Fatalf("ResolveContext error=%v ensureCalled=%t; want cancellation without install", err, ensureCalled)
	}
}

func TestResolveContextCancelsInFlightSelector(t *testing.T) {
	started := make(chan struct{})
	ensureCalled := false
	oldHook := ensureHook
	ensureHook = stubEnsure{called: &ensureCalled}
	defer func() { ensureHook = oldHook }()
	lang := Language{ID: "inflight-selector-test", SelectContext: func(ctx context.Context, _ string) (string, []string, error) {
		close(started)
		<-ctx.Done()
		return "", nil, ctx.Err()
	}}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, err := ResolveContext(ctx, lang, "")
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("context-aware selector did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || ensureCalled {
			t.Fatalf("ResolveContext error=%v ensureCalled=%t; want canceled selector without install", err, ensureCalled)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ResolveContext did not return after selector cancellation")
	}
}

func TestResolveContextCancelsInFlightInstaller(t *testing.T) {
	lang := Language{
		ID:            "inflight-resolve-test",
		SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound },
	}
	started := make(chan struct{})
	oldHook := ensureHook
	ensureHook = EnsureFunc(func(ctx context.Context, _ Language, _ string) (string, []string, error) {
		close(started)
		<-ctx.Done()
		return "", nil, ctx.Err()
	})
	defer func() { ensureHook = oldHook }()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, err := ResolveContext(ctx, lang, "")
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("context-aware installer did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ResolveContext error = %v; want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ResolveContext did not return after cancellation")
	}
}
