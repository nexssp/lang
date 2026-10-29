// Package core provides the shared language runner runtime: profile loading,
// alias resolution, interpreter discovery and installation, source
// materialization, and byte-oriented execution. Language-specific profiles and
// templates are owned by their respective language packages and composed into
// the public RunnerFiles() view used by runtime integrations.
package core
