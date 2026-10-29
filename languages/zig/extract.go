// Package zig provides source signature extraction for zig files.
package zig

import (
	"strings"
)

// parseZig extracts function, struct, and enum signatures.
// When exportedOnly=true, only public declarations (pub) are kept.
//
//nolint:gocyclo // line-oriented parser; every branch handles a distinct header form.
func parseZig(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	content := string(src)
	lines := strings.Split(content, "\n")
	var currentDecl strings.Builder
	inDecl := false
	var declKind string
	var isPub bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if !inDecl {
			// Check for function declaration
			if strings.HasPrefix(trimmed, "pub fn ") || strings.HasPrefix(trimmed, "fn ") {
				isPub = strings.HasPrefix(trimmed, "pub fn ")
				declKind = "fn"

				// Semicolon-only declaration (no body): e.g. "pub fn declared_only() void;"
				if strings.Contains(trimmed, ";") && !strings.Contains(trimmed, "{") {
					if !exportedOnly || isPub {
						snippets = append(snippets, trimmed)
						funcCount++
					}
					continue
				}

				inDecl = true
				currentDecl.WriteString(trimmed)
				if strings.Contains(trimmed, "{") {
					inDecl = false
					sig := currentDecl.String()
					if idx := strings.Index(sig, "{"); idx != -1 {
						sig = strings.TrimSpace(sig[:idx]) + " {}"
					}
					if !exportedOnly || isPub {
						snippets = append(snippets, sig)
						funcCount++
					}
					currentDecl.Reset()
				}
				continue
			}

			// Check for container declaration (struct, enum, union)
			isConstDecl := strings.HasPrefix(trimmed, "pub const ") || strings.HasPrefix(trimmed, "const ")
			isContainer := strings.Contains(trimmed, "struct") || strings.Contains(trimmed, "enum") || strings.Contains(trimmed, "union")

			if isConstDecl && isContainer {
				isPub = strings.HasPrefix(trimmed, "pub const ")
				declKind = "container"

				// Empty container on a single line: "pub const EmptyStruct = struct {};"
				if strings.Contains(trimmed, "{};") || strings.Contains(trimmed, "{}") {
					if !exportedOnly || isPub {
						snippets = append(snippets, trimmed)
					}
					continue
				}

				inDecl = true
				currentDecl.WriteString(trimmed)
				if strings.Contains(trimmed, "{") {
					inDecl = false
					sig := currentDecl.String()
					if idx := strings.Index(sig, "{"); idx != -1 {
						sig = strings.TrimSpace(sig[:idx]) + " {"
					}
					if !exportedOnly || isPub {
						snippets = append(snippets, sig)
					}
					currentDecl.Reset()
				}
				continue
			}
		} else {
			// In multi-line declaration header
			currentDecl.WriteString(" " + trimmed)
			if strings.Contains(trimmed, "{") {
				inDecl = false
				sig := currentDecl.String()
				if idx := strings.Index(sig, "{"); idx != -1 {
					if declKind == "fn" {
						sig = strings.TrimSpace(sig[:idx]) + " {}"
					} else {
						sig = strings.TrimSpace(sig[:idx]) + " {"
					}
				}
				if !exportedOnly || isPub {
					snippets = append(snippets, sig)
					if declKind == "fn" {
						funcCount++
					}
				}
				currentDecl.Reset()
			}
		}
	}

	return snippets, funcCount, nil
}

// ExtractSignatures parses Zig source and emits declaration signatures.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return parseZig(src, exportedOnly)
}
