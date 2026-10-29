package core

import (
	"fmt"

	"github.com/nexssp/kernel/xerr"
)

// Define is the in-flow @lang.define directive's data model.
//
// It lets users register a language the bundle doesn't ship built-in,
// without forking the repo or writing a runner file. Example DSL:
//
//	@lang.define "gleam" {
//	  ext: ".gleam"
//	  aliases: ["gleam"]
//	  find: ["gleam run"]
//	  stdin: "json"
//	  stdout: "text"
//	}
//	{ "value": "x" } -> gleam"..."
//
// This is the runtime equivalent of a runners/*.nflow file — same fields,
// same RunnerSpec underneath. Definitions update the process-wide registry and
// remain registered for the process lifetime; each (ID, Version) and alias
// must be declared only once.
type Define struct {
	ID           string   // required, e.g. "gleam"
	Version      string   // optional, e.g. "3"
	Ext          string   // required, including dot, e.g. ".gleam"
	Aliases      []string // optional, e.g. ["gleam", "beam"]
	Find         []string // required, e.g. ["gleam", "run"]
	Stdin        string   // "json" (default) | "text" | "raw"
	Stdout       string   // "json" (default) | "text" | "raw"
	FileTemplate string   // optional JSON-in/JSON-out source for .file :autocreate
	Install      OSRecipe // optional, per-OS install commands
	Deps         string   // optional command template containing {file}
	DepsMarker   string   // optional source marker that selects Deps
}

// RegisterDefine adds a user-defined language to the Registry.
//
// Called by the @lang.define directive. Go hosts may also register a language
// directly before compiling flows.
//
// Returns an error rather than panicking: user-defined languages fail
// gracefully, while builtins (which the developer shipped intentionally)
// panic on duplicate registration via Register.
func RegisterDefine(d Define) error {
	if d.ID == "" {
		return xerr.Validation("@lang.define: id is required")
	}
	if d.Ext == "" {
		return xerr.Validation(fmt.Sprintf("@lang.define %s: ext is required", d.ID))
	}
	if len(d.Find) == 0 {
		return xerr.Validation(fmt.Sprintf("@lang.define %s: find is required", d.ID))
	}
	if _, dup := Registry[registryKey(d.ID, d.Version)]; dup {
		return xerr.Conflict(fmt.Sprintf("@lang.define %s: language already registered", d.ID))
	}

	// Build a RunnerSpec from Define so the runtime path is unified —
	// there's exactly one Language construction path, whether the source
	// is a runner file or an @lang.define block.
	spec := &RunnerSpec{
		ID:           d.ID,
		Version:      d.Version,
		Ext:          d.Ext,
		Aliases:      d.Aliases,
		Find:         d.Find,
		Stdin:        d.Stdin,
		Stdout:       d.Stdout,
		FileTemplate: d.FileTemplate,
		Install:      d.Install,
		Deps:         d.Deps,
		DepsMarker:   d.DepsMarker,
	}
	return registerSpec(spec)
}
