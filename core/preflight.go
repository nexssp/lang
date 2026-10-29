package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nexssp/kernel/xerr"
)

// Preflight checks interpreter availability for a set of programs. It has no
// compiler or AST dependency; host adapters decide when and how to invoke it.
type Preflight struct {
	// Installer, if non-nil, is invoked for missing interpreters. A nil
	// Installer keeps preflight find-only.
	Installer Ensure
}

// Report is the result of a preflight pass. Zero Missing entries means every
// requested interpreter is available (or was installed successfully).
//
//nolint:errname // Report is a result type that also implements error.
type Report struct {
	Checked []Check
	Missing []Check
}

// Check is one language examined during preflight.
type Check struct {
	ID      string
	Version string
	Tag     string
	Cmd     string
	Err     error
}

// OK reports whether every requested language was resolved.
func (r Report) OK() bool { return len(r.Missing) == 0 }

// SelfTestSkipReason returns a skip reason only when missing interpreter
// tooling is the sole failure; unknown language/configuration errors remain
// failures.
func (r Report) SelfTestSkipReason() string {
	if r.OK() {
		return ""
	}
	for _, check := range r.Missing {
		if errors.Is(check.Err, ErrNotFound) {
			continue
		}
		if _, ok := errors.AsType[*NotInstalledError](check.Err); ok {
			continue
		}
		return ""
	}
	return r.Error()
}

// Error formats a concise report suitable for wrapping in a host diagnostic.
func (r Report) Error() string {
	if r.OK() {
		return ""
	}
	var b strings.Builder
	b.WriteString("lang preflight: missing interpreters — install before running:\n")
	for _, check := range r.Missing {
		if check.Version != "" {
			fmt.Fprintf(&b, "  lang.%s@%s", check.ID, check.Version)
		} else {
			fmt.Fprintf(&b, "  lang.%s", check.ID)
		}
		if check.Tag != "" {
			fmt.Fprintf(&b, " (keyword %s)", check.Tag)
		}
		var notInstalled *NotInstalledError
		if errors.As(check.Err, &notInstalled) && notInstalled.Cause != nil {
			fmt.Fprintf(&b, " — %s", notInstalled.Cause)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Run checks each distinct (ID, Version) pair in programs, sorting results for
// deterministic diagnostics. When configured, Installer handles missing
// interpreters before the report is returned.
func (p Preflight) Run(programs []Program) Report {
	return p.RunContext(context.Background(), programs)
}

// RunContext checks interpreter availability using the caller's context.
func (p Preflight) RunContext(ctx context.Context, programs []Program) Report {
	seen := make(map[string]*Check, len(programs))
	for i := range programs {
		program := &programs[i]
		key := registryKey(program.ID, program.Version)
		if _, ok := seen[key]; ok {
			continue
		}
		check := &Check{ID: program.ID, Version: program.Version, Tag: program.Tag}
		language, err := Lookup(program.ID, program.Version)
		if err != nil {
			check.Err = err
			seen[key] = check
			continue
		}
		if cmd, _, handled, depErr := resolveDepsCmd(language, program.Source, "<source>"); handled {
			check.Cmd, check.Err = cmd, depErr
			seen[key] = check
			continue
		}
		switch {
		case language.SelectContext != nil:
			check.Cmd, _, check.Err = language.SelectContext(ctx, program.Version)
		case language.Select != nil:
			check.Cmd, _, check.Err = language.Select(program.Version)
		default:
			check.Err = &NotInstalledError{ID: language.ID, Version: program.Version, Cause: xerr.Internal("runner has no command selector")}
		}
		seen[key] = check
		if check.Err != nil && p.Installer != nil && ctx != nil && ctx.Err() == nil {
			if cmd, _, installErr := p.Installer.Ensure(language, program.Version); installErr == nil {
				check.Cmd, check.Err = cmd, nil
			} else {
				check.Err = installErr
			}
		}
	}

	out := Report{}
	for _, check := range seen {
		out.Checked = append(out.Checked, *check)
		if check.Err != nil {
			out.Missing = append(out.Missing, *check)
		}
	}
	sortChecks := func(checks []Check) {
		sort.Slice(checks, func(i, j int) bool {
			if checks[i].ID != checks[j].ID {
				return checks[i].ID < checks[j].ID
			}
			return checks[i].Version < checks[j].Version
		})
	}
	sortChecks(out.Checked)
	sortChecks(out.Missing)
	return out
}
