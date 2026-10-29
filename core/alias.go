package core

import "sort"

// Alias maps a runner keyword (for example, "py") to a canonical language ID
// and version. Aliases are derived from runner profiles, not hand-maintained.
type Alias struct {
	Tag     string
	ID      string
	Version string
}

var aliases []Alias

func registerAlias(tag, id, version string) {
	if tag == "" {
		return
	}
	for _, alias := range aliases {
		if alias.Tag == tag {
			panic("lang: alias @" + tag + " claimed by both " + alias.ID + " and " + id)
		}
	}
	aliases = append(aliases, Alias{Tag: tag, ID: id, Version: version})
}

// ResolveAlias maps a runner keyword to its canonical language ID and version.
// The returned ok is false for an unknown alias.
func ResolveAlias(tag string) (id, version string, ok bool) {
	if tag == "" {
		return "", "", false
	}
	for _, alias := range aliases {
		if alias.Tag == tag {
			return alias.ID, alias.Version, true
		}
	}
	return "", "", false
}

// Aliases returns a sorted copy of the aliases currently registered.
func Aliases() []Alias {
	out := append([]Alias(nil), aliases...)
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

// resolveAlias and allAliases are retained as package-local helpers for the
// core's validation tests.
func resolveAlias(tag string) (id, version string, ok bool) { return ResolveAlias(tag) }
func allAliases() []Alias                                   { return Aliases() }
