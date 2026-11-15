package core

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/nexssp/kernel/xerr"
)

// Language is the runtime view of a runner spec. One per (language, version).
// Tags are gone by here — runner file aliases are applied at parse time,
// and from this struct onward every code path speaks the canonical ID
// ("python"), never the tag ("py").
//
// One struct, every language. New languages add a runners/<id>.nflow file,
// zero Go changes. See runners.go.
type Language struct {
	// ID is the canonical language identifier, e.g. "python".
	ID string

	// Ext is the file extension used when materializing source to disk,
	// including the leading dot, e.g. ".py".
	Ext string

	// SelectContext resolves a version while honoring the caller's context.
	// Runtime and preflight resolution always pass their caller's context.
	SelectContext func(ctx context.Context, version string) (cmd string, args []string, err error)

	// Encode serializes the incoming stream value to bytes for stdin.
	// JSON is the default; shell/raw languages use text.
	Encode func(v any) ([]byte, error)

	// Decode parses stdout back into a typed value the next atom sees.
	// Takes string (not []byte) to avoid the []byte(string) copy from
	// external.ExecResult.Stdout. JSON is the default; shell/raw return string.
	Decode func(s string) (any, error)

	// spec is the parsed RunnerSpec. Used by Ensure for install recipes
	// and by preflight for reporting. Nil for languages registered via
	// RegisterDefine (@lang.define data model) — those have no install
	// recipe and rely on the user to install the interpreter.
	spec *RunnerSpec

	// install is the per-OS install recipe. Empty OSRecipe means
	// "no auto-install path; surface NotInstalledError with guidance."
	install OSRecipe
}

// registry is the process-global table of known languages. Values and their
// private runner data are copied at registration and lookup boundaries.
var (
	registry   = make(map[string]Language)
	registryMu sync.RWMutex
)

// registryKey returns the map key for an (ID, Version) pair.
// "python" with version "" -> "python"
// "python" with version "3" -> "python@3"
func registryKey(id, version string) string {
	if version == "" {
		return id
	}
	return id + "@" + version
}

// Register adds a language to the process registry. It returns a Kernel
// Validation error for an empty ID and a Conflict error for a duplicate
// (ID, Version). The language value is copied; its SelectContext and codec
// callbacks may be called concurrently and must be safe for concurrent use.
// SelectContext must honor ctx.
func Register(l Language) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	return registerLanguageLocked(l)
}

// Unregister removes an exact language version and all of its aliases.
// Previously returned Language values remain valid snapshots. It returns
// false when the language is not registered.
func Unregister(id, version string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	key := registryKey(id, version)
	if _, ok := registry[key]; !ok {
		return false
	}
	delete(registry, key)
	kept := aliases[:0]
	for _, alias := range aliases {
		if alias.ID != id || alias.Version != version {
			kept = append(kept, alias)
		}
	}
	aliases = kept
	return true
}

// registerLanguageLocked adds a language while registryMu is held.
func registerLanguageLocked(l Language) error {
	if l.ID == "" {
		return xerr.Validation("lang: Language.ID must be non-empty")
	}
	key := registryKey(l.ID, l.Version())
	if _, dup := registry[key]; dup {
		return xerr.Conflict("lang: language " + key + " already registered")
	}
	registry[key] = cloneLanguage(l)
	return nil
}

// Lookup returns a defensive value copy for an exact (ID, Version) pair.
// Mutating the returned Language or any data returned by Spec cannot mutate
// the process registry. Function callbacks are shared function values and
// must themselves be safe for concurrent use.
func Lookup(id, version string) (Language, error) {
	registryMu.RLock()
	l, ok := registry[registryKey(id, version)]
	if ok {
		l = cloneLanguage(l)
	}
	registryMu.RUnlock()
	if !ok {
		return Language{}, &UnknownLanguageError{ID: id, Version: version}
	}
	return l, nil
}

// RegisteredLanguages returns stable, sorted defensive copies of all
// registered languages. It replaces direct iteration over the old exported
// mutable registry state.
func RegisteredLanguages() []Language {
	registryMu.RLock()
	out := make([]Language, 0, len(registry))
	for key := range registry {
		out = append(out, cloneLanguage(registry[key]))
	}
	registryMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version() < out[j].Version()
	})
	return out
}

func cloneLanguage(language Language) Language {
	language.spec = cloneRunnerSpec(language.spec)
	language.install.Windows = append([]string(nil), language.install.Windows...)
	language.install.Darwin = append([]string(nil), language.install.Darwin...)
	language.install.Linux = append([]string(nil), language.install.Linux...)
	return language
}

// UnknownLanguageError is returned by Lookup and ParseTag when the alias
// resolves to an (ID, Version) pair the process registry has no entry for. It
// surfaces as a compile-time SourceError via the parser hook.
type UnknownLanguageError struct {
	ID, Version string
}

func (e *UnknownLanguageError) Error() string {
	if e.Version == "" {
		return "unknown language: " + e.ID
	}
	return "unknown language: " + e.ID + "@" + e.Version
}

func (e *UnknownLanguageError) Unwrap() error { return xerr.NotFound(e.Error()) }

// Version returns the language's version string. Reads from spec.Version
// when a spec is attached (the runner-file path); "" otherwise (the
// @lang.define path stores version on the spec too via RegisterDefine).
func (l *Language) Version() string {
	if l.spec != nil {
		return l.spec.Version
	}
	return ""
}

// Spec returns a copy of the language's runner profile. Languages registered
// directly without a RunnerSpec return the zero value and false.
func (l *Language) Spec() (RunnerSpec, bool) {
	if l == nil || l.spec == nil {
		return RunnerSpec{}, false
	}
	return *cloneRunnerSpec(l.spec), true
}

// --- shared codecs -------------------------------------------------------

// encodeJSON is the default stdin serializer. nil input becomes empty stdin,
// not the bytes "null", because most languages treat EOF on stdin as "no
// input" rather than "JSON null".
func encodeJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// decodeJSON parses stdout as JSON. Empty stdout becomes nil rather than an
// EOF error — pipelines frequently end in a side-effecting print with no
// return value.
//
// Uses json.NewDecoder on a strings.Reader to avoid the []byte(string)
// copy that json.Unmarshal would require.
func decodeJSON(s string) (any, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // Empty output intentionally decodes to a nil value.
	}
	var v any
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// encodeText is used by shell and "raw" codecs. Strings and byte slices pass
// through; structured values are JSON-encoded as a fallback so a flow that
// passes {"x": 1} into bash still produces parseable input.
func encodeText(v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(t), nil
	case []byte:
		return t, nil
	default:
		return json.Marshal(v)
	}
}

// decodeText returns the raw stdout as a string. Used by shell and raw codecs.
// No allocation beyond the string header — stdout is already a string from
// external.ExecResult.
func decodeText(s string) (any, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // Empty output intentionally decodes to a nil value.
	}
	return s, nil
}
