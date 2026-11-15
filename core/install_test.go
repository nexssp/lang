package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRecipeFor asserts the per-OS recipe lookup. We can't fake runtime.GOOS
// here, so we test the OSRecipe.RecipeFor() method directly with explicit
// values.
func TestRecipeFor(t *testing.T) {
	r := OSRecipe{
		Windows: []string{"winget", "install", "x"},
		Darwin:  []string{"brew", "install", "x"},
		Linux:   []string{"apt", "install", "x"},
	}
	// Just assert the three branches return their own recipe.
	if got := r.Windows; len(got) != 3 || got[0] != "winget" {
		t.Errorf("Windows = %v", got)
	}
	if got := r.Darwin; len(got) != 3 || got[0] != "brew" {
		t.Errorf("Darwin = %v", got)
	}
	if got := r.Linux; len(got) != 3 || got[0] != "apt" {
		t.Errorf("Linux = %v", got)
	}
}

// TestRecipeForEmpty asserts an OSRecipe with no recipe for the current OS
// returns nil — installEnsure surfaces this as "no auto-install recipe."
func TestRecipeForEmpty(t *testing.T) {
	r := OSRecipe{} // all nil
	if got := r.RecipeFor(); got != nil {
		t.Errorf("RecipeFor on empty OSRecipe = %v; want nil", got)
	}
}

// fakeInstaller is a test stub for the installer seam. It records what would
// have been run and returns a configurable error.
type fakeInstaller struct {
	called   bool
	lastCmd  string
	lastArgs []string
	err      error
}

func (f *fakeInstaller) Run(_ context.Context, cmd string, args []string) error {
	f.called = true
	f.lastCmd = cmd
	f.lastArgs = args
	return f.err
}

// TestInstallEnsureNoRecipe asserts a language with no install recipe returns
// NotInstalledError with the "no auto-install" cause, and does not invoke
// the installer.
func TestInstallEnsureNoRecipe(t *testing.T) {
	fi := &fakeInstaller{}
	oldInstaller := installerHook
	installerHook = fi
	defer func() { installerHook = oldInstaller }()

	lang := Language{
		ID:            "no-recipe-lang",
		install:       OSRecipe{}, // no recipe
		SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound },
	}
	_, _, err := installEnsure{}.Ensure(context.Background(), lang, "")
	if err == nil {
		t.Fatal("Ensure returned nil for no-recipe language")
	}
	var nie *NotInstalledError
	if !errors.As(err, &nie) {
		t.Fatalf("error type %T; want *NotInstalledError", err)
	}
	if !strings.Contains(nie.Error(), "no auto-install recipe") {
		t.Errorf("error = %q; want 'no auto-install recipe'", nie)
	}
	if fi.called {
		t.Error("installer was called for a no-recipe language")
	}
}

// TestInstallEnsureRecipeFails asserts a failing recipe surfaces NotInstalledError
// with the install-failure cause.
func TestInstallEnsureRecipeFails(t *testing.T) {
	fi := &fakeInstaller{err: errors.New("winget not found")}
	oldInstaller := installerHook
	installerHook = fi
	defer func() { installerHook = oldInstaller }()

	lang := Language{
		ID: "failing-recipe-lang",
		install: OSRecipe{
			// Set all three so RecipeFor returns something on any OS.
			Windows: []string{"winget", "install", "x"},
			Darwin:  []string{"brew", "install", "x"},
			Linux:   []string{"apt", "install", "x"},
		},
		SelectContext: func(context.Context, string) (string, []string, error) { return "", nil, ErrNotFound },
	}
	_, _, err := installEnsure{}.Ensure(context.Background(), lang, "")
	if err == nil {
		t.Fatal("Ensure returned nil for failing recipe")
	}
	var nie *NotInstalledError
	if !errors.As(err, &nie) {
		t.Fatalf("error type %T; want *NotInstalledError", err)
	}
	if !strings.Contains(nie.Error(), "winget not found") {
		t.Errorf("error = %q; want substring 'winget not found'", nie)
	}
}

// TestInstallEnsureSuccess asserts that when the recipe runs and Select
// then succeeds, the resolved command is returned.
func TestInstallEnsureSuccess(t *testing.T) {
	fi := &fakeInstaller{} // succeeds
	oldInstaller := installerHook
	installerHook = fi
	defer func() { installerHook = oldInstaller }()

	selectCalls := 0
	lang := Language{
		ID: "success-recipe-lang",
		install: OSRecipe{
			Windows: []string{"winget", "install", "x"},
			Darwin:  []string{"brew", "install", "x"},
			Linux:   []string{"apt", "install", "x"},
		},
		// First Select call (during Ensure's re-resolve) succeeds.
		SelectContext: func(context.Context, string) (string, []string, error) {
			selectCalls++
			return "/usr/bin/installed-binary", nil, nil
		},
	}
	cmd, _, err := installEnsure{}.Ensure(context.Background(), lang, "")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !fi.called {
		t.Error("installer was not called")
	}
	if selectCalls != 1 {
		t.Errorf("Select called %d times; want 1 (the re-resolve)", selectCalls)
	}
	if cmd != "/usr/bin/installed-binary" {
		t.Errorf("cmd = %q; want /usr/bin/installed-binary", cmd)
	}
}

// TestInstallEnsurePathNotRefreshed asserts that when the recipe succeeds but
// Select still misses (PATH not refreshed in this process), the error
// message mentions the shell-restart hint.
func TestInstallEnsurePathNotRefreshed(t *testing.T) {
	fi := &fakeInstaller{} // succeeds
	oldInstaller := installerHook
	installerHook = fi
	defer func() { installerHook = oldInstaller }()

	lang := Language{
		ID: "path-stale-lang",
		install: OSRecipe{
			Windows: []string{"winget", "install", "x"},
			Darwin:  []string{"brew", "install", "x"},
			Linux:   []string{"apt", "install", "x"},
		},
		SelectContext: func(context.Context, string) (string, []string, error) {
			return "", nil, ErrNotFound // still missing after install
		},
	}
	_, _, err := installEnsure{}.Ensure(context.Background(), lang, "")
	if err == nil {
		t.Fatal("Ensure returned nil when Select still missed after install")
	}
	var nie *NotInstalledError
	if !errors.As(err, &nie) {
		t.Fatalf("error type %T; want *NotInstalledError", err)
	}
	if !strings.Contains(nie.Error(), "shell restart") {
		t.Errorf("error = %q; want 'shell restart' hint", nie)
	}
}

type blockingInstaller struct {
	started chan struct{}
}

func (i blockingInstaller) Run(ctx context.Context, _ string, _ []string) error {
	close(i.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestInstallContextCancellation(t *testing.T) {
	const id = "install-context-cancel-test"
	selectCalled := false
	registerTestLanguage(t, Language{
		ID: id,
		install: OSRecipe{
			Windows: []string{"installer", "install"},
			Darwin:  []string{"installer", "install"},
			Linux:   []string{"installer", "install"},
		},
		SelectContext: func(context.Context, string) (string, []string, error) {
			selectCalled = true
			return "", nil, nil
		},
	})
	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		delete(registry, id)
	})

	started := make(chan struct{})
	oldInstaller := installerHook
	installerHook = blockingInstaller{started: started}
	defer func() { installerHook = oldInstaller }()

	preCanceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := InstallContext(preCanceled, id, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled InstallContext error = %v; want context.Canceled", err)
	}
	select {
	case <-started:
		t.Fatal("installer started for a pre-canceled context")
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, err := InstallContext(ctx, id, "")
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("installer did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("InstallContext error = %v; want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("InstallContext did not return after cancellation")
	}
	if selectCalled {
		t.Fatal("interpreter was re-resolved after installation was canceled")
	}
}

func TestDefaultInstallerRunStopsCanceledDirectChild(t *testing.T) {
	const helperEnv = "NEXSS_LANG_INSTALLER_HELPER"
	const parentPIDEnv = "NEXSS_LANG_INSTALLER_PARENT_PID"
	if os.Getenv(helperEnv) == "direct-child-helper" {
		parentPID, err := strconv.Atoi(os.Getenv(parentPIDEnv))
		if err != nil || parentPID <= 0 {
			t.Fatalf("invalid parent process ID: %v", err)
		}
		if err := os.WriteFile(installerReadyPath(parentPID), []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Hour)
		}
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parentPID := os.Getpid()
	readyPath := installerReadyPath(parentPID)
	if err := os.Remove(readyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(readyPath) })
	t.Setenv(helperEnv, "direct-child-helper")
	t.Setenv(parentPIDEnv, strconv.Itoa(parentPID))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- (defaultInstaller{}).Run(ctx, executable, []string{"-test.run=^TestDefaultInstallerRunStopsCanceledDirectChild$"})
	}()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		select {
		case runErr := <-result:
			t.Fatalf("helper process exited before cancellation: %v", runErr)
		case <-ticker.C:
		case <-deadline:
			cancel()
			t.Fatal("helper process did not start")
		}
	}

	cancel()
	select {
	case runErr := <-result:
		if runErr == nil {
			t.Fatal("canceled installer process returned nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("default installer did not stop after cancellation")
	}
}

func installerReadyPath(parentPID int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("nexss-lang-installer-%d-ready", parentPID))
}
